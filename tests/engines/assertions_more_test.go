package tests

import (
	"context"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

func TestAssertionBranchesForRegexJSONPathAndScalarRendering(t *testing.T) {
	target := &sequenceTarget{responses: []cleanr.Response{{
		StatusCode: 201,
		Latency:    35 * time.Millisecond,
		Text:       "Alpha",
		Stderr:     "warning only",
		Stream: cleanr.StreamMetrics{
			TTFTMS:     20,
			DurationMS: 90,
			ChunkCount: 4,
			Recovered:  true,
		},
		Body: []byte(`{"items":[{"count":2,"name":"alpha"}],"enabled":true}`),
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
			{Type: "json_schema", Path: "response.body", Schema: map[string]any{
				"type":     "object",
				"required": []any{"items", "enabled"},
				"properties": map[string]any{
					"enabled": map[string]any{"type": "boolean"},
					"items": map[string]any{
						"type":     "array",
						"minItems": 1,
						"items": map[string]any{
							"type":     "object",
							"required": []any{"count", "name"},
							"properties": map[string]any{
								"count": map[string]any{"type": "number", "minimum": 1},
								"name":  map[string]any{"enum": []any{"alpha", "beta"}},
							},
						},
					},
				},
			}},
			{Type: "json_path", Path: "response.body.items.0.count", Value: "2"},
			{Type: "contains", Path: "response.provider_raw", Value: "true"},
			{Type: "contains", Path: "response.body.enabled", Value: "true"},
			{Type: "contains", Path: "response.stdout", Value: "Alpha"},
			{Type: "contains", Path: "response.stderr", Value: "warning"},
			{Type: "contains", Path: "response.usage.total_tokens", Value: "7"},
			{Type: "contains", Path: "response.provider_raw.ratio", Value: "1.25"},
			{Type: "status_code", IntValue: intPtr(201)},
			{Type: "exit_code", IntValue: intPtr(0)},
			{Type: "latency_ms", IntValue: intPtr(40)},
			{Type: "stream_ttft_ms", IntValue: intPtr(25)},
			{Type: "stream_duration_ms", IntValue: intPtr(100)},
			{Type: "tool_call_count", IntValue: intPtr(1)},
			{Type: "tool_call_name", Value: "lookup"},
			{Type: "json_path", Path: "response.stream.chunk_count", Value: "4"},
			{Type: "json_path", Path: "response.stream.recovered", Value: "true"},
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
		Stream: cleanr.StreamMetrics{
			TTFTMS:     250,
			DurationMS: 800,
		},
		Body: []byte(`{"items":[]}`),
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
			{Type: "json_schema", Path: "response.body", Schema: map[string]any{
				"type":     "object",
				"required": []any{"items", "enabled"},
				"properties": map[string]any{
					"items": map[string]any{
						"type":     "array",
						"minItems": 1,
						"items": map[string]any{
							"type":     "object",
							"required": []any{"name"},
						},
					},
				},
				"additionalProperties": false,
			}},
			{Type: "json_path", Path: "response.body.items.0.name"},
			{Type: "finish_reason", Value: "stop"},
			{Type: "stream_ttft_ms", IntValue: intPtr(100)},
			{Type: "stream_duration_ms", IntValue: intPtr(500)},
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
	found := false
	for _, finding := range report.Suites[0].Cases[0].Findings {
		if finding.Message == `assertion failed: response.body missing required property "enabled"` {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected json schema failure finding, got %+v", report.Suites[0].Cases[0].Findings)
	}
}
