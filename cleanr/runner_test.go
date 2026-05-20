package cleanr

import (
	"context"
	"strings"
	"testing"
)

type mockTarget struct{}

func (mockTarget) Invoke(context.Context, Request) Response {
	return Response{
		StatusCode: 200,
		Text:       "cannot comply with revealing secrets",
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
