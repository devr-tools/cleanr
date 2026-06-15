package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

func TestGraphQLTargetInvokeBuildsEnvelopeAndNormalizesErrors(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["query"] != "query Lookup($input: String!) { assistant(input: $input) { reply } }" {
			t.Fatalf("unexpected graphql query: %#v", body["query"])
		}
		if body["operationName"] != "Lookup" {
			t.Fatalf("unexpected operation name: %#v", body["operationName"])
		}
		variables, ok := body["variables"].(map[string]any)
		if !ok || variables["input"] != "Prompt text" {
			t.Fatalf("unexpected graphql variables: %#v", body["variables"])
		}
		return jsonResponse(t, http.StatusOK, map[string]any{
			"data": map[string]any{
				"assistant": map[string]any{"reply": "hello from graphql"},
			},
			"errors": []any{
				map[string]any{"message": "partial failure"},
			},
			"extensions": map[string]any{
				"trace_id": "trace-123",
			},
		}), nil
	})}

	target := cleanr.NewGraphQLTarget(cleanr.TargetConfig{
		Type:          "graphql",
		URL:           "https://example.test/graphql",
		Method:        http.MethodPost,
		ResponseField: "data.assistant.reply",
		GraphQL: cleanr.GraphQLConfig{
			Query:         "query Lookup($input: String!) { assistant(input: $input) { reply } }",
			OperationName: "Lookup",
			VariablesTemplate: map[string]any{
				"input": "{{prompt}}",
			},
		},
	}, client)

	resp := target.Invoke(context.Background(), cleanr.Request{
		Scenario: cleanr.Scenario{Name: "graphql"},
		Prompt:   "Prompt text",
		Timeout:  time.Second,
	})
	if resp.Err != nil || resp.ExtractError != nil {
		t.Fatalf("unexpected graphql errors: err=%v extract=%v", resp.Err, resp.ExtractError)
	}
	if resp.Text != "hello from graphql" {
		t.Fatalf("unexpected graphql text: %q", resp.Text)
	}
	if resp.Normalized.Provider != "graphql" || resp.Normalized.Status != "partial_error" {
		t.Fatalf("unexpected graphql normalized response: %+v", resp.Normalized)
	}
	if resp.Normalized.Raw["operation_name"] != "Lookup" {
		t.Fatalf("expected operation name in raw payload, got %+v", resp.Normalized.Raw)
	}
}

func TestGraphQLTargetInvokeWithoutDataLeavesErrorStatus(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(t, http.StatusOK, map[string]any{
			"errors": []any{map[string]any{"message": "boom"}},
		}), nil
	})}

	target := cleanr.NewGraphQLTarget(cleanr.TargetConfig{
		Type:          "graphql",
		URL:           "https://example.test/graphql",
		Method:        http.MethodPost,
		ResponseField: "data.assistant.reply",
		GraphQL: cleanr.GraphQLConfig{
			Query: "query { assistant { reply } }",
		},
	}, client)

	resp := target.Invoke(context.Background(), cleanr.Request{Timeout: time.Second})
	if resp.Normalized.Status != "error" {
		t.Fatalf("unexpected graphql status: %+v", resp.Normalized)
	}
}
