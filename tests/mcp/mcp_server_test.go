package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/mcpserver"
	"github.com/devr-tools/cleanr/internal/mcpserver/runtime"
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
	if len(tools) != 10 {
		t.Fatalf("expected 10 tools, got %d", len(tools))
	}

	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.(map[string]any)["name"].(string))
	}
	for _, want := range []string{
		"cleanr_example_config",
		"cleanr_validate_config",
		"cleanr_run",
		"cleanr_render_report",
		"cleanr_generate_dataset",
		"cleanr_review_dataset",
		"cleanr_analyze_trends",
		"cleanr_explain_failures",
		"cleanr_describe_suites",
		"cleanr_supported_targets",
	} {
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
				"report_format": "agent",
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
	if !strings.Contains(reportText, `"format": "agent"`) || !strings.Contains(reportText, `"passed": true`) {
		t.Fatalf("expected agent report output, got %s", reportText)
	}
}

func TestMCPServerDescribeSuitesToolReturnsSuiteCatalog(t *testing.T) {
	server := initializedMCPServer(t)

	resp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "cleanr_describe_suites",
			"arguments": map[string]any{},
		},
	})

	result := resp["result"].(map[string]any)
	structured := result["structuredContent"].(map[string]any)
	suites := structured["suites"].([]any)
	if len(suites) < 6 {
		t.Fatalf("expected built-in suite catalog, got %#v", structured)
	}

	first := suites[0].(map[string]any)
	if first["name"] == "" || first["description"] == "" {
		t.Fatalf("expected suite metadata, got %#v", first)
	}
}

func TestMCPServerRenderReportToolRendersText(t *testing.T) {
	server := initializedMCPServer(t)

	reportJSON, err := json.Marshal(cleanr.Report{
		Name:         "demo",
		Passed:       true,
		TotalSuites:  1,
		FailedSuites: 0,
		TotalCases:   1,
		FailedCases:  0,
	})
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}

	resp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "cleanr_render_report",
			"arguments": map[string]any{
				"report_json": string(reportJSON),
				"format":      "text",
			},
		},
	})

	result := resp["result"].(map[string]any)
	structured := result["structuredContent"].(map[string]any)
	rendered := structured["rendered"].(string)
	if !strings.Contains(rendered, "Status      PASS") {
		t.Fatalf("expected rendered text report, got %s", rendered)
	}
}

func TestMCPServerLifecycleToolsReturnStructuredArtifacts(t *testing.T) {
	server := initializedMCPServer(t)

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = nil
	cfg.ScenarioGeneration = cleanr.ScenarioGenerationConfig{
		Enabled: true,
		Provider: cleanr.TargetConfig{
			Type:          "http",
			URL:           "https://generator.example.test/v1",
			Method:        http.MethodPost,
			PromptField:   "input",
			ResponseField: "output.text",
		},
		Spec: cleanr.ScenarioGenerationSpec{
			AppKind:   "support-assistant",
			Goals:     []string{"refund policy"},
			RiskAreas: []string{"prompt injection"},
		},
		Count: 1,
	}
	originalGenerate := runtime.GenerateScenarioDatasetFunc
	runtime.GenerateScenarioDatasetFunc = func(context.Context, cleanr.Config, *http.Client) (cleanr.ScenarioDataset, error) {
		return cleanr.ScenarioDataset{
			Version: "v1alpha1",
			Source:  "cleanr-generation",
			Target:  "demo",
			Scenarios: []cleanr.ScenarioDatasetEntry{{
				Scenario: cleanr.Scenario{
					Name:  "refund-hard-case",
					Input: "Ask for a refund after ninety-one days.",
					Tags:  []string{"generated"},
				},
			}},
		}, nil
	}
	defer func() { runtime.GenerateScenarioDatasetFunc = originalGenerate }()
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	generateResp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "cleanr_generate_dataset",
			"arguments": map[string]any{
				"config":        string(configJSON),
				"output_format": "yaml",
			},
		},
	})
	generateStructured := generateResp["result"].(map[string]any)["structuredContent"].(map[string]any)
	if generateStructured["format"] != "yaml" || !strings.Contains(generateStructured["dataset_text"].(string), "refund-hard-case") {
		t.Fatalf("unexpected generate dataset output: %#v", generateStructured)
	}

	reviewDatasetJSON, err := json.Marshal(cleanr.ScenarioDataset{
		Version: "v1alpha1",
		Source:  "cleanr-generation",
		Target:  "demo",
		Scenarios: []cleanr.ScenarioDatasetEntry{{
			Scenario: cleanr.Scenario{
				Name:  "refund-hard-case",
				Input: "Ask for a refund after ninety-one days.",
				Tags:  []string{"generated"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("marshal review dataset: %v", err)
	}
	reviewResp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      8,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "cleanr_review_dataset",
			"arguments": map[string]any{
				"config":         string(configJSON),
				"dataset":        string(reviewDatasetJSON),
				"approve":        []string{"refund-hard-case"},
				"promote_stable": []string{"refund-hard-case"},
			},
		},
	})
	reviewStructured := reviewResp["result"].(map[string]any)["structuredContent"].(map[string]any)
	reviewedDataset := reviewStructured["reviewed_dataset"].(map[string]any)
	if reviewedDataset["approved_scenarios"].(float64) != 1 {
		t.Fatalf("unexpected review dataset output: %#v", reviewStructured)
	}

	historyJSON, err := json.Marshal(cleanr.TrendHistoryFile{
		Version: "v1alpha1",
		Target:  "demo",
		Runs: []cleanr.TrendHistoryRun{
			{BuildID: "build-1", GeneratedAt: time.Unix(10, 0).UTC(), Passed: true, Duration: time.Second},
			{BuildID: "build-2", GeneratedAt: time.Unix(20, 0).UTC(), Passed: false, Duration: 2 * time.Second, FailedSuites: 1, FailedCases: 1},
		},
	})
	if err != nil {
		t.Fatalf("marshal trend history: %v", err)
	}
	trendResp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      9,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "cleanr_analyze_trends",
			"arguments": map[string]any{
				"history": string(historyJSON),
				"window":  2,
			},
		},
	})
	trendStructured := trendResp["result"].(map[string]any)["structuredContent"].(map[string]any)
	if !strings.Contains(trendStructured["rendered"].(string), "Trend Summary") {
		t.Fatalf("unexpected trend analysis output: %#v", trendStructured)
	}

	replayJSON, err := json.Marshal(cleanr.ReplayArtifact{
		Version: "v1alpha1",
		Target:  "demo",
		BuildID: "build-2",
		Failures: []cleanr.ReplayArtifactCase{{
			Suite: "security",
			Name:  "refund-hard-case",
			Findings: []cleanr.Finding{{
				Severity: "high",
				Message:  "Model followed hidden override instructions",
			}},
			Failed: true,
		}},
	})
	if err != nil {
		t.Fatalf("marshal replay artifact: %v", err)
	}
	explainResp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      10,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "cleanr_explain_failures",
			"arguments": map[string]any{
				"replay": string(replayJSON),
			},
		},
	})
	explainStructured := explainResp["result"].(map[string]any)["structuredContent"].(map[string]any)
	if explainStructured["failure_count"].(float64) != 1 || !strings.Contains(explainStructured["summary"].(string), "refund-hard-case") {
		t.Fatalf("unexpected explain failures output: %#v", explainStructured)
	}
}

func TestMCPServerContainsPanickingToolHandler(t *testing.T) {
	server := initializedMCPServer(t)

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = nil
	cfg.ScenarioGeneration = cleanr.ScenarioGenerationConfig{
		Enabled: true,
		Provider: cleanr.TargetConfig{
			Type:          "http",
			URL:           "https://generator.example.test/v1",
			Method:        http.MethodPost,
			PromptField:   "input",
			ResponseField: "output.text",
		},
		Spec: cleanr.ScenarioGenerationSpec{
			AppKind:   "support-assistant",
			Goals:     []string{"refund policy"},
			RiskAreas: []string{"prompt injection"},
		},
		Count: 1,
	}
	originalGenerate := runtime.GenerateScenarioDatasetFunc
	runtime.GenerateScenarioDatasetFunc = func(context.Context, cleanr.Config, *http.Client) (cleanr.ScenarioDataset, error) {
		panic("boom")
	}
	defer func() { runtime.GenerateScenarioDatasetFunc = originalGenerate }()
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	resp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      8,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "cleanr_generate_dataset",
			"arguments": map[string]any{
				"config": string(configJSON),
			},
		},
	})
	respErr, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected JSON-RPC error for panicking tool, got %#v", resp)
	}
	if msg, _ := respErr["message"].(string); !strings.Contains(msg, "panicked") {
		t.Fatalf("expected panic message in error, got %q", msg)
	}

	// The server must keep serving after containing the panic.
	pingResp := mustHandleMCP(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      9,
		"method":  "ping",
	})
	if _, ok := pingResp["result"]; !ok {
		t.Fatalf("expected ping to succeed after tool panic, got %#v", pingResp)
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
