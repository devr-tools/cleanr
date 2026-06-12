package tests

import (
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/testutil"
)

func TestValidateConfigRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*cleanr.Config)
		wantErr string
	}{
		{
			name: "missing target url",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.URL = "   "
			},
			wantErr: "invalid config: target.url: is required. Fix: set target.url to the full API endpoint URL",
		},
		{
			name: "missing target prompt field",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.PromptField = ""
			},
			wantErr: "invalid config: target.prompt_field: is required. Fix: set target.prompt_field to the request field that receives the prompt text",
		},
		{
			name: "missing target response field",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.ResponseField = "\n\t"
			},
			wantErr: "invalid config: target.response_field: is required. Fix: set target.response_field to the JSON path that contains the model text response",
		},
		{
			name: "missing scenarios",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios = nil
			},
			wantErr: "invalid config: scenarios: at least one scenario is required. Fix: add a scenario with both name and input so cleanr has something to execute",
		},
		{
			name: "missing scenario name",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Name = " "
			},
			wantErr: "invalid config: scenarios[0].name: is required. Fix: set a short stable scenario name, for example \"happy-path\"",
		},
		{
			name: "missing scenario input",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Name = "login"
				cfg.Scenarios[0].Input = "\t"
			},
			wantErr: "invalid config: scenarios[0].input: is required. Fix: set the end-user prompt or test input for this scenario",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := cleanr.ExampleConfig()
			tt.mutate(&cfg)

			err := cleanr.ValidateConfig(cfg)
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestValidateConfigInvalidLoadAndDriftSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*cleanr.Config)
		wantErr string
	}{
		{
			name: "load virtual users must be positive",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Load.Enabled = true
				cfg.Suites.Load.VirtualUsers = 0
			},
			wantErr: "invalid config: suites.load.virtual_users: must be > 0. Fix: set virtual_users to at least 1 when the load suite is enabled",
		},
		{
			name: "load requests per user must be positive",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Load.Enabled = true
				cfg.Suites.Load.RequestsPerUser = -1
			},
			wantErr: "invalid config: suites.load.requests_per_user: must be > 0. Fix: set requests_per_user to at least 1 when the load suite is enabled",
		},
		{
			name: "drift iterations must be at least two",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Drift.Enabled = true
				cfg.Suites.Drift.Iterations = 1
			},
			wantErr: "invalid config: suites.drift.iterations: must be >= 2. Fix: set iterations to 2 or more so drift can compare repeated runs",
		},
		{
			name: "snapshot drift threshold must be between zero and one",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Drift.Enabled = true
				cfg.Suites.Drift.MaxSnapshotDrift = 1.5
			},
			wantErr: "invalid config: suites.drift.max_snapshot_drift: must be between 0 and 1. Fix: use a decimal threshold such as 0.18",
		},
		{
			name: "semantic drift threshold must be between zero and one",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Drift.Enabled = true
				cfg.Suites.Drift.MaxSemanticDrift = -0.1
			},
			wantErr: "invalid config: suites.drift.max_semantic_drift: must be between 0 and 1. Fix: use a decimal threshold such as 0.25",
		},
		{
			name: "semantic consistency score must be between zero and one",
			mutate: func(cfg *cleanr.Config) {
				cfg.Suites.Drift.Enabled = true
				cfg.Suites.Drift.MinSemanticConsistencyScore = 1.1
			},
			wantErr: "invalid config: suites.drift.min_semantic_consistency_score: must be between 0 and 1. Fix: use a decimal threshold such as 0.75",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := cleanr.ExampleConfig()
			tt.mutate(&cfg)

			err := cleanr.ValidateConfig(cfg)
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestValidateConfigInvalidAssertions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*cleanr.Config)
		wantErr string
	}{
		{
			name: "unsupported assertion type",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Assertions = []cleanr.Assertion{{Type: "banana"}}
			},
			wantErr: "invalid config: scenarios[0].assertions[0].type: must be one of contains, not_contains, regex, json_path, status_code, latency_ms, stream_ttft_ms, stream_duration_ms, finish_reason, tool_call_count, or tool_call_name. Fix: pick one of the built-in assertion types",
		},
		{
			name: "invalid regex assertion",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Assertions = []cleanr.Assertion{{Type: "regex", Pattern: "("}}
			},
			wantErr: "invalid config: scenarios[0].assertions[0].pattern: must be a valid Go regular expression. Fix: fix the pattern syntax or remove the assertion",
		},
		{
			name: "json path assertion requires path",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Assertions = []cleanr.Assertion{{Type: "json_path", Value: "gpt-4o-mini"}}
			},
			wantErr: "invalid config: scenarios[0].assertions[0].path: is required. Fix: set the response path to check, for example response.provider_model or response.body.output.0.content.0.text",
		},
		{
			name: "tool call count accepts only non-negative values",
			mutate: func(cfg *cleanr.Config) {
				n := -1
				cfg.Scenarios[0].Assertions = []cleanr.Assertion{{Type: "tool_call_count", IntValue: &n}}
			},
			wantErr: "invalid config: scenarios[0].assertions[0].int_value: must be >= 0. Fix: use a non-negative expected tool call count",
		},
		{
			name: "stream ttft assertion requires int value",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios[0].Assertions = []cleanr.Assertion{{Type: "stream_ttft_ms"}}
			},
			wantErr: "invalid config: scenarios[0].assertions[0].int_value: is required. Fix: set the maximum allowed latency in milliseconds",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := cleanr.ExampleConfig()
			tt.mutate(&cfg)

			err := cleanr.ValidateConfig(cfg)
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestLoadConfigFileValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		payload       string
		wantErr       string
		wantErrPrefix string
	}{
		{
			name:          "malformed json",
			payload:       `{"target":`,
			wantErrPrefix: "decode config:",
		},
		{
			name: "missing required target field",
			payload: `{
				"target": {
					"url": "http://localhost:8080",
					"prompt_field": "input"
				},
				"scenarios": [
					{"name": "happy-path", "input": "hello"}
				]
			}`,
			wantErr: "invalid config: target.response_field: is required. Fix: set target.response_field to the JSON path that contains the model text response",
		},
		{
			name: "missing scenarios",
			payload: `{
				"target": {
					"url": "http://localhost:8080",
					"prompt_field": "input",
					"response_field": "output.text"
				}
			}`,
			wantErr: "invalid config: scenarios: at least one scenario is required. Fix: add a scenario with both name and input so cleanr has something to execute",
		},
		{
			name: "blank scenario name",
			payload: `{
				"target": {
					"url": "http://localhost:8080",
					"prompt_field": "input",
					"response_field": "output.text"
				},
				"scenarios": [
					{"name": " ", "input": "hello"}
				]
			}`,
			wantErr: "invalid config: scenarios[0].name: is required. Fix: set a short stable scenario name, for example \"happy-path\"",
		},
		{
			name: "blank scenario input",
			payload: `{
				"target": {
					"url": "http://localhost:8080",
					"prompt_field": "input",
					"response_field": "output.text"
				},
				"scenarios": [
					{"name": "happy-path", "input": " "}
				]
			}`,
			wantErr: "invalid config: scenarios[0].input: is required. Fix: set the end-user prompt or test input for this scenario",
		},
		{
			name: "invalid load settings after defaults",
			payload: `{
				"target": {
					"url": "http://localhost:8080",
					"prompt_field": "input",
					"response_field": "output.text"
				},
				"scenarios": [
					{"name": "happy-path", "input": "hello"}
				],
				"suites": {
					"load": {
						"enabled": true,
						"virtual_users": -2
					}
				}
			}`,
			wantErr: "invalid config: suites.load.virtual_users: must be > 0. Fix: set virtual_users to at least 1 when the load suite is enabled",
		},
		{
			name: "invalid drift settings after defaults",
			payload: `{
				"target": {
					"url": "http://localhost:8080",
					"prompt_field": "input",
					"response_field": "output.text"
				},
				"scenarios": [
					{"name": "happy-path", "input": "hello"}
				],
				"suites": {
					"drift": {
						"enabled": true,
						"iterations": 1
					}
				}
			}`,
			wantErr: "invalid config: suites.drift.iterations: must be >= 2. Fix: set iterations to 2 or more so drift can compare repeated runs",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := testutil.WriteConfigFile(t, tt.payload)

			_, err := cleanr.LoadConfigFile(path)
			if err == nil {
				t.Fatal("expected load error, got nil")
			}
			if tt.wantErr != "" && err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
			}
			if tt.wantErrPrefix != "" && !strings.HasPrefix(err.Error(), tt.wantErrPrefix) {
				t.Fatalf("expected error prefix %q, got %q", tt.wantErrPrefix, err.Error())
			}
		})
	}
}

func TestLoadConfigFileAppliesDefaultsBeforeValidation(t *testing.T) {
	t.Parallel()

	path := testutil.WriteConfigFile(t, `{
		"target": {
			"url": "http://localhost:8080",
			"prompt_field": "input",
			"response_field": "output.text"
		},
		"scenarios": [
			{"name": "happy-path", "input": "hello"}
		],
		"suites": {
			"load": {
				"enabled": true
			},
			"drift": {
				"enabled": true
			}
		}
	}`)

	cfg, err := cleanr.LoadConfigFile(path)
	if err != nil {
		t.Fatalf("expected defaults to satisfy validation, got %v", err)
	}
	if cfg.Suites.Load.VirtualUsers != 4 {
		t.Fatalf("expected default virtual users 4, got %d", cfg.Suites.Load.VirtualUsers)
	}
	if cfg.Suites.Load.RequestsPerUser != 5 {
		t.Fatalf("expected default requests per user 5, got %d", cfg.Suites.Load.RequestsPerUser)
	}
	if cfg.Suites.Drift.Iterations != 3 {
		t.Fatalf("expected default drift iterations 3, got %d", cfg.Suites.Drift.Iterations)
	}
	if cfg.Suites.Drift.MaxSemanticDrift != 0.25 {
		t.Fatalf("expected default semantic drift 0.25, got %v", cfg.Suites.Drift.MaxSemanticDrift)
	}
	if cfg.Suites.Drift.MinSemanticConsistencyScore != 0.75 {
		t.Fatalf("expected default semantic consistency 0.75, got %v", cfg.Suites.Drift.MinSemanticConsistencyScore)
	}
}

func TestLoadConfigFileYAMLAppliesDefaultsBeforeValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
	}{
		{name: "yaml extension", filename: "cleanr.yaml"},
		{name: "yml extension", filename: "cleanr.yml"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := testutil.WriteNamedConfigFile(t, tt.filename, `
target:
  url: http://localhost:8080
  prompt_field: input
  response_field: output.text
scenarios:
  - name: happy-path
    input: hello
suites:
  load:
    enabled: true
  drift:
    enabled: true
`)

			cfg, err := cleanr.LoadConfigFile(path)
			if err != nil {
				t.Fatalf("expected YAML defaults to satisfy validation, got %v", err)
			}
			if cfg.Target.Method != "POST" {
				t.Fatalf("expected default target method POST, got %q", cfg.Target.Method)
			}
			if cfg.Suites.Load.VirtualUsers != 4 {
				t.Fatalf("expected default virtual users 4, got %d", cfg.Suites.Load.VirtualUsers)
			}
			if cfg.Suites.Load.RequestsPerUser != 5 {
				t.Fatalf("expected default requests per user 5, got %d", cfg.Suites.Load.RequestsPerUser)
			}
			if cfg.Suites.Drift.Iterations != 3 {
				t.Fatalf("expected default drift iterations 3, got %d", cfg.Suites.Drift.Iterations)
			}
			if cfg.Suites.Drift.MaxSemanticDrift != 0.25 {
				t.Fatalf("expected default semantic drift 0.25, got %v", cfg.Suites.Drift.MaxSemanticDrift)
			}
			if cfg.Suites.Drift.MinSemanticConsistencyScore != 0.75 {
				t.Fatalf("expected default semantic consistency 0.75, got %v", cfg.Suites.Drift.MinSemanticConsistencyScore)
			}
		})
	}
}

func TestLoadConfigFileYAML(t *testing.T) {
	t.Parallel()

	path := testutil.WriteNamedConfigFile(t, "cleanr.yaml", `
version: v1alpha1
target:
  name: yaml-target
  url: http://localhost:8080/v1/chat
  method: POST
  prompt_field: input
  system_field: system
  response_field: output.text
  request_template:
    input: "{{prompt}}"
    system: "{{system}}"
scenarios:
  - name: yaml-happy-path
    system: You are a helpful assistant.
    input: Explain the refund policy.
reporting:
  format: json
`)

	cfg, err := cleanr.LoadConfigFile(path)
	if err != nil {
		t.Fatalf("load yaml config: %v", err)
	}
	if cfg.Target.Name != "yaml-target" {
		t.Fatalf("unexpected target name: %s", cfg.Target.Name)
	}
	if cfg.Target.PromptField != "input" || cfg.Target.ResponseField != "output.text" {
		t.Fatalf("unexpected target fields: %+v", cfg.Target)
	}
	if len(cfg.Scenarios) != 1 || cfg.Scenarios[0].Name != "yaml-happy-path" {
		t.Fatalf("unexpected scenarios: %+v", cfg.Scenarios)
	}
	template, ok := cfg.Target.RequestTemplate.(map[string]any)
	if !ok {
		t.Fatalf("expected request template map, got %T", cfg.Target.RequestTemplate)
	}
	if template["input"] != "{{prompt}}" {
		t.Fatalf("unexpected template input: %#v", template["input"])
	}
}

func TestLoadConfigFileMalformedYAML(t *testing.T) {
	t.Parallel()

	path := testutil.WriteNamedConfigFile(t, "cleanr.yaml", `
target:
  url: http://localhost:8080/v1/chat
  prompt_field: input
  response_field: output.text
scenarios:
  - name: broken
    input: [hello
`)

	_, err := cleanr.LoadConfigFile(path)
	if err == nil {
		t.Fatal("expected malformed YAML to fail loading")
	}
	if !strings.HasPrefix(err.Error(), "decode config:") {
		t.Fatalf("expected decode config prefix, got %q", err.Error())
	}
}
