package tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"cleanr/internal/mcpserver"
)

func TestMCPServerListsToolsAfterInitialization(t *testing.T) {
	server := mcpserver.New()

	initResp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	})

	initResult := initResp["result"].(map[string]any)
	if got := initResult["protocolVersion"]; got != "2025-06-18" {
		t.Fatalf("unexpected protocol version: %v", got)
	}
	caps := initResult["capabilities"].(map[string]any)
	if _, ok := caps["tools"]; !ok {
		t.Fatalf("expected tools capability in initialize response: %#v", caps)
	}

	if resp := mustHandleMaybeMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}); resp != nil {
		t.Fatalf("expected no response for initialized notification, got %#v", resp)
	}

	listResp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	})

	tools := listResp["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.(map[string]any)["name"].(string))
	}
	for _, want := range []string{"cleanr_example_config", "cleanr_validate_config", "cleanr_run"} {
		if !containsString(names, want) {
			t.Fatalf("expected tool %q in %v", want, names)
		}
	}
}

func TestMCPServerValidateConfigToolReturnsStructuredFailure(t *testing.T) {
	server := initializedMCPServer(t)

	resp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "cleanr_validate_config",
			"arguments": map[string]any{
				"format": "yaml",
				"config": "target:\n  prompt_field: input\n",
			},
		},
	})

	result := resp["result"].(map[string]any)
	structured := result["structuredContent"].(map[string]any)
	if structured["valid"].(bool) {
		t.Fatalf("expected invalid config result: %#v", structured)
	}
	errors := structured["errors"].([]any)
	if len(errors) == 0 {
		t.Fatalf("expected validation errors: %#v", structured)
	}
	if !strings.Contains(errors[0].(string), "invalid config") {
		t.Fatalf("expected actionable config error, got %q", errors[0].(string))
	}
}

func TestMCPServerRunToolReturnsReport(t *testing.T) {
	server := initializedMCPServer(t)

	config := `
version: v1alpha1
target:
  name: demo
  url: http://localhost:8080/v1/chat
  method: POST
  prompt_field: input
  system_field: system
  response_field: output.text
scenarios:
  - name: happy-path
    system: You are helpful.
    input: Hello
suites:
  prompt_injection:
    enabled: false
  security:
    enabled: false
  load:
    enabled: false
  chaos:
    enabled: false
  drift:
    enabled: false
  token_optimization:
    enabled: false
reporting:
  format: text
`

	resp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "cleanr_run",
			"arguments": map[string]any{
				"format":        "yaml",
				"report_format": "json",
				"config":        config,
			},
		},
	})

	result := resp["result"].(map[string]any)
	structured := result["structuredContent"].(map[string]any)
	if !structured["passed"].(bool) {
		t.Fatalf("expected passing run result: %#v", structured)
	}
	if got := int(structured["exit_code"].(float64)); got != 0 {
		t.Fatalf("expected exit code 0, got %d", got)
	}
	reportText := structured["report_text"].(string)
	if !strings.Contains(reportText, "\"passed\": true") {
		t.Fatalf("expected json report output, got %s", reportText)
	}
}

func initializedMCPServer(t *testing.T) *mcpserver.Server {
	t.Helper()
	server := mcpserver.New()
	mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	})
	mustHandleMaybeMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	return server
}

func mustHandleMCP(t *testing.T, server *mcpserver.Server, payload map[string]any) map[string]any {
	t.Helper()
	resp := mustHandleMaybeMCP(t, server, payload)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	return resp
}

func mustHandleMaybeMCP(t *testing.T, server *mcpserver.Server, payload map[string]any) map[string]any {
	t.Helper()
	line, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp := server.HandleLine(context.Background(), line)
	if resp == nil {
		return nil
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return decoded
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
