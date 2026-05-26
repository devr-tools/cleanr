package tests

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/mcpserver"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read boom") }

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("write boom") }

func TestMCPServerCoversProtocolErrorsAndExampleTool(t *testing.T) {
	t.Parallel()

	server := mcpserver.New()
	if resp := server.HandleLine(context.Background(), []byte("{")); resp == nil || resp.Error == nil || resp.Error.Code != -32700 {
		t.Fatalf("expected parse error response, got %#v", resp)
	}

	resp := server.HandleLine(context.Background(), []byte(`{"jsonrpc":"1.0","id":1,"method":"ping"}`))
	if resp == nil || resp.Error == nil || resp.Error.Code != -32600 {
		t.Fatalf("expected invalid request response, got %#v", resp)
	}

	resp = server.HandleLine(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	if resp == nil || resp.Error == nil || !strings.Contains(resp.Error.Message, "server not initialized") {
		t.Fatalf("expected uninitialized error, got %#v", resp)
	}

	server = initializedMCPServer(t)
	pingResp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "ping",
		"params":  map[string]any{},
	})
	if pingResp["error"] != nil {
		t.Fatalf("unexpected ping error: %#v", pingResp)
	}

	exampleResp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "cleanr_example_config",
			"arguments": map[string]any{
				"format": "yaml",
			},
		},
	})
	result := exampleResp["result"].(map[string]any)
	structured := result["structuredContent"].(map[string]any)
	if structured["format"] != "yaml" || !strings.Contains(structured["config"].(string), "target:") {
		t.Fatalf("unexpected example config output: %#v", structured)
	}

	validateResp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "cleanr_validate_config",
			"arguments": map[string]any{
				"config_path": filepath.Join(t.TempDir(), "missing.json"),
			},
		},
	})
	structured = validateResp["result"].(map[string]any)["structuredContent"].(map[string]any)
	if structured["valid"].(bool) {
		t.Fatalf("expected invalid path result: %#v", structured)
	}

	runResp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "cleanr_run",
			"arguments": map[string]any{
				"format":        "json",
				"report_format": "junit",
				"config":        `{"target":{"url":"https://example.com","prompt_field":"input","response_field":"output.text"},"scenarios":[]}`,
			},
		},
	})
	structured = runResp["result"].(map[string]any)["structuredContent"].(map[string]any)
	if structured["exit_code"].(float64) != 2 || structured["report_format"] != "junit" {
		t.Fatalf("unexpected invalid run result: %#v", structured)
	}
}

func TestMCPServerServeAndRunCoverIOBranches(t *testing.T) {
	t.Parallel()

	server := mcpserver.New()
	input := bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-06-18\"}}\n\n{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\",\"params\":{}}\n")
	var output bytes.Buffer
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("serve failed: %v", err)
	}
	if !strings.Contains(output.String(), `"tools"`) {
		t.Fatalf("unexpected serve output: %s", output.String())
	}

	if err := server.Serve(context.Background(), errReader{}, &output); err == nil || !strings.Contains(err.Error(), "read boom") {
		t.Fatalf("expected reader error, got %v", err)
	}

	if err := server.Serve(context.Background(), bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-06-18\"}}\n"), errWriter{}); err == nil || !strings.Contains(err.Error(), "write boom") {
		t.Fatalf("expected writer error, got %v", err)
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	os.Stdin = reader
	stdoutFile, err := os.Create(filepath.Join(t.TempDir(), "stdout.txt"))
	if err != nil {
		t.Fatalf("create stdout file: %v", err)
	}
	os.Stdout = stdoutFile
	_ = writer.Close()
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		_ = reader.Close()
		_ = stdoutFile.Close()
	}()

	if err := mcpserver.Run(context.Background()); err != nil {
		t.Fatalf("mcp Run failed: %v", err)
	}
}

func TestMCPServerCoversUnknownToolAndConfigPathRun(t *testing.T) {
	t.Parallel()

	server := initializedMCPServer(t)
	resp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "unknown",
		},
	})
	if resp["error"] == nil {
		t.Fatalf("expected unknown tool error: %#v", resp)
	}

	cfg := cleanr.ExampleConfig()
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	path := filepath.Join(t.TempDir(), "cleanr.json")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	resp = mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      8,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "cleanr_run",
			"arguments": map[string]any{
				"config_path":   path,
				"report_format": "json",
			},
		},
	})
	structured := resp["result"].(map[string]any)["structuredContent"].(map[string]any)
	if structured["target_name"] != cfg.Target.Name || structured["report_format"] != "json" {
		t.Fatalf("unexpected config_path run result: %#v", structured)
	}
}
