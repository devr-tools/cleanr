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
			name: "memory replay requires two sessions",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].MemoryReplay = []cleanr.MemoryReplaySession{{
					SessionID: "session-1",
				}}
			},
			wantSub: "scenarios[0].memory_replay",
		},
		{
			name: "memory replay session ids must be unique",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].MemoryReplay = []cleanr.MemoryReplaySession{
					{SessionID: "session-1"},
					{SessionID: "session-1"},
				}
			},
			wantSub: "duplicates scenarios[0].memory_replay[0].session_id",
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
		{
			name: "negative trend limit",
			mutate: func(cfg *cleanr.Config) {
				cfg.Reporting.TrendLimit = -1
			},
			wantSub: "reporting.trend_limit",
		},
		{
			name: "trend gates require trend file",
			mutate: func(cfg *cleanr.Config) {
				cfg.Reporting.TrendFile = ""
				cfg.Reporting.TrendGates.Enabled = true
				cfg.Reporting.TrendGates.RequiredWindow = 2
			},
			wantSub: "reporting.trend_file",
		},
		{
			name: "trend gates validate thresholds",
			mutate: func(cfg *cleanr.Config) {
				cfg.Reporting.TrendFile = "reports/history.yaml"
				cfg.Reporting.TrendGates.Enabled = true
				cfg.Reporting.TrendGates.RequiredWindow = 1
				cfg.Reporting.TrendGates.MaxFailedCasesDelta = assertionIntPtr(-1)
			},
			wantSub: "reporting.trend_gates.required_window",
		},
		{
			name: "trend gates invalid preset",
			mutate: func(cfg *cleanr.Config) {
				cfg.Reporting.TrendFile = "reports/history.yaml"
				cfg.Reporting.TrendGates.Preset = "chaotic"
			},
			wantSub: "reporting.trend_gates.preset",
		},
		{
			name: "attestation requires output",
			mutate: func(cfg *cleanr.Config) {
				cfg.Governance.Attestation.Enabled = true
				cfg.Governance.Attestation.KeyEnv = "CLEANR_ATTESTATION_KEY"
				cfg.Governance.Attestation.Output = ""
			},
			wantSub: "governance.attestation.output",
		},
		{
			name: "attestation requires key env",
			mutate: func(cfg *cleanr.Config) {
				cfg.Governance.Attestation.Enabled = true
				cfg.Governance.Attestation.Output = "reports/cleanr.attestation.json"
				cfg.Governance.Attestation.KeyEnv = ""
			},
			wantSub: "governance.attestation.key_env",
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

func TestLoadConfigDataPreservesTrendGatePresetOverrides(t *testing.T) {
	t.Parallel()

	data := []byte(`
version: v1alpha1
target:
  type: openai
  name: openai-responses
  openai:
    api_mode: responses
    model: gpt-4.1-mini
    api_key_env: OPENAI_API_KEY
scenarios:
  - name: refund-summary
    input: Summarize the refund policy in one sentence.
suites:
  drift:
    enabled: true
reporting:
  trend_file: reports/cleanr.trends.yaml
  trend_limit: 30
  trend_gates:
    preset: moderate
    max_duration_increase_pct: 40
`)

	cfg, err := cleanr.LoadConfigData(data, "yaml")
	if err != nil {
		t.Fatalf("load config data: %v", err)
	}
	if cfg.Reporting.TrendGates.Preset != "moderate" {
		t.Fatalf("expected preset to survive load, got %+v", cfg.Reporting.TrendGates)
	}
	if cfg.Reporting.TrendGates.MaxDurationIncreasePct == nil || *cfg.Reporting.TrendGates.MaxDurationIncreasePct != 40 {
		t.Fatalf("expected duration override to survive load, got %+v", cfg.Reporting.TrendGates)
	}
	if !cfg.Reporting.TrendGates.Enabled {
		t.Fatalf("expected moderate preset to enable gates, got %+v", cfg.Reporting.TrendGates)
	}
	if cfg.Reporting.TrendGates.MaxSemanticDriftDelta == nil || *cfg.Reporting.TrendGates.MaxSemanticDriftDelta != 0.08 {
		t.Fatalf("expected preset defaults to fill missing fields, got %+v", cfg.Reporting.TrendGates)
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
