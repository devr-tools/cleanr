package cleanr

import (
	"context"
	"strings"
	"testing"
)

type mockTarget struct{}
type verboseMockTarget struct{}

func (mockTarget) Invoke(context.Context, Request) Response {
	return Response{
		StatusCode: 200,
		Text:       "cannot comply with revealing secrets",
	}
}

func (verboseMockTarget) Invoke(context.Context, Request) Response {
	return Response{
		StatusCode: 200,
		Text: "This answer repeats itself. This answer repeats itself. " +
			"This answer repeats itself. This answer repeats itself. " +
			"This answer repeats itself.",
	}
}

func TestRunnerWithMockTarget(t *testing.T) {
	cfg := ExampleConfig()
	cfg.Target.Name = "mock"
	cfg.Target.ResponseField = "output.text"
	report := NewRunner(cfg, mockTarget{}).Run(context.Background())

	if report.TotalSuites == 0 {
		t.Fatalf("expected suites")
	}
	if report.Name != "mock" {
		t.Fatalf("unexpected report name: %s", report.Name)
	}
}

func TestNormalizedDistance(t *testing.T) {
	if got := normalizedDistance("abc", "abc"); got != 0 {
		t.Fatalf("expected zero distance, got %f", got)
	}
	if got := normalizedDistance("abc", "xyz"); got <= 0 {
		t.Fatalf("expected non-zero distance, got %f", got)
	}
}

func TestTextReport(t *testing.T) {
	out := TextReport(Report{
		Name:         "demo",
		Passed:       false,
		TotalSuites:  1,
		FailedSuites: 1,
		TotalCases:   1,
		FailedCases:  1,
		Suites: []SuiteResult{{
			Name:   "security",
			Passed: false,
			Cases: []CaseResult{{
				Name:   "case-1",
				Passed: false,
				Findings: []Finding{{
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
	cfg := ExampleConfig()
	cfg.Scenarios = []Scenario{{
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

	report := NewRunner(cfg, verboseMockTarget{}).Run(context.Background())
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
