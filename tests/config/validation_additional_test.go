package tests

import (
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
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
			name: "cli requires command",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "cli"
				cfg.Target.CLI.Command = ""
			},
			wantSub: "target.cli.command",
		},
		{
			name: "graphql requires query",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "graphql"
				cfg.Target.URL = "https://example.test/graphql"
				cfg.Target.GraphQL.Query = ""
			},
			wantSub: "target.graphql.query",
		},
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
			name: "openai compatible invalid base url",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "openai_compatible"
				cfg.Target.OpenAI.Model = "llama3.1"
				cfg.Target.OpenAI.BaseURL = "not-a-url"
			},
			wantSub: "target.openai.base_url",
		},
		{
			name: "azure openai requires base url",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "azure_openai"
				cfg.Target.OpenAI.Model = "gpt-4o-mini"
				cfg.Target.OpenAI.BaseURL = ""
				cfg.Target.OpenAI.APIVersion = "2025-03-01-preview"
			},
			wantSub: "target.openai.base_url",
		},
		{
			name: "azure openai requires api version",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "azure_openai"
				cfg.Target.OpenAI.Model = "gpt-4o-mini"
				cfg.Target.OpenAI.BaseURL = "https://example-resource.openai.azure.com/openai/deployments/test-deployment"
				cfg.Target.OpenAI.APIVersion = ""
			},
			wantSub: "target.openai.api_version",
		},
		{
			name: "vertex requires base url",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "vertex"
				cfg.Target.OpenAI.Model = "gemini-2.5-pro"
				cfg.Target.OpenAI.BaseURL = ""
			},
			wantSub: "target.openai.base_url",
		},
		{
			name: "bedrock requires base url",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "bedrock"
				cfg.Target.OpenAI.Model = "anthropic.claude-3-5-sonnet-20240620-v1:0"
				cfg.Target.OpenAI.BaseURL = ""
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
			name: "mcp requires url",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "mcp"
				cfg.Target.MCP.Tool = "cleanr_run"
				cfg.Target.MCP.URL = ""
			},
			wantSub: "target.mcp.url",
		},
		{
			name: "mcp requires tool",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "mcp"
				cfg.Target.MCP.URL = "http://localhost:8080/mcp"
				cfg.Target.MCP.Tool = ""
			},
			wantSub: "target.mcp.tool",
		},
		{
			name: "negative timeout",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.TimeoutMS = -1
			},
			wantSub: "target.timeout_ms",
		},
		{
			name: "load min tokens per second must be non-negative",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Load.Enabled = true
				cfg.Suites.Load.MinTokensPerSecond = -1
			},
			wantSub: "suites.load.min_tokens_per_second",
		},
		{
			name: "load max cost per request requires pricing",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Load.Enabled = true
				cfg.Suites.Load.MaxCostPerRequest = 0.01
				cfg.Suites.Load.InputCostPer1MTokens = 0
				cfg.Suites.Load.OutputCostPer1MTokens = 0
			},
			wantSub: "cost-per-request gating requires pricing",
		},
		{
			name: "load input price must be non-negative",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Load.Enabled = true
				cfg.Suites.Load.InputCostPer1MTokens = -1
			},
			wantSub: "suites.load.input_cost_per_1m_tokens",
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
			name: "turns allow empty legacy input",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Input = ""
				cfg.Scenarios[0].Turns = []cleanr.ConversationTurn{
					{Role: "user", Content: "hello"},
				}
			},
			wantSub: "",
		},
		{
			name: "scenario images allow empty legacy input",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Input = ""
				cfg.Scenarios[0].Images = []cleanr.MediaInput{{
					URL: "https://example.test/refund.png",
				}}
			},
			wantSub: "",
		},
		{
			name: "invalid turn role",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Input = ""
				cfg.Scenarios[0].Turns = []cleanr.ConversationTurn{
					{Role: "banana", Content: "hello"},
				}
			},
			wantSub: "scenarios[0].turns[0].role",
		},
		{
			name: "tool turn requires name",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Input = ""
				cfg.Scenarios[0].Turns = []cleanr.ConversationTurn{
					{Role: "tool", Content: "{\"ok\":true}"},
				}
			},
			wantSub: "scenarios[0].turns[0].name",
		},
		{
			name: "media input requires locator",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Input = ""
				cfg.Scenarios[0].Turns = []cleanr.ConversationTurn{
					{Role: "user", Images: []cleanr.MediaInput{{Detail: "high"}}},
				}
			},
			wantSub: "scenarios[0].turns[0].images[0]",
		},
		{
			name: "mock tool result requires name",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Input = ""
				cfg.Scenarios[0].Turns = []cleanr.ConversationTurn{{
					Role:    "user",
					Content: "hello",
					MockToolResults: []cleanr.MockToolResult{{
						Content: `{"ok":true}`,
					}},
				}}
			},
			wantSub: "scenarios[0].turns[0].mock_tool_results[0].name",
		},
		{
			name: "mock tool result requires payload",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Input = ""
				cfg.Scenarios[0].Turns = []cleanr.ConversationTurn{{
					Role:    "user",
					Content: "hello",
					MockToolResults: []cleanr.MockToolResult{{
						Name: "lookup_policy",
					}},
				}}
			},
			wantSub: "scenarios[0].turns[0].mock_tool_results[0].content",
		},
		{
			name: "invalid image detail",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Input = ""
				cfg.Scenarios[0].Images = []cleanr.MediaInput{{
					URL:    "https://example.test/refund.png",
					Detail: "ultra",
				}}
			},
			wantSub: "scenarios[0].images[0].detail",
		},
		{
			name: "turns cannot combine with memory replay",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Turns = []cleanr.ConversationTurn{{Role: "user", Content: "hello"}}
				cfg.Scenarios[0].MemoryReplay = []cleanr.MemoryReplaySession{{SessionID: "a"}, {SessionID: "b"}}
			},
			wantSub: "cannot combine turns with memory_replay",
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
				cfg.Suites.Drift.MaxNormalizedDrift = float64Ptr(2)
			},
			wantSub: "suites.drift.max_normalized_drift",
		},
		{
			name: "drift invalid consistency score",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Drift.Enabled = true
				cfg.Suites.Drift.MinConsistencyScore = float64Ptr(-0.1)
			},
			wantSub: "suites.drift.min_consistency_score",
		},
		{
			name: "drift invalid semantic thresholds",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Drift.Enabled = true
				cfg.Suites.Drift.MaxSemanticDrift = float64Ptr(2)
				cfg.Suites.Drift.MaxSemanticSnapshotDrift = float64Ptr(-1)
				cfg.Suites.Drift.MinSemanticConsistencyScore = float64Ptr(2)
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
				cfg.Reporting.TrendGates.Enabled = boolPtr(true)
				cfg.Reporting.TrendGates.RequiredWindow = 2
			},
			wantSub: "reporting.trend_file",
		},
		{
			name: "trend gates validate thresholds",
			mutate: func(cfg *cleanr.Config) {
				cfg.Reporting.TrendFile = "reports/history.yaml"
				cfg.Reporting.TrendGates.Enabled = boolPtr(true)
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
		{
			name: "result sink requires absolute endpoint",
			mutate: func(cfg *cleanr.Config) {
				cfg.Integrations.ResultSinks = []cleanr.ResultSinkConfig{{
					Type:     "braintrust",
					Endpoint: "not-a-url",
				}}
			},
			wantSub: "integrations.result_sinks[0].endpoint",
		},
		{
			name: "langfuse sink requires public key env",
			mutate: func(cfg *cleanr.Config) {
				cfg.Integrations.ResultSinks = []cleanr.ResultSinkConfig{{
					Type:         "langfuse",
					SecretKeyEnv: "LANGFUSE_SECRET_KEY",
				}}
			},
			wantSub: "integrations.result_sinks[0].public_key_env",
		},
		{
			name: "langfuse sink requires secret key env",
			mutate: func(cfg *cleanr.Config) {
				cfg.Integrations.ResultSinks = []cleanr.ResultSinkConfig{{
					Type:         "langfuse",
					PublicKeyEnv: "LANGFUSE_PUBLIC_KEY",
				}}
			},
			wantSub: "integrations.result_sinks[0].secret_key_env",
		},
		{
			name: "posthog sink requires project token env",
			mutate: func(cfg *cleanr.Config) {
				cfg.Integrations.ResultSinks = []cleanr.ResultSinkConfig{{
					Type: "posthog",
				}}
			},
			wantSub: "integrations.result_sinks[0].project_token_env",
		},
		{
			name: "trend source validates required selector",
			mutate: func(cfg *cleanr.Config) {
				cfg.Integrations.TrendSources = []cleanr.TrendSourceConfig{{
					Type: "file",
				}}
			},
			wantSub: "integrations.trend_sources[0].path",
		},
		{
			name: "langsmith source requires path or url",
			mutate: func(cfg *cleanr.Config) {
				cfg.Integrations.TrendSources = []cleanr.TrendSourceConfig{{
					Type: "langsmith",
				}}
			},
			wantSub: "integrations.trend_sources[0]",
		},
		{
			name: "summary accepts html format",
			mutate: func(cfg *cleanr.Config) {
				cfg.Integrations.Summaries = []cleanr.SummaryConfig{{
					Format: "html",
					Output: "reports/summary.md",
				}}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := cleanr.ExampleConfig()
			tt.mutate(&cfg)
			err := cleanr.ValidateConfig(cfg)
			if tt.wantSub == "" {
				if err != nil {
					t.Fatalf("expected config to be valid, got %v", err)
				}
				return
			}
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
	if !cfg.Reporting.TrendGates.EnabledValue() {
		t.Fatalf("expected moderate preset to enable gates, got %+v", cfg.Reporting.TrendGates)
	}
	if cfg.Reporting.TrendGates.MaxSemanticDriftDelta == nil || *cfg.Reporting.TrendGates.MaxSemanticDriftDelta != 0.08 {
		t.Fatalf("expected preset defaults to fill missing fields, got %+v", cfg.Reporting.TrendGates)
	}
}

func TestLoadConfigDataAppliesNativeProviderDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		data           string
		wantName       string
		wantAPIMode    string
		wantAPIKeyEnv  string
		wantBaseURL    string
		wantAuthHeader string
		wantAuthScheme string
		wantProvider   string
	}{
		{
			name: "gemini",
			data: `
version: v1alpha1
target:
  type: gemini
  openai:
    model: gemini-2.5-flash
scenarios:
  - name: x
    input: hello
`,
			wantName:       "gemini",
			wantAPIMode:    "chat_completions",
			wantAPIKeyEnv:  "GEMINI_API_KEY",
			wantBaseURL:    "https://generativelanguage.googleapis.com/v1beta/openai",
			wantAuthHeader: "Authorization",
			wantAuthScheme: "Bearer",
			wantProvider:   "gemini",
		},
		{
			name: "mistral",
			data: `
version: v1alpha1
target:
  type: mistral
  openai:
    model: mistral-small-latest
scenarios:
  - name: x
    input: hello
`,
			wantName:       "mistral",
			wantAPIMode:    "chat_completions",
			wantAPIKeyEnv:  "MISTRAL_API_KEY",
			wantBaseURL:    "https://api.mistral.ai/v1",
			wantAuthHeader: "Authorization",
			wantAuthScheme: "Bearer",
			wantProvider:   "mistral",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := cleanr.LoadConfigData([]byte(tt.data), "yaml")
			if err != nil {
				t.Fatalf("load config data: %v", err)
			}
			if cfg.Target.Name != tt.wantName {
				t.Fatalf("unexpected target name: %q", cfg.Target.Name)
			}
			if cfg.Target.OpenAI.APIMode != tt.wantAPIMode {
				t.Fatalf("unexpected api mode: %q", cfg.Target.OpenAI.APIMode)
			}
			if cfg.Target.OpenAI.APIKeyEnv != tt.wantAPIKeyEnv {
				t.Fatalf("unexpected api key env: %q", cfg.Target.OpenAI.APIKeyEnv)
			}
			if cfg.Target.OpenAI.BaseURL != tt.wantBaseURL {
				t.Fatalf("unexpected base url: %q", cfg.Target.OpenAI.BaseURL)
			}
			if cfg.Target.OpenAI.AuthHeader != tt.wantAuthHeader {
				t.Fatalf("unexpected auth header: %q", cfg.Target.OpenAI.AuthHeader)
			}
			if cfg.Target.OpenAI.AuthScheme != tt.wantAuthScheme {
				t.Fatalf("unexpected auth scheme: %q", cfg.Target.OpenAI.AuthScheme)
			}
			if got := cfg.Target.OpenAI.ProviderValue(cfg.Target.Type); got != tt.wantProvider {
				t.Fatalf("unexpected provider label: %q", got)
			}
		})
	}
}

func TestValidateConfigAcceptsNativeBraintrustIntegrationConfig(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.Integrations.ResultSinks = []cleanr.ResultSinkConfig{{
		Type:       "braintrust",
		Project:    "qa-gates",
		Experiment: "release-gate",
		APIKeyEnv:  "CLEANR_BRAINTRUST_TOKEN",
	}}
	cfg.Integrations.TrendSources = []cleanr.TrendSourceConfig{{
		Type:       "braintrust",
		Project:    "qa-gates",
		Experiment: "release-gate",
	}}

	if err := cleanr.ValidateConfig(cfg); err != nil {
		t.Fatalf("expected native Braintrust config to validate, got %v", err)
	}
}

func TestValidateConfigAcceptsNativeLangfuseIntegrationConfig(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.Integrations.ResultSinks = []cleanr.ResultSinkConfig{{
		Type:         "langfuse",
		BaseURL:      "https://cloud.langfuse.com",
		PublicKeyEnv: "LANGFUSE_PUBLIC_KEY",
		SecretKeyEnv: "LANGFUSE_SECRET_KEY",
		Experiment:   "release-gate",
	}}

	if err := cleanr.ValidateConfig(cfg); err != nil {
		t.Fatalf("expected native Langfuse config to validate, got %v", err)
	}
}

func TestValidateConfigAcceptsNativePostHogIntegrationConfig(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.Integrations.ResultSinks = []cleanr.ResultSinkConfig{{
		Type:            "posthog",
		BaseURL:         "https://us.i.posthog.com",
		ProjectTokenEnv: "POSTHOG_PROJECT_API_KEY",
		Experiment:      "release-gate",
	}}

	if err := cleanr.ValidateConfig(cfg); err != nil {
		t.Fatalf("expected native PostHog config to validate, got %v", err)
	}
}

func TestValidateConfigAcceptsVendorTrendImports(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.Integrations.TrendSources = []cleanr.TrendSourceConfig{
		{Type: "langsmith", Path: "exports/langsmith.json"},
		{Type: "openllmetry", URL: "https://collector.example.test/traces"},
		{Type: "provider_logs", Path: "exports/provider-logs.yaml"},
	}

	if err := cleanr.ValidateConfig(cfg); err != nil {
		t.Fatalf("expected vendor trend imports to validate, got %v", err)
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

func TestValidateConfigAllowsMultimodalOnlyScenario(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name: "receipt-image",
		Images: []cleanr.MediaInput{{
			Path:      "fixtures/receipt.png",
			MediaType: "image/png",
		}},
	}}

	if err := cleanr.ValidateConfig(cfg); err != nil {
		t.Fatalf("expected multimodal-only scenario to validate, got %v", err)
	}
}

func TestValidateConfigRejectsInvalidJudgeOutputHooks(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:  "poster",
		Input: "Render a poster",
		JudgeOutputs: []cleanr.JudgeOutput{
			{Type: "video", Path: "response.body.output.0.url"},
			{Type: "image"},
		},
	}}

	err := cleanr.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected judge output validation error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "scenarios[0].judge_outputs[0].type") || !strings.Contains(msg, "scenarios[0].judge_outputs[1].path") {
		t.Fatalf("unexpected judge output validation error: %s", msg)
	}
}
