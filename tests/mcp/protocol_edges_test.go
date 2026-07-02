package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/internal/mcpserver"
)

// A final request without a trailing newline (common for one-shot piped
// clients) must be answered at EOF, not silently dropped.
func TestMCPServerAnswersFinalUnterminatedLine(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`, // no trailing newline
	}, "\n")

	var out bytes.Buffer
	if err := mcpserver.New().Serve(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	responses := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(responses) != 2 {
		t.Fatalf("expected initialize and ping responses, got %d: %s", len(responses), out.String())
	}
	if !strings.Contains(responses[1], `"id":2`) {
		t.Fatalf("expected ping response for the unterminated final line, got %s", responses[1])
	}
}

// JSON-RPC ids must round-trip exactly: id 0 and id "" were previously
// dropped by omitempty, leaving the client unable to correlate the response.
func TestMCPServerEchoesFalsyRequestIDs(t *testing.T) {
	server := initializedMCPServer(t)

	resp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      0,
		"method":  "ping",
	})
	id, present := resp["id"]
	if !present || id != float64(0) {
		t.Fatalf("expected id 0 to be echoed, got %#v (present=%v)", id, present)
	}

	resp = mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      "",
		"method":  "ping",
	})
	if id, present := resp["id"]; !present || id != "" {
		t.Fatalf("expected empty-string id to be echoed, got %#v (present=%v)", id, present)
	}
}

// Parse errors must carry "id": null per JSON-RPC, not omit the field.
func TestMCPServerParseErrorCarriesNullID(t *testing.T) {
	t.Parallel()

	raw := mcpserver.New().HandleLine(context.Background(), []byte("{not json"))
	if raw == nil {
		t.Fatal("expected parse-error response")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if !strings.Contains(string(data), `"id":null`) {
		t.Fatalf("expected id:null on parse error, got %s", data)
	}
	if !strings.Contains(string(data), `-32700`) {
		t.Fatalf("expected parse-error code, got %s", data)
	}
}

// Ordinary tool execution failures come back as isError results the calling
// model can read; only unknown tools and contained panics are protocol-level
// errors.
func TestMCPServerToolFailureModes(t *testing.T) {
	server := initializedMCPServer(t)

	// Execution failure: missing config → isError result, not a JSON-RPC error.
	resp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params":  map[string]any{"name": "cleanr_generate_dataset", "arguments": map[string]any{}},
	})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("execution failure must be an isError result, got protocol error: %#v", resp)
	}
	result := resp["result"].(map[string]any)
	if isErr, _ := result["isError"].(bool); !isErr {
		t.Fatalf("expected isError result, got %#v", result)
	}
	content := result["content"].([]any)
	if text := content[0].(map[string]any)["text"].(string); !strings.Contains(text, "config") {
		t.Fatalf("expected readable failure text, got %q", text)
	}

	// Unknown tool → protocol-level invalid params.
	resp = mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params":  map[string]any{"name": "cleanr_no_such_tool", "arguments": map[string]any{}},
	})
	respErr, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected protocol error for unknown tool, got %#v", resp)
	}
	if code, _ := respErr["code"].(float64); code != -32602 {
		t.Fatalf("expected -32602 for unknown tool, got %v", respErr["code"])
	}
}
