package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
)

func TestAssertionExpansionSupportsStreamCadenceAndToolSchemas(t *testing.T) {
	t.Parallel()

	target := &sequenceTarget{responses: []cleanr.Response{{
		StatusCode: 200,
		Text:       "workflow complete",
		Stream: cleanr.StreamMetrics{
			TTFTMS:          40,
			DurationMS:      220,
			ChunkCount:      5,
			CompletionState: "completed",
		},
		Normalized: cleanr.ProviderResponse{
			ToolCalls: []cleanr.ToolCall{
				{
					Name:       "lookup_policy",
					Arguments:  `{"policy_id":"refunds"}`,
					ParsedArgs: map[string]any{"policy_id": "refunds"},
				},
				{
					Name:       "create_ticket",
					Arguments:  `{"priority":"high","reason":"refund"}`,
					ParsedArgs: map[string]any{"priority": "high", "reason": "refund"},
				},
			},
		},
	}}}

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:  "assertion-expansion-pass",
		Input: "hello",
		Assertions: []cleanr.Assertion{
			{Type: "stream_chunk_cadence_ms", IntValue: intPtr(50)},
			{Type: "stream_tool_call_name", Value: "lookup_policy"},
			{Type: "tool_call_order", Value: "lookup_policy, create_ticket"},
			{Type: "tool_call_arguments_schema", Path: "response.tool_calls.0", Schema: map[string]any{
				"type":     "object",
				"required": []any{"policy_id"},
				"properties": map[string]any{
					"policy_id": map[string]any{"const": "refunds"},
				},
			}},
			{Type: "tool_call_arguments_schema", Path: "response.tool_calls.1.parsed_arguments", Schema: map[string]any{
				"type":     "object",
				"required": []any{"priority", "reason"},
				"properties": map[string]any{
					"priority": map[string]any{"const": "high"},
					"reason":   map[string]any{"enum": []any{"refund", "billing"}},
				},
			}},
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
		t.Fatalf("expected expanded assertions to pass, got %+v", report)
	}
}

func TestAssertionExpansionFailureMessages(t *testing.T) {
	t.Parallel()

	target := &sequenceTarget{responses: []cleanr.Response{{
		StatusCode: 200,
		Text:       "workflow complete",
		Stream: cleanr.StreamMetrics{
			TTFTMS:          100,
			DurationMS:      500,
			ChunkCount:      3,
			CompletionState: "completed",
		},
		Normalized: cleanr.ProviderResponse{
			ToolCalls: []cleanr.ToolCall{
				{
					Name:       "create_ticket",
					Arguments:  `{"priority":"low"}`,
					ParsedArgs: map[string]any{"priority": "low"},
				},
			},
		},
	}}}

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:  "assertion-expansion-fail",
		Input: "hello",
		Assertions: []cleanr.Assertion{
			{Type: "stream_chunk_cadence_ms", IntValue: intPtr(80)},
			{Type: "stream_tool_call_name", Value: "lookup_policy"},
			{Type: "tool_call_order", Value: "lookup_policy, create_ticket"},
			{Type: "tool_call_arguments_schema", Path: "response.tool_calls.0", Schema: map[string]any{
				"type":     "object",
				"required": []any{"policy_id"},
				"properties": map[string]any{
					"policy_id": map[string]any{"const": "refunds"},
				},
			}},
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
		t.Fatalf("expected expanded assertions to fail, got %+v", report)
	}

	messages := make([]string, 0, len(report.Suites[0].Cases[0].Findings))
	for _, finding := range report.Suites[0].Cases[0].Findings {
		messages = append(messages, finding.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{
		"expected stream chunk cadence <= 80ms, got 200ms",
		`expected streamed tool call "lookup_policy"`,
		"expected tool call order [lookup_policy, create_ticket], got [create_ticket]",
		`response.tool_calls.0 missing required property "policy_id"`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected finding containing %q, got %s", want, joined)
		}
	}
}

func TestAssertionExpansionValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		assertion cleanr.Assertion
		wantSub   string
	}{
		{
			name:      "stream chunk cadence requires int value",
			assertion: cleanr.Assertion{Type: "stream_chunk_cadence_ms"},
			wantSub:   "assertions[0].int_value: is required",
		},
		{
			name:      "stream tool call name requires value",
			assertion: cleanr.Assertion{Type: "stream_tool_call_name"},
			wantSub:   "assertions[0].value: is required",
		},
		{
			name:      "tool call order requires value",
			assertion: cleanr.Assertion{Type: "tool_call_order"},
			wantSub:   "assertions[0].value: is required",
		},
		{
			name:      "tool call argument schema requires path",
			assertion: cleanr.Assertion{Type: "tool_call_arguments_schema", Schema: map[string]any{"type": "object"}},
			wantSub:   "assertions[0].path: is required",
		},
		{
			name:      "tool call argument schema requires schema",
			assertion: cleanr.Assertion{Type: "tool_call_arguments_schema", Path: "response.tool_calls.0"},
			wantSub:   "assertions[0].schema: is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := cleanr.ExampleConfig()
			cfg.Scenarios[0].Assertions = []cleanr.Assertion{tt.assertion}

			err := cleanr.ValidateConfig(cfg)
			if err == nil {
				t.Fatalf("expected validation error containing %q, got nil", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("expected validation error containing %q, got %q", tt.wantSub, err.Error())
			}
		})
	}
}
