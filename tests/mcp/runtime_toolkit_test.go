package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/mcpserver/catalog"
	"github.com/devr-tools/cleanr/internal/mcpserver/runtime"
	"github.com/devr-tools/cleanr/internal/mcpserver/toolkit"
	"github.com/devr-tools/cleanr/internal/mcpserver/tools"
)

func TestToolkitCatalogRuntimeAndRegistryCoverage(t *testing.T) {
	t.Parallel()

	if len(tools.Definitions()) < 10 {
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
	if len(targetCatalog.Targets) != 5 {
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

	generateCfg := cleanr.ExampleConfig()
	generateCfg.Scenarios = nil
	generateCfg.ScenarioGeneration = cleanr.ScenarioGenerationConfig{
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
					Name:  "generated-refund",
					Input: "Explain the refund exception path.",
					Tags:  []string{"generated"},
				},
			}},
		}, nil
	}
	defer func() { runtime.GenerateScenarioDatasetFunc = originalGenerate }()
	generateConfigJSON, err := json.Marshal(generateCfg)
	if err != nil {
		t.Fatalf("marshal generation config: %v", err)
	}
	generateResult, err := runtime.GenerateDataset(context.Background(), map[string]any{
		"config":        string(generateConfigJSON),
		"output_format": "yaml",
	})
	if err != nil {
		t.Fatalf("generate dataset: %v", err)
	}
	generateOut := generateResult.StructuredContent.(toolkit.GenerateDatasetOutput)
	if generateOut.Format != "yaml" || len(generateOut.Dataset.Scenarios) != 1 || !strings.Contains(generateOut.DatasetText, "generated-refund") {
		t.Fatalf("unexpected generate dataset output: %+v", generateOut)
	}

	reviewCfg := cleanr.ExampleConfig()
	reviewCfg.Scenarios = []cleanr.Scenario{{
		Name:  "existing-faq",
		Input: "How do refunds work?",
	}}
	reviewConfigJSON, err := json.Marshal(reviewCfg)
	if err != nil {
		t.Fatalf("marshal review config: %v", err)
	}
	datasetJSON, err := json.Marshal(cleanr.ScenarioDataset{
		Version: "v1alpha1",
		Source:  "cleanr-generation",
		Target:  "demo",
		Scenarios: []cleanr.ScenarioDatasetEntry{{
			Scenario: cleanr.Scenario{
				Name:  "fresh-edge-case",
				Input: "Escalate the refund request after 91 days.",
				Tags:  []string{"generated"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("marshal dataset: %v", err)
	}
	reviewResult, err := runtime.ReviewDataset(context.Background(), map[string]any{
		"config":         string(reviewConfigJSON),
		"dataset":        string(datasetJSON),
		"approve":        []string{"fresh-edge-case"},
		"promote_stable": []string{"fresh-edge-case"},
		"output_format":  "json",
	})
	if err != nil {
		t.Fatalf("review dataset: %v", err)
	}
	reviewOut := reviewResult.StructuredContent.(toolkit.ReviewDatasetOutput)
	if reviewOut.ReviewedDataset.ApprovedScenarios != 1 || len(reviewOut.ApprovedDataset.Scenarios) != 1 || !strings.Contains(reviewOut.ApprovedDatasetText, `"fresh-edge-case"`) {
		t.Fatalf("unexpected review dataset output: %+v", reviewOut)
	}

	historyJSON, err := json.Marshal(cleanr.TrendHistoryFile{
		Version: "v1alpha1",
		Target:  "demo",
		Runs: []cleanr.TrendHistoryRun{
			{BuildID: "build-1", GeneratedAt: time.Unix(10, 0).UTC(), Passed: true, Duration: time.Second, FailedSuites: 0, FailedCases: 0},
			{BuildID: "build-2", GeneratedAt: time.Unix(20, 0).UTC(), Passed: false, Duration: 2 * time.Second, FailedSuites: 1, FailedCases: 2},
		},
	})
	if err != nil {
		t.Fatalf("marshal trend history: %v", err)
	}
	trendResult, err := runtime.AnalyzeTrends(context.Background(), map[string]any{
		"history":       string(historyJSON),
		"window":        2,
		"output_format": "json",
	})
	if err != nil {
		t.Fatalf("analyze trends: %v", err)
	}
	trendOut := trendResult.StructuredContent.(toolkit.AnalyzeTrendsOutput)
	if trendOut.Format != "json" || trendOut.Analysis.WindowSize != 2 || !strings.Contains(trendOut.Rendered, `"window_size":2`) {
		t.Fatalf("unexpected trend analysis output: %+v", trendOut)
	}

	replayJSON, err := json.Marshal(cleanr.ReplayArtifact{
		Version: "v1alpha1",
		Target:  "demo",
		BuildID: "build-2",
		Failures: []cleanr.ReplayArtifactCase{{
			Suite: "security",
			Name:  "prompt-injection",
			Findings: []cleanr.Finding{{
				Severity: "high",
				Message:  "Model followed hidden override instructions",
			}},
			Evidence: map[string]any{"unsupported_claim": "refund exceptions"},
			Failed:   true,
		}},
	})
	if err != nil {
		t.Fatalf("marshal replay artifact: %v", err)
	}
	explainResult, err := runtime.ExplainFailures(context.Background(), map[string]any{
		"replay": string(replayJSON),
	})
	if err != nil {
		t.Fatalf("explain failures: %v", err)
	}
	explainOut := explainResult.StructuredContent.(toolkit.ExplainFailuresOutput)
	if explainOut.FailureCount != 1 || len(explainOut.Explanations) != 1 || !strings.Contains(explainOut.Summary, "prompt-injection") {
		t.Fatalf("unexpected explain failures output: %+v", explainOut)
	}
}

func TestToolkitHelpersCoverFormattingAndRunBranches(t *testing.T) {
	t.Parallel()

	if toolkit.NormalizeConfigFormat(" yml ") != "yaml" || toolkit.NormalizeConfigFormat("json") != "json" {
		t.Fatal("unexpected config format normalization")
	}
	if toolkit.NormalizeReportFormat(" junit ") != "junit" || toolkit.NormalizeReportFormat("agent") != "agent" || toolkit.NormalizeReportFormat("nope") != "text" {
		t.Fatal("unexpected report format normalization")
	}
	if toolkit.NormalizeDataFormat(" yml ") != "yaml" || toolkit.NormalizeTrendFormat("nope") != "text" {
		t.Fatal("unexpected data or trend format normalization")
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

	dataset, err := toolkit.LoadScenarioDatasetSource("", `{"version":"v1alpha1","scenarios":[{"scenario":{"name":"demo","input":"hello"}}]}`, "json")
	if err != nil || len(dataset.Scenarios) != 1 {
		t.Fatalf("load scenario dataset source: dataset=%+v err=%v", dataset, err)
	}
	if _, err := toolkit.LoadScenarioDatasetSource("", "", "json"); err == nil {
		t.Fatal("expected dataset source error")
	}

	policy, err := toolkit.LoadDatasetReviewPolicySource("", `{"version":"v1alpha1","rules":[{"action":"approve","statuses":["new"]}]}`, "json")
	if err != nil || policy == nil || len(policy.Rules) != 1 {
		t.Fatalf("load policy source: policy=%+v err=%v", policy, err)
	}
	if policy, err := toolkit.LoadDatasetReviewPolicySource("", "", "json"); err != nil || policy != nil {
		t.Fatalf("expected nil optional policy, got policy=%+v err=%v", policy, err)
	}

	history, err := toolkit.LoadTrendHistorySource("", `{"version":"v1alpha1","target":"demo","runs":[]}`, "json")
	if err != nil || history.Target != "demo" {
		t.Fatalf("load trend history source: history=%+v err=%v", history, err)
	}
	analysis, err := toolkit.AnalyzeTrendHistorySource("", `{"version":"v1alpha1","target":"demo","runs":[]}`, "json", 0)
	if err != nil || analysis.Target != "demo" {
		t.Fatalf("analyze trend history source: analysis=%+v err=%v", analysis, err)
	}

	replay, err := toolkit.LoadReplayArtifactSource("", `{"version":"v1alpha1","target":"demo","failures":[]}`, "json")
	if err != nil || replay.Target != "demo" {
		t.Fatalf("load replay source: replay=%+v err=%v", replay, err)
	}

	encodedYAML, err := toolkit.EncodeData(map[string]any{"name": "demo"}, "yaml")
	if err != nil || !strings.Contains(encodedYAML, "name: demo") {
		t.Fatalf("encode yaml: %q err=%v", encodedYAML, err)
	}

	trendRendered, err := toolkit.RenderTrendAnalysis(cleanr.TrendAnalysis{Target: "demo"}, "text")
	if err != nil || !strings.Contains(trendRendered, "Trend Summary") {
		t.Fatalf("render trend analysis: %q err=%v", trendRendered, err)
	}
}
