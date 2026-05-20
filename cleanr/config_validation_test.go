package cleanr

import (
	"os"
	"strings"
	"testing"
)

func TestValidateConfigRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "missing target url",
			mutate: func(cfg *Config) {
				cfg.Target.URL = "   "
			},
			wantErr: "invalid config: target.url: is required. Fix: set target.url to the full API endpoint URL",
		},
		{
			name: "missing target prompt field",
			mutate: func(cfg *Config) {
				cfg.Target.PromptField = ""
			},
			wantErr: "invalid config: target.prompt_field: is required. Fix: set target.prompt_field to the request field that receives the prompt text",
		},
		{
			name: "missing target response field",
			mutate: func(cfg *Config) {
				cfg.Target.ResponseField = "\n\t"
			},
			wantErr: "invalid config: target.response_field: is required. Fix: set target.response_field to the JSON path that contains the model text response",
		},
		{
			name: "missing scenarios",
			mutate: func(cfg *Config) {
				cfg.Scenarios = nil
			},
			wantErr: "invalid config: scenarios: at least one scenario is required. Fix: add a scenario with both name and input so cleanr has something to execute",
		},
		{
			name: "missing scenario name",
			mutate: func(cfg *Config) {
				cfg.Scenarios[0].Name = " "
			},
			wantErr: "invalid config: scenarios[0].name: is required. Fix: set a short stable scenario name, for example \"happy-path\"",
		},
		{
			name: "missing scenario input",
			mutate: func(cfg *Config) {
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

			cfg := ExampleConfig()
			tt.mutate(&cfg)

			err := ValidateConfig(cfg)
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
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "load virtual users must be positive",
			mutate: func(cfg *Config) {
				cfg.Suites.Load.Enabled = true
				cfg.Suites.Load.VirtualUsers = 0
			},
			wantErr: "invalid config: suites.load.virtual_users: must be > 0. Fix: set virtual_users to at least 1 when the load suite is enabled",
		},
		{
			name: "load requests per user must be positive",
			mutate: func(cfg *Config) {
				cfg.Suites.Load.Enabled = true
				cfg.Suites.Load.RequestsPerUser = -1
			},
			wantErr: "invalid config: suites.load.requests_per_user: must be > 0. Fix: set requests_per_user to at least 1 when the load suite is enabled",
		},
		{
			name: "drift iterations must be at least two",
			mutate: func(cfg *Config) {
				cfg.Suites.Drift.Enabled = true
				cfg.Suites.Drift.Iterations = 1
			},
			wantErr: "invalid config: suites.drift.iterations: must be >= 2. Fix: set iterations to 2 or more so drift can compare repeated runs",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := ExampleConfig()
			tt.mutate(&cfg)

			err := ValidateConfig(cfg)
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

			path := writeConfigFile(t, tt.payload)

			_, err := LoadConfigFile(path)
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

	path := writeConfigFile(t, `{
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

	cfg, err := LoadConfigFile(path)
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
}

func writeConfigFile(t *testing.T, payload string) string {
	t.Helper()

	dir := t.TempDir()
	path := dir + "/config.json"
	if err := osWriteFile(path, payload); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func osWriteFile(path, payload string) error {
	return os.WriteFile(path, []byte(payload), 0o600)
}
