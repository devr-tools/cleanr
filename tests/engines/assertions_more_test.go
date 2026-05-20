package tests

import (
	"context"
	"testing"
	"time"

	"cleanr/cleanr"
)

func TestAssertionBranchesForRegexJSONPathAndScalarRendering(t *testing.T) {
	target := &sequenceTarget{responses: []cleanr.Response{{
		StatusCode: 201,
		Latency:    35 * time.Millisecond,
		Text:       "Alpha",
		Body:       []byte(`{"items":[{"count":2,"name":"alpha"}],"enabled":true}`),
		Usage: cleanr.TokenUsage{
			InputTokens:  3,
			OutputTokens: 4,
			TotalTokens:  7,
		},
		Normalized: cleanr.ProviderResponse{
			FinishReason: "stop",
			ToolCalls:    []cleanr.ToolCall{{Name: "lookup", Arguments: "{}"}},
			Raw:          map[string]any{"enabled": true, "ratio": 1.25},
		},
	}}}

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:  "assertion-branches",
		Input: "hello",
		Assertions: []cleanr.Assertion{
			{Type: "regex", Pattern: "^Alpha$"},
			{Type: "json_path", Path: "response.body.items.0.count", Value: "2"},
			{Type: "contains", Path: "response.provider_raw", Value: "true"},
			{Type: "contains", Path: "response.body.enabled", Value: "true"},
			{Type: "contains", Path: "response.usage.total_tokens", Value: "7"},
			{Type: "contains", Path: "response.provider_raw.ratio", Value: "1.25"},
			{Type: "status_code", IntValue: intPtr(201)},
			{Type: "latency_ms", IntValue: intPtr(40)},
			{Type: "tool_call_count", IntValue: intPtr(1)},
			{Type: "tool_call_name", Value: "lookup"},
		},
	}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = true
	cfg.Suites.Security.MaxPIIMatches = 0
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false

	report := cleanr.NewRunner(cfg, target).Run(context.Background())
	if !report.Passed {
		t.Fatalf("expected assertions to pass, got %+v", report)
	}
}

func TestAssertionFailureBranchesForMissingAndMismatchCases(t *testing.T) {
	target := &sequenceTarget{responses: []cleanr.Response{{
		StatusCode: 200,
		Latency:    100 * time.Millisecond,
		Text:       "hello",
		Body:       []byte(`{"items":[]}`),
		Normalized: cleanr.ProviderResponse{
			FinishReason: "done",
		},
	}}}

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:  "assertion-failures",
		Input: "hello",
		Assertions: []cleanr.Assertion{
			{Type: "regex", Pattern: "[", Message: "bad regex"},
			{Type: "json_path", Path: "response.body.items.0.name"},
			{Type: "finish_reason", Value: "stop"},
			{Type: "tool_call_count", IntValue: intPtr(2)},
			{Type: "tool_call_name", Value: "missing"},
			{Type: "not_contains", Value: "hello"},
			{Type: "contains", Value: "missing"},
		},
	}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = true
	cfg.Suites.Security.MaxPIIMatches = 0
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false

	report := cleanr.NewRunner(cfg, target).Run(context.Background())
	if report.Passed || len(report.Suites) != 1 || report.Suites[0].Cases[0].Passed {
		t.Fatalf("expected assertion failures, got %+v", report)
	}
}
