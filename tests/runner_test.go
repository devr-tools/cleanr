package tests

import (
	"context"
	"strings"
	"testing"

	"cleanr/cleanr"
)

type mockTarget struct{}
type verboseMockTarget struct{}
type stableTarget struct{}

func (mockTarget) Invoke(context.Context, cleanr.Request) cleanr.Response {
	return cleanr.Response{
		StatusCode: 200,
		Text:       "cannot comply with revealing secrets",
	}
}

func (verboseMockTarget) Invoke(context.Context, cleanr.Request) cleanr.Response {
	return cleanr.Response{
		StatusCode: 200,
		Text: "This answer repeats itself. This answer repeats itself. " +
			"This answer repeats itself. This answer repeats itself. " +
			"This answer repeats itself.",
	}
}

func (stableTarget) Invoke(context.Context, cleanr.Request) cleanr.Response {
	return cleanr.Response{
		StatusCode: 200,
		Text:       "stable answer",
	}
}

func TestRunnerWithMockTarget(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Target.Name = "mock"
	cfg.Target.ResponseField = "output.text"
	report := cleanr.NewRunner(cfg, mockTarget{}).Run(context.Background())

	if report.TotalSuites == 0 {
		t.Fatalf("expected suites")
	}
	if report.Name != "mock" {
		t.Fatalf("unexpected report name: %s", report.Name)
	}
}

func TestDriftSuitePassesStableResponses(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Suites.Drift.Enabled = true
	cfg.Suites.Drift.Iterations = 3
	cfg.Suites.Drift.MaxNormalizedDrift = 0.01

	report := cleanr.NewRunner(cfg, stableTarget{}).Run(context.Background())
	if len(report.Suites) != 1 || report.Suites[0].Name != "drift" {
		t.Fatalf("unexpected suite result: %+v", report.Suites)
	}
	if !report.Passed {
		t.Fatalf("expected stable drift suite to pass")
	}
}

func TestTextReport(t *testing.T) {
	out := cleanr.TextReport(cleanr.Report{
		Name:         "demo",
		Passed:       false,
		TotalSuites:  1,
		FailedSuites: 1,
		TotalCases:   1,
		FailedCases:  1,
		Suites: []cleanr.SuiteResult{{
			Name:   "security",
			Passed: false,
			Cases: []cleanr.CaseResult{{
				Name:   "case-1",
				Passed: false,
				Findings: []cleanr.Finding{{
					Severity: "high",
					Message:  "problem",
				}},
			}},
		}},
	})
	if !strings.Contains(out, "cleanr FAIL") {
		t.Fatalf("unexpected report output: %s", out)
	}
}

func TestTokenOptimizationEngineFlagsWastefulOutput(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "token-heavy",
		System: "Answer briefly.",
		Input:  "Summarize the policy.",
	}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = true
	cfg.Suites.TokenOptimization.MaxOutputTokens = 12
	cfg.Suites.TokenOptimization.MaxTotalTokens = 24
	cfg.Suites.TokenOptimization.MaxResponseDuplicationRatio = 0.05

	report := cleanr.NewRunner(cfg, verboseMockTarget{}).Run(context.Background())
	if report.Passed {
		t.Fatalf("expected token optimization suite to fail")
	}
	if len(report.Suites) != 1 || report.Suites[0].Name != "token-optimization" {
		t.Fatalf("unexpected suite result: %+v", report.Suites)
	}
	if report.Suites[0].Cases[0].Passed {
		t.Fatalf("expected wasteful case to fail")
	}
}
