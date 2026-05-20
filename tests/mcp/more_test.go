package tests

import (
	"context"
	"strings"
	"testing"

	"cleanr/internal/mcpserver"
	toolspkg "cleanr/internal/mcpserver/tools"
)

func TestMCPServerAdditionalRequestBranches(t *testing.T) {
	server := mcpserver.New()

	resp := server.HandleLine(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":"bad"}`))
	if resp == nil || resp.Error == nil || !strings.Contains(resp.Error.Message, "invalid initialize params") {
		t.Fatalf("expected invalid initialize params error, got %#v", resp)
	}

	server = initializedMCPServer(t)
	resp = server.HandleLine(context.Background(), []byte(`{"jsonrpc":"2.0","method":"notifications/custom"}`))
	if resp != nil {
		t.Fatalf("expected custom notification to be ignored, got %#v", resp)
	}

	resp = server.HandleLine(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":"bad"}`))
	if resp == nil || resp.Error == nil || !strings.Contains(resp.Error.Message, "invalid tool call params") {
		t.Fatalf("expected invalid tool call params error, got %#v", resp)
	}

	resp = server.HandleLine(context.Background(), []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"   "}}`))
	if resp == nil || resp.Error == nil || !strings.Contains(resp.Error.Message, "tool name is required") {
		t.Fatalf("expected missing tool name error, got %#v", resp)
	}

	resp = server.HandleLine(context.Background(), []byte(`{"jsonrpc":"2.0","id":4,"method":"missing"}`))
	if resp == nil || resp.Error == nil || !strings.Contains(resp.Error.Message, "method not found") {
		t.Fatalf("expected method not found error, got %#v", resp)
	}
}

func TestMCPToolsDefinitionsExposeCatalogAndRuntimeTools(t *testing.T) {
	defs := toolspkg.Definitions()
	if len(defs) < 6 {
		t.Fatalf("expected MCP tool definitions, got %d", len(defs))
	}
	var sawRenderReport bool
	var sawSupportedTargets bool
	for _, def := range defs {
		if def.Name == "cleanr_render_report" {
			sawRenderReport = true
		}
		if def.Name == "cleanr_supported_targets" {
			sawSupportedTargets = true
		}
	}
	if !sawRenderReport || !sawSupportedTargets {
		t.Fatalf("expected render_report and supported_targets in definitions: %+v", defs)
	}
}
