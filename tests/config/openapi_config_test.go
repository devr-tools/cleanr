package tests

import (
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
)

func TestOpenAPIConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*cleanr.Config)
		wantSub string
	}{
		{
			name: "scenario generation requires source",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "http"
				cfg.OpenAPI.ScenarioGeneration.Enabled = true
			},
			wantSub: "openapi.source",
		},
		{
			name: "scenario generation requires http target",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "cli"
				cfg.Target.CLI.Command = "/bin/echo"
				cfg.OpenAPI.Source.Inline = map[string]any{"openapi": "3.1.0", "paths": map[string]any{"/health": map[string]any{"get": map[string]any{"responses": map[string]any{"200": map[string]any{}}}}}}
				cfg.OpenAPI.ScenarioGeneration.Enabled = true
			},
			wantSub: "openapi.scenario_generation",
		},
		{
			name: "contract diff requires baseline",
			mutate: func(cfg *cleanr.Config) {
				cfg.OpenAPI.Source.Inline = map[string]any{"openapi": "3.1.0", "paths": map[string]any{"/health": map[string]any{"get": map[string]any{"responses": map[string]any{"200": map[string]any{}}}}}}
				cfg.OpenAPI.ContractDiff.Enabled = true
			},
			wantSub: "openapi.contract_diff.baseline",
		},
		{
			name: "source must be singular",
			mutate: func(cfg *cleanr.Config) {
				cfg.OpenAPI.Source.Path = "spec.yaml"
				cfg.OpenAPI.Source.URL = "https://example.test/spec.yaml"
				cfg.OpenAPI.ScenarioGeneration.Enabled = true
			},
			wantSub: "must set exactly one of path, url, or inline",
		},
		{
			name: "invalid include method",
			mutate: func(cfg *cleanr.Config) {
				cfg.OpenAPI.Source.Inline = map[string]any{"openapi": "3.1.0", "paths": map[string]any{"/health": map[string]any{"get": map[string]any{"responses": map[string]any{"200": map[string]any{}}}}}}
				cfg.OpenAPI.ScenarioGeneration.Enabled = true
				cfg.OpenAPI.ScenarioGeneration.IncludeMethods = []string{"FETCH"}
			},
			wantSub: "openapi.scenario_generation.include_methods[0]",
		},
		{
			name: "target openapi hook requires http target",
			mutate: func(cfg *cleanr.Config) {
				cfg.Target.Type = "graphql"
				cfg.Target.URL = "https://example.test/graphql"
				cfg.Target.GraphQL.Query = "query { ok }"
				cfg.Target.OpenAPI.Enabled = true
			},
			wantSub: "target.openapi.enabled",
		},
		{
			name: "openapi metadata allows empty input",
			mutate: func(cfg *cleanr.Config) {
				cfg.Scenarios = []cleanr.Scenario{{
					Name:  "list-tickets",
					Input: "",
					Metadata: map[string]string{
						"openapi.method": "GET",
						"openapi.path":   "/tickets",
					},
				}}
				cfg.Target.Type = "http"
				cfg.Target.OpenAPI.Enabled = true
			},
			wantSub: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := cleanr.ExampleConfig()
			cfg.OpenAPI = cleanr.OpenAPIConfig{}
			tt.mutate(&cfg)
			err := cleanr.ValidateConfig(cfg)
			if tt.wantSub == "" {
				if err != nil {
					t.Fatalf("expected config to validate, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("expected validation error containing %q, got %v", tt.wantSub, err)
			}
		})
	}
}

func TestOpenAPIConfigRoundTrip(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.OpenAPI = cleanr.OpenAPIConfig{
		Source: cleanr.OpenAPISource{
			Path: "specs/public-api.yaml",
		},
		ScenarioGeneration: cleanr.OpenAPIScenarioGenerationConfig{
			Enabled:        true,
			IncludeMethods: []string{"GET", "POST"},
		},
		ContractDiff: cleanr.OpenAPIContractDiffConfig{
			Enabled: true,
			Baseline: cleanr.OpenAPISource{
				Path: "specs/baseline.yaml",
			},
		},
	}

	data, err := cleanr.MarshalConfig(cfg, "yaml")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	loaded, err := cleanr.LoadConfigData(data, "yaml")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.OpenAPI.Source.Path != "specs/public-api.yaml" {
		t.Fatalf("unexpected openapi source: %+v", loaded.OpenAPI)
	}
	if !loaded.OpenAPI.ScenarioGeneration.Enabled || loaded.OpenAPI.ScenarioGeneration.OutputFile != "generated/openapi.scenarios.yaml" {
		t.Fatalf("unexpected scenario generation config: %+v", loaded.OpenAPI.ScenarioGeneration)
	}
	if !loaded.OpenAPI.ContractDiff.Enabled || loaded.OpenAPI.ContractDiff.OutputFile != "reports/openapi.diff.yaml" {
		t.Fatalf("unexpected contract diff config: %+v", loaded.OpenAPI.ContractDiff)
	}
}
