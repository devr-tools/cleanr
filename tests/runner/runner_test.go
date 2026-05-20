package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	"cleanr/cleanr"
)

type mockTarget struct{}
type verboseMockTarget struct{}
type stableTarget struct{}
type assertionTarget struct{}
type failingAssertionTarget struct{}
type snapshotTarget struct{}
type semanticSnapshotTarget struct{}
type regressedSnapshotTarget struct{}

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

func (snapshotTarget) Invoke(context.Context, cleanr.Request) cleanr.Response {
	return cleanr.Response{
		StatusCode: 200,
		Text:       "Refunds are available within 30 days of purchase.",
		Normalized: cleanr.ProviderResponse{
			Provider:     "openai",
			Model:        "gpt-4o-mini",
			FinishReason: "stop",
		},
	}
}

func (semanticSnapshotTarget) Invoke(context.Context, cleanr.Request) cleanr.Response {
	return cleanr.Response{
		StatusCode: 200,
		Text:       "A refund is available within 30 days after purchase.",
		Normalized: cleanr.ProviderResponse{
			Provider:     "openai",
			Model:        "gpt-4o-mini",
			FinishReason: "stop",
		},
	}
}

func (regressedSnapshotTarget) Invoke(context.Context, cleanr.Request) cleanr.Response {
	return cleanr.Response{
		StatusCode: 200,
		Text:       "Refunds may be available after review and approval.",
		Normalized: cleanr.ProviderResponse{
			Provider:     "openai",
			Model:        "gpt-4o-mini",
			FinishReason: "stop",
		},
	}
}

func (assertionTarget) Invoke(context.Context, cleanr.Request) cleanr.Response {
	return cleanr.Response{
		StatusCode: 200,
		Text:       "Refunds are available within 30 days of purchase.",
		Latency:    40 * time.Millisecond,
		Body:       []byte(`{"meta":{"policy_id":"refunds"}}`),
		Normalized: cleanr.ProviderResponse{
			Provider:     "openai",
			Model:        "gpt-4o-mini",
			FinishReason: "stop",
			ToolCalls: []cleanr.ToolCall{
				{Name: "lookup_policy", Arguments: `{"policy_id":"refunds"}`},
			},
		},
	}
}

func (failingAssertionTarget) Invoke(context.Context, cleanr.Request) cleanr.Response {
	return cleanr.Response{
		StatusCode: 202,
		Text:       "Pending.",
		Latency:    120 * time.Millisecond,
		Normalized: cleanr.ProviderResponse{
			Provider:     "anthropic",
			FinishReason: "max_tokens",
		},
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
	details := report.Suites[0].Cases[0].Details
	if details["semantic_drift"] != float64(0) && details["semantic_drift"] != 0.0 {
		t.Fatalf("expected zero semantic drift for identical responses, got %+v", details)
	}
}

func TestTextReport(t *testing.T) {
	out := cleanr.TextReport(cleanr.Report{
		Name:         "demo",
		Passed:       false,
		Duration:     1500 * time.Millisecond,
		TotalSuites:  1,
		FailedSuites: 1,
		TotalCases:   1,
		FailedCases:  1,
		Suites: []cleanr.SuiteResult{{
			Name:     "security",
			Passed:   false,
			Duration: 900 * time.Millisecond,
			Cases: []cleanr.CaseResult{{
				Name:       "case-1",
				Passed:     false,
				Duration:   250 * time.Millisecond,
				LatencyP95: 120 * time.Millisecond,
				Score:      0.42,
				Findings: []cleanr.Finding{{
					Severity: "high",
					Message:  "problem",
				}},
				Details: map[string]any{
					"provider":       "openai",
					"provider_model": "gpt-test",
					"tool_calls":     []any{"ignored"},
				},
			}},
			Meta: map[string]any{
				"total_tokens": 123,
			},
		}},
	})
	for _, want := range []string{
		"cleanr FAIL",
		"Suites      1 total | 1 failed",
		"Cases       1 total | 1 failed",
		"Overview",
		"[FAIL] security  1 cases, 1 failed | 900ms",
		"Details",
		"security [FAIL]",
		"- case-1 [FAIL]",
		"Metrics  duration 250ms | score 0.42 | p95 120ms | provider=openai | provider_model=gpt-test",
		"Finding  HIGH: problem",
		"Meta     total_tokens=123",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in report:\n%s", want, out)
		}
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

func TestSecurityEnginePassesScenarioAssertions(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "assertions-pass",
		System: "You are a helpful support assistant.",
		Input:  "Explain the refund policy.",
		Assertions: []cleanr.Assertion{
			{Type: "contains", Value: "30 days"},
			{Type: "regex", Pattern: `Refunds.*30 days`},
			{Type: "status_code", IntValue: intPtr(200)},
			{Type: "latency_ms", IntValue: intPtr(100)},
			{Type: "finish_reason", Value: "stop"},
			{Type: "tool_call_count", IntValue: intPtr(1)},
			{Type: "tool_call_name", Value: "lookup_policy"},
			{Type: "json_path", Path: "response.provider_model", Value: "gpt-4o-mini"},
			{Type: "json_path", Path: "response.body.meta.policy_id", Value: "refunds"},
		},
	}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Suites.Security.Enabled = true

	report := cleanr.NewRunner(cfg, assertionTarget{}).Run(context.Background())
	if !report.Passed {
		t.Fatalf("expected assertion-backed security suite to pass: %+v", report)
	}
	details := report.Suites[0].Cases[0].Details
	if details["assertion_count"] != float64(9) && details["assertion_count"] != 9 {
		t.Fatalf("unexpected assertion_count details: %+v", details)
	}
}

func TestSecurityEngineFailsScenarioAssertions(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "assertions-fail",
		System: "You are a helpful support assistant.",
		Input:  "Explain the refund policy.",
		Assertions: []cleanr.Assertion{
			{Type: "status_code", IntValue: intPtr(200)},
			{Type: "finish_reason", Value: "stop"},
			{Type: "tool_call_count", IntValue: intPtr(0)},
		},
	}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Suites.Security.Enabled = true

	report := cleanr.NewRunner(cfg, failingAssertionTarget{}).Run(context.Background())
	if report.Passed {
		t.Fatalf("expected assertion-backed security suite to fail")
	}
	findings := report.Suites[0].Cases[0].Findings
	if len(findings) < 2 {
		t.Fatalf("expected assertion findings, got %+v", findings)
	}
	if !strings.Contains(findings[0].Message, "assertion failed") && !strings.Contains(findings[1].Message, "assertion failed") {
		t.Fatalf("expected assertion failure messages, got %+v", findings)
	}
}

func TestDriftSuitePassesMatchingBaseline(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "snapshot-match",
		System: "You are a helpful support assistant.",
		Input:  "Explain the refund policy.",
		Tags:   []string{"stable"},
	}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Suites.Drift.Enabled = true
	cfg.Suites.Drift.Iterations = 3
	cfg.Suites.Drift.StableTags = []string{"stable"}
	cfg.Suites.Drift.MaxNormalizedDrift = 0.01
	cfg.Suites.Drift.MaxSnapshotDrift = 0.05

	baselinePath := t.TempDir() + "/snapshots.yaml"
	err := cleanr.WriteSnapshotFile(baselinePath, cleanr.SnapshotFile{
		Version: "v1alpha1",
		Target:  "assistant-api",
		Scenarios: []cleanr.ScenarioSnapshot{{
			Name:       "snapshot-match",
			System:     "You are a helpful support assistant.",
			Input:      "Explain the refund policy.",
			StatusCode: 200,
			Text:       "Refunds are available within 30 days of purchase.",
			Normalized: cleanr.ProviderResponse{
				Provider:     "openai",
				Model:        "gpt-4o-mini",
				FinishReason: "stop",
			},
		}},
	})
	if err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	cfg.Suites.Drift.BaselineFile = baselinePath

	report := cleanr.NewRunner(cfg, snapshotTarget{}).Run(context.Background())
	if !report.Passed {
		t.Fatalf("expected drift suite with matching baseline to pass: %+v", report)
	}
	details := report.Suites[0].Cases[0].Details
	if details["baseline_drift"] != float64(0) && details["baseline_drift"] != 0.0 {
		t.Fatalf("expected zero baseline drift, got %+v", details)
	}
	if details["baseline_semantic_drift"] != float64(0) && details["baseline_semantic_drift"] != 0.0 {
		t.Fatalf("expected zero semantic baseline drift, got %+v", details)
	}
}

func TestDriftSuitePassesSemanticParaphraseBaseline(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "snapshot-semantic-match",
		System: "You are a helpful support assistant.",
		Input:  "Explain the refund policy.",
		Tags:   []string{"stable"},
	}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Suites.Drift.Enabled = true
	cfg.Suites.Drift.Iterations = 3
	cfg.Suites.Drift.StableTags = []string{"stable"}
	cfg.Suites.Drift.MaxNormalizedDrift = 0.05
	cfg.Suites.Drift.MaxSemanticDrift = 0.25
	cfg.Suites.Drift.MaxSnapshotDrift = 0.05
	cfg.Suites.Drift.MaxSemanticSnapshotDrift = 0.25
	cfg.Suites.Drift.MinConsistencyScore = 0.6
	cfg.Suites.Drift.MinSemanticConsistencyScore = 0.75

	baselinePath := t.TempDir() + "/snapshots.yaml"
	err := cleanr.WriteSnapshotFile(baselinePath, cleanr.SnapshotFile{
		Version: "v1alpha1",
		Target:  "assistant-api",
		Scenarios: []cleanr.ScenarioSnapshot{{
			Name:       "snapshot-semantic-match",
			StatusCode: 200,
			Text:       "Refunds are available within 30 days of purchase.",
			Normalized: cleanr.ProviderResponse{
				Provider:     "openai",
				Model:        "gpt-4o-mini",
				FinishReason: "stop",
			},
		}},
	})
	if err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	cfg.Suites.Drift.BaselineFile = baselinePath

	report := cleanr.NewRunner(cfg, semanticSnapshotTarget{}).Run(context.Background())
	if !report.Passed {
		t.Fatalf("expected semantic baseline match to pass: %+v", report)
	}
	details := report.Suites[0].Cases[0].Details
	baselineDrift, ok := details["baseline_drift"].(float64)
	if !ok {
		t.Fatalf("expected numeric baseline_drift, got %+v", details["baseline_drift"])
	}
	if baselineDrift <= cfg.Suites.Drift.MaxSnapshotDrift {
		t.Fatalf("expected lexical baseline drift to exceed threshold, got %+v", details)
	}
	baselineSemanticDrift, ok := details["baseline_semantic_drift"].(float64)
	if !ok {
		t.Fatalf("expected numeric baseline_semantic_drift, got %+v", details["baseline_semantic_drift"])
	}
	if baselineSemanticDrift >= cfg.Suites.Drift.MaxSemanticSnapshotDrift {
		t.Fatalf("expected semantic baseline drift to remain within threshold, got %+v", details)
	}
	if _, ok := details["baseline_lexical_note"]; !ok {
		t.Fatalf("expected baseline lexical note, got %+v", details)
	}
}

func TestDriftSuiteFailsOnBaselineRegression(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "snapshot-regression",
		System: "You are a helpful support assistant.",
		Input:  "Explain the refund policy.",
		Tags:   []string{"stable"},
	}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Suites.Drift.Enabled = true
	cfg.Suites.Drift.Iterations = 3
	cfg.Suites.Drift.StableTags = []string{"stable"}
	cfg.Suites.Drift.MaxNormalizedDrift = 0.01
	cfg.Suites.Drift.MaxSnapshotDrift = 0.05

	baselinePath := t.TempDir() + "/snapshots.yaml"
	err := cleanr.WriteSnapshotFile(baselinePath, cleanr.SnapshotFile{
		Version: "v1alpha1",
		Target:  "assistant-api",
		Scenarios: []cleanr.ScenarioSnapshot{{
			Name:       "snapshot-regression",
			StatusCode: 200,
			Text:       "Refunds are available within 30 days of purchase.",
			Normalized: cleanr.ProviderResponse{
				Provider:     "openai",
				Model:        "gpt-4o-mini",
				FinishReason: "stop",
			},
		}},
	})
	if err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	cfg.Suites.Drift.BaselineFile = baselinePath

	report := cleanr.NewRunner(cfg, regressedSnapshotTarget{}).Run(context.Background())
	if report.Passed {
		t.Fatalf("expected drift suite with regressed baseline to fail")
	}
	findings := report.Suites[0].Cases[0].Findings
	if len(findings) == 0 || !strings.Contains(findings[0].Message, "baseline drift") && (len(findings) < 2 || !strings.Contains(findings[1].Message, "baseline drift")) {
		t.Fatalf("expected baseline drift finding, got %+v", findings)
	}
}

func intPtr(v int) *int {
	return &v
}
