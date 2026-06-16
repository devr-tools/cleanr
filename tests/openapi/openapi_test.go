package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
)

func TestGenerateOpenAPIScenariosFromInlineSpec(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.Target.Type = "http"
	cfg.Target.URL = "https://api.example.test/v1"
	cfg.Target.OpenAPI = cleanr.OpenAPITargetConfig{Enabled: true}
	cfg.OpenAPI = cleanr.OpenAPIConfig{
		Source: cleanr.OpenAPISource{Inline: map[string]any{
			"openapi": "3.1.0",
			"info": map[string]any{
				"title": "Ticket API",
			},
			"paths": map[string]any{
				"/tickets/{ticket_id}": map[string]any{
					"get": map[string]any{
						"operationId": "get-ticket",
						"tags":        []any{"tickets"},
						"parameters": []any{
							map[string]any{"name": "ticket_id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}, "example": "t_123"},
							map[string]any{"name": "verbose", "in": "query", "required": true, "schema": map[string]any{"type": "boolean"}, "example": true},
						},
						"responses": map[string]any{
							"200": map[string]any{
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": map[string]any{
											"type":     "object",
											"required": []any{"id", "status"},
											"properties": map[string]any{
												"id":     map[string]any{"type": "string"},
												"status": map[string]any{"type": "string"},
											},
										},
									},
								},
							},
						},
					},
				},
				"/tickets": map[string]any{
					"post": map[string]any{
						"operationId": "create-ticket",
						"tags":        []any{"tickets"},
						"requestBody": map[string]any{
							"required": true,
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type":     "object",
										"required": []any{"title"},
										"properties": map[string]any{
											"title":    map[string]any{"type": "string"},
											"priority": map[string]any{"type": "string", "default": "medium"},
										},
									},
								},
							},
						},
						"responses": map[string]any{
							"201": map[string]any{
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": map[string]any{
											"type":     "object",
											"required": []any{"id"},
											"properties": map[string]any{
												"id": map[string]any{"type": "string"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}},
		ScenarioGeneration: cleanr.OpenAPIScenarioGenerationConfig{
			Enabled: true,
		},
	}

	scenarios, err := cleanr.GenerateOpenAPIScenarios(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("generate scenarios: %v", err)
	}
	if len(scenarios) != 2 {
		t.Fatalf("expected two generated scenarios, got %d", len(scenarios))
	}

	getScenario := scenarios[0]
	if getScenario.Name != "get-ticket" {
		t.Fatalf("unexpected GET scenario name: %+v", getScenario)
	}
	if getScenario.Input != "" {
		t.Fatalf("expected no request body for GET scenario, got %q", getScenario.Input)
	}
	if getScenario.Metadata["openapi.path"] != "/tickets/t_123" || getScenario.Metadata["openapi.query"] != "verbose=true" {
		t.Fatalf("unexpected GET scenario metadata: %+v", getScenario.Metadata)
	}
	if len(getScenario.Assertions) != 2 || getScenario.Assertions[0].Type != "status_code" || *getScenario.Assertions[0].IntValue != 200 {
		t.Fatalf("unexpected GET scenario assertions: %+v", getScenario.Assertions)
	}

	postScenario := scenarios[1]
	if postScenario.Metadata["openapi.method"] != "POST" || postScenario.Metadata["openapi.path"] != "/tickets" {
		t.Fatalf("unexpected POST scenario metadata: %+v", postScenario.Metadata)
	}
	if !strings.Contains(postScenario.Input, "\"title\"") || !strings.Contains(postScenario.Input, "\"priority\"") {
		t.Fatalf("expected generated request body example, got %q", postScenario.Input)
	}
	if postScenario.Metadata["openapi.content_type"] != "application/json" {
		t.Fatalf("unexpected POST content type metadata: %+v", postScenario.Metadata)
	}
}

func TestDiffOpenAPIContracts(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.OpenAPI = cleanr.OpenAPIConfig{
		Source: cleanr.OpenAPISource{Inline: map[string]any{
			"openapi": "3.1.0",
			"paths": map[string]any{
				"/tickets/{ticket_id}": map[string]any{
					"get": map[string]any{
						"operationId": "get-ticket",
						"parameters": []any{
							map[string]any{"name": "ticket_id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}},
							map[string]any{"name": "region", "in": "query", "required": true, "schema": map[string]any{"type": "string"}},
						},
						"responses": map[string]any{
							"200": map[string]any{
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": map[string]any{
											"type":     "object",
											"required": []any{"id", "status", "priority"},
											"properties": map[string]any{
												"id":       map[string]any{"type": "string"},
												"status":   map[string]any{"type": "string"},
												"priority": map[string]any{"type": "string"},
											},
										},
									},
								},
							},
						},
					},
				},
				"/tickets": map[string]any{
					"post": map[string]any{
						"operationId": "create-ticket",
						"responses": map[string]any{
							"201": map[string]any{
								"description": "created",
							},
						},
					},
				},
			},
		}},
		ContractDiff: cleanr.OpenAPIContractDiffConfig{
			Enabled: true,
			Baseline: cleanr.OpenAPISource{Inline: map[string]any{
				"openapi": "3.1.0",
				"paths": map[string]any{
					"/tickets/{ticket_id}": map[string]any{
						"get": map[string]any{
							"operationId": "get-ticket",
							"parameters": []any{
								map[string]any{"name": "ticket_id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}},
							},
							"responses": map[string]any{
								"200": map[string]any{
									"content": map[string]any{
										"application/json": map[string]any{
											"schema": map[string]any{
												"type":     "object",
												"required": []any{"id", "status"},
												"properties": map[string]any{
													"id":     map[string]any{"type": "string"},
													"status": map[string]any{"type": "string"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}},
		},
	}

	diff, err := cleanr.DiffOpenAPIContracts(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("diff contracts: %v", err)
	}
	if !diff.Breaking {
		t.Fatalf("expected breaking diff, got %+v", diff)
	}
	if diff.Summary.OperationsAdded != 1 || diff.Summary.OperationsChanged != 1 {
		t.Fatalf("unexpected diff summary: %+v", diff.Summary)
	}

	var sawRequiredParam, sawResponseSchema, sawOperationAdded bool
	for _, change := range diff.Changes {
		switch change.Kind {
		case "parameter_added":
			if change.Level == "breaking" {
				sawRequiredParam = true
			}
		case "response_schema_changed":
			sawResponseSchema = true
		case "operation_added":
			sawOperationAdded = true
		}
	}
	if !sawRequiredParam || !sawResponseSchema || !sawOperationAdded {
		t.Fatalf("unexpected diff changes: %+v", diff.Changes)
	}
}
