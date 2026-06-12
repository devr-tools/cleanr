package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

func TestMCPTargetInvokesToolOverJSONRPC(t *testing.T) {
	t.Parallel()

	callCount := 0
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		callCount++
		if r.URL.String() != "https://mcp.test/rpc" {
			t.Fatalf("unexpected mcp url: %s", r.URL.String())
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch body["method"] {
		case "initialize":
			return jsonResponse(t, http.StatusOK, map[string]any{
				"jsonrpc": "2.0",
				"id":      body["id"],
				"result": map[string]any{
					"protocolVersion": "2025-06-18",
				},
			}), nil
		case "notifications/initialized":
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       http.NoBody,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "tools/call":
			params := body["params"].(map[string]any)
			if params["name"] != "lookup_customer" {
				t.Fatalf("unexpected tool name: %#v", params)
			}
			args := params["arguments"].(map[string]any)
			if args["prompt"] != "Need customer profile" || args["transcript"] == "" {
				t.Fatalf("unexpected tool arguments: %#v", args)
			}
			if _, ok := args["messages"].([]any); !ok {
				t.Fatalf("expected transcript messages in args: %#v", args)
			}
			return jsonResponse(t, http.StatusOK, map[string]any{
				"jsonrpc": "2.0",
				"id":      body["id"],
				"result": map[string]any{
					"content": []any{
						map[string]any{"type": "text", "text": "lookup complete"},
					},
					"structuredContent": map[string]any{
						"customer": map[string]any{
							"id":   "cust_123",
							"tier": "gold",
						},
					},
				},
			}), nil
		default:
			t.Fatalf("unexpected method: %#v", body["method"])
		}
		return nil, nil
	})}

	mcpTarget := cleanr.NewMCPTarget(cleanr.TargetConfig{
		Type: "mcp",
		MCP: cleanr.MCPConfig{
			URL:            "https://mcp.test/rpc",
			Tool:           "lookup_customer",
			ResultTextPath: "customer.id",
			ArgumentsTemplate: map[string]any{
				"prompt":     "{{prompt}}",
				"transcript": "{{transcript}}",
			},
		},
	}, client)

	resp := mcpTarget.Invoke(context.Background(), cleanr.BuildScenarioRequest(cleanr.Scenario{
		Name: "mcp-target",
		Turns: []cleanr.ConversationTurn{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Need customer profile"},
		},
	}, 2*time.Second))

	if resp.Err != nil || resp.ExtractError != nil {
		t.Fatalf("unexpected mcp response errors: err=%v extract=%v", resp.Err, resp.ExtractError)
	}
	if resp.Text != "cust_123" {
		t.Fatalf("unexpected mcp response text: %q", resp.Text)
	}
	if resp.Normalized.Provider != "mcp" || len(resp.Normalized.ToolCalls) != 1 || resp.Normalized.ToolCalls[0].Name != "lookup_customer" {
		t.Fatalf("unexpected mcp normalized response: %+v", resp.Normalized)
	}
	if callCount != 3 {
		t.Fatalf("expected initialize, notification, and tool call; got %d", callCount)
	}
}
