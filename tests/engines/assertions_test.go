package tests

import (
	"context"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

func intPtr(v int) *int { return &v }

func TestSecurityEngineCoversScenarioAssertions(t *testing.T) {
	t.Parallel()

	target := &sequenceTarget{responses: []cleanr.Response{{
		StatusCode: 200,
		Latency:    20 * time.Millisecond,
		Text:       "hello world",
		Body:       []byte(`{"output":{"text":"hello world","items":[{"name":"refund"}]}}`),
		Normalized: cleanr.ProviderResponse{
			FinishReason: "stop",
			ToolCalls:    []cleanr.ToolCall{{Name: "lookup_refund", ParsedArgs: map[string]any{"policy_id": "refunds"}}},
			Raw:          map[string]any{"mode": "demo"},
		},
	}}}

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "assertions",
		Input:  "hello",
		System: "system",
		Assertions: []cleanr.Assertion{
			{Type: "contains", Value: "hello"},
			{Type: "not_contains", Value: "secret"},
			{Type: "regex", Pattern: "^hello"},
			{Type: "json_schema", Schema: map[string]any{
				"type":     "object",
				"required": []any{"output"},
				"properties": map[string]any{
					"output": map[string]any{
						"type":     "object",
						"required": []any{"items"},
						"properties": map[string]any{
							"items": map[string]any{
								"type":     "array",
								"minItems": 1,
								"items": map[string]any{
									"type":     "object",
									"required": []any{"name"},
									"properties": map[string]any{
										"name": map[string]any{"const": "refund"},
									},
								},
							},
						},
					},
				},
			}},
			{Type: "json_path", Path: "response.body.output.items.0.name", Value: "refund"},
			{Type: "status_code", IntValue: intPtr(200)},
			{Type: "latency_ms", IntValue: intPtr(30)},
			{Type: "finish_reason", Value: "stop"},
			{Type: "tool_call_count", IntValue: intPtr(1)},
			{Type: "tool_call_name", Value: "lookup_refund"},
			{Type: "contains", Path: "response.provider_raw", Value: "demo"},
			{Type: "json_path", Path: "response.tool_calls.0.name", Value: "lookup_refund"},
			{Type: "json_path", Path: "response.tool_calls.0.parsed_arguments.policy_id", Value: "refunds"},
		},
	}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = true
	cfg.Suites.Security.MaxPIIMatches = 0
	cfg.Suites.Security.DangerousToolIndicators = nil
	cfg.Suites.Security.SecretExposureIndicators = nil
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false

	report := cleanr.NewRunner(cfg, target).Run(context.Background())
	if len(report.Suites) != 1 || !report.Suites[0].Passed {
		t.Fatalf("unexpected assertion report: %+v", report)
	}
	if report.Suites[0].Cases[0].Details["assertion_count"] != 13 {
		t.Fatalf("expected assertion count in details, got %+v", report.Suites[0].Cases[0].Details)
	}
}
