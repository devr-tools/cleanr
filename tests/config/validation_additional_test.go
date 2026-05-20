package tests

import (
	"strings"
	"testing"

	"cleanr/cleanr"
)

func assertionIntPtr(v int) *int { return &v }

func TestValidateConfigCoversProviderAndSuiteEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*cleanr.Config)
		wantSub string
	}{
		{
			name: "openai requires model",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "openai"
				cfg.Target.OpenAI.Model = ""
			},
			wantSub: "target.openai.model",
		},
		{
			name: "openai invalid api mode",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "openai"
				cfg.Target.OpenAI.Model = "gpt-4.1-mini"
				cfg.Target.OpenAI.APIMode = "bad"
			},
			wantSub: "target.openai.api_mode",
		},
		{
			name: "openai invalid base url",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "openai"
				cfg.Target.OpenAI.Model = "gpt-4.1-mini"
				cfg.Target.OpenAI.BaseURL = "not-a-url"
			},
			wantSub: "target.openai.base_url",
		},
		{
			name: "anthropic invalid base url",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "anthropic"
				cfg.Target.Anthropic.Model = "claude-sonnet-4-20250514"
				cfg.Target.Anthropic.BaseURL = "not-a-url"
			},
			wantSub: "target.anthropic.base_url",
		},
		{
			name: "anthropic invalid max tokens",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "anthropic"
				cfg.Target.Anthropic.Model = "claude-sonnet-4-20250514"
				cfg.Target.Anthropic.MaxTokens = -1
			},
			wantSub: "target.anthropic.max_tokens",
		},
		{
			name: "negative timeout",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.TimeoutMS = -1
			},
			wantSub: "target.timeout_ms",
		},
		{
			name: "duplicate scenario names",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios = []cleanr.Scenario{{Name: "dup", Input: "a"}, {Name: "dup", Input: "b"}}
			},
			wantSub: "duplicates scenarios[0].name",
		},
		{
			name: "load max error rate range",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Load.Enabled = true
				cfg.Suites.Load.MaxErrorRatePct = 101
			},
			wantSub: "suites.load.max_error_rate_pct",
		},
		{
			name: "load latency negative",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Load.Enabled = true
				cfg.Suites.Load.P95LatencyMS = -1
			},
			wantSub: "suites.load.p95_latency_ms",
		},
		{
			name: "invalid leak regex",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Security.LeakPatterns = []string{"["}
			},
			wantSub: "suites.security.leak_patterns[0]",
		},
		{
			name: "chaos invalid fault",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Chaos.Enabled = true
				cfg.Suites.Chaos.Faults = []string{"broken"}
			},
			wantSub: "suites.chaos.faults[0]",
		},
		{
			name: "chaos invalid timeout scale",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Chaos.Enabled = true
				cfg.Suites.Chaos.TimeoutScale = 0
			},
			wantSub: "suites.chaos.timeout_scale",
		},
		{
			name: "chaos negative noise",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Chaos.Enabled = true
				cfg.Suites.Chaos.NoiseBytes = -1
			},
			wantSub: "suites.chaos.noise_bytes",
		},
		{
			name: "chaos invalid max error rate",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Chaos.Enabled = true
				cfg.Suites.Chaos.MaxErrorRate = 101
			},
			wantSub: "suites.chaos.max_error_rate_pct",
		},
		{
			name: "drift invalid normalized drift",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Drift.Enabled = true
				cfg.Suites.Drift.MaxNormalizedDrift = 2
			},
			wantSub: "suites.drift.max_normalized_drift",
		},
		{
			name: "drift invalid consistency score",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Drift.Enabled = true
				cfg.Suites.Drift.MinConsistencyScore = -0.1
			},
			wantSub: "suites.drift.min_consistency_score",
		},
		{
			name: "drift invalid semantic thresholds",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Drift.Enabled = true
				cfg.Suites.Drift.MaxSemanticDrift = 2
				cfg.Suites.Drift.MaxSemanticSnapshotDrift = -1
				cfg.Suites.Drift.MinSemanticConsistencyScore = 2
			},
			wantSub: "suites.drift.max_semantic_drift",
		},
		{
			name: "token optimization invalid budgets and ratios",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.TokenOptimization.Enabled = true
				cfg.Suites.TokenOptimization.MaxInputTokens = -1
				cfg.Suites.TokenOptimization.MaxOutputTokens = -1
				cfg.Suites.TokenOptimization.MaxTotalTokens = -1
				cfg.Suites.TokenOptimization.MaxOutputInputRatio = 0
				cfg.Suites.TokenOptimization.MaxPromptDuplicationRatio = 2
				cfg.Suites.TokenOptimization.MaxResponseDuplicationRatio = -1
				cfg.Suites.TokenOptimization.SuggestedMaxOutputTokens = -1
			},
			wantSub: "suites.token_optimization.max_input_tokens",
		},
		{
			name: "assertion validation",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Assertions = []cleanr.Assertion{
					{Type: "regex", Pattern: "[", Severity: "bogus"},
					{Type: "status_code"},
					{Type: "latency_ms", IntValue: assertionIntPtr(-1)},
					{Type: "tool_call_count", IntValue: assertionIntPtr(-1)},
					{Type: "tool_call_name", Value: ""},
					{Type: "json_path", Path: ""},
					{Type: "unknown"},
				}
			},
			wantSub: "assertions[0].pattern",
		},
		{
			name: "unknown target type",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "mystery"
			},
			wantSub: "target.type",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := cleanr.ExampleConfig()
			tt.mutate(&cfg)
			err := cleanr.ValidateConfig(cfg)
			if err == nil || !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("expected %q in error, got %v", tt.wantSub, err)
			}
		})
	}
}

func TestFieldAndValidationErrorFormattingBranches(t *testing.T) {
	t.Parallel()

	field := cleanr.FieldError{Path: "x", Message: "bad"}
	if got := field.Error(); got != "x: bad" {
		t.Fatalf("unexpected field error without hint: %q", got)
	}

	var errs cleanr.ValidationErrors
	if errs.Error() != "" {
		t.Fatalf("expected empty error string")
	}
	errs.Add("x", "bad", "fix it")
	if !strings.Contains(errs.Error(), "invalid config: x: bad. Fix: fix it") {
		t.Fatalf("unexpected single validation error: %q", errs.Error())
	}
}
