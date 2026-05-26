package tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/mcpserver/catalog"
	"github.com/devr-tools/cleanr/internal/mcpserver/runtime"
	"github.com/devr-tools/cleanr/internal/mcpserver/toolkit"
	"github.com/devr-tools/cleanr/internal/mcpserver/tools"
)

func TestToolkitCatalogRuntimeAndRegistryCoverage(t *testing.T) {
	t.Parallel()

	if len(tools.Definitions()) < 6 {
		t.Fatalf("expected tool definitions")
	}
	if _, err := tools.Call(context.Background(), "unknown", nil); err == nil {
		t.Fatal("expected unknown tool error")
	}

	targets, err := catalog.SupportedTargets(context.Background(), nil)
	if err != nil {
		t.Fatalf("supported targets: %v", err)
	}
	targetCatalog := targets.StructuredContent.(toolkit.TargetCatalogOutput)
	if len(targetCatalog.Targets) != 3 {
		t.Fatalf("unexpected target catalog: %+v", targetCatalog)
	}

	suites, err := catalog.DescribeSuites(context.Background(), nil)
	if err != nil {
		t.Fatalf("describe suites: %v", err)
	}
	suiteCatalog := suites.StructuredContent.(toolkit.SuiteCatalogOutput)
	if len(suiteCatalog.Suites) == 0 {
		t.Fatalf("unexpected suite catalog: %+v", suiteCatalog)
	}

	if _, err := runtime.ExampleConfig(context.Background(), map[string]any{"format": func() {}}); err == nil {
		t.Fatal("expected example-config arg decode error")
	}
	exampleResult, err := runtime.ExampleConfig(context.Background(), map[string]any{"format": "yaml"})
	if err != nil {
		t.Fatalf("example config: %v", err)
	}
	exampleOut := exampleResult.StructuredContent.(toolkit.ExampleConfigOutput)
	if exampleOut.Format != "yaml" || !strings.Contains(exampleOut.Config, "target:") {
		t.Fatalf("unexpected example config output: %+v", exampleOut)
	}

	validateResult, err := runtime.ValidateConfig(context.Background(), map[string]any{
		"config": `{"target":{"url":"https://example.com","prompt_field":"input","response_field":"output.text"},"scenarios":[{"name":"demo","input":"hello"}]}`,
	})
	if err != nil {
		t.Fatalf("validate config: %v", err)
	}
	validateOut := validateResult.StructuredContent.(toolkit.ValidateConfigOutput)
	if !validateOut.Valid || validateOut.ScenarioCount != 1 {
		t.Fatalf("unexpected validate result: %+v", validateOut)
	}

	runResult, err := runtime.Run(context.Background(), map[string]any{
		"config":        `{"target":{"url":"https://example.com","prompt_field":"input","response_field":"output.text"},"scenarios":[{"name":"demo","input":"hello"}],"suites":{"prompt_injection":{"enabled":false},"security":{"enabled":false},"load":{"enabled":false},"chaos":{"enabled":false},"drift":{"enabled":false},"token_optimization":{"enabled":false}}}`,
		"report_format": "json",
		"timeout_ms":    1,
	})
	if err != nil {
		t.Fatalf("run tool: %v", err)
	}
	runOut := runResult.StructuredContent.(toolkit.RunOutput)
	if runOut.ReportFormat != "json" {
		t.Fatalf("unexpected run output: %+v", runOut)
	}

	reportJSON, err := json.Marshal(runOut.Report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	renderResult, err := runtime.RenderReport(context.Background(), map[string]any{
		"report_json": string(reportJSON),
		"format":      "junit",
	})
	if err != nil {
		t.Fatalf("render report: %v", err)
	}
	renderOut := renderResult.StructuredContent.(toolkit.RenderReportOutput)
	if renderOut.Format != "junit" || !strings.Contains(renderOut.Rendered, "<testsuites>") {
		t.Fatalf("unexpected render output: %+v", renderOut)
	}
	if _, err := runtime.RenderReport(context.Background(), map[string]any{"report_json": "{"}); err == nil {
		t.Fatal("expected invalid report json error")
	}
}

func TestToolkitHelpersCoverFormattingAndRunBranches(t *testing.T) {
	t.Parallel()

	if toolkit.NormalizeConfigFormat(" yml ") != "yaml" || toolkit.NormalizeConfigFormat("json") != "json" {
		t.Fatal("unexpected config format normalization")
	}
	if toolkit.NormalizeReportFormat(" junit ") != "junit" || toolkit.NormalizeReportFormat("nope") != "text" {
		t.Fatal("unexpected report format normalization")
	}

	if err := toolkit.DecodeArgs(map[string]any{"bad": func() {}}, &toolkit.ExampleConfigArgs{}); err == nil {
		t.Fatal("expected decode args error")
	}
	result := toolkit.StructuredToolResult(map[string]any{"ok": true}, "")
	if result.Content[0].Text != "{}" {
		t.Fatalf("unexpected empty structured tool text: %+v", result)
	}

	cfg := cleanr.ExampleConfig()
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	runOut, err := toolkit.RunWithConfig(context.Background(), cfg, "text", 1)
	if err != nil {
		t.Fatalf("run with config: %v", err)
	}
	if !runOut.Passed || runOut.ReportFormat != "text" {
		t.Fatalf("unexpected runWithConfig output: %+v", runOut)
	}

	rendered, err := toolkit.RenderReport(runOut.Report, "json")
	if err != nil {
		t.Fatalf("render report: %v", err)
	}
	if !strings.Contains(rendered, `"passed": true`) {
		t.Fatalf("unexpected rendered report: %s", rendered)
	}
}
