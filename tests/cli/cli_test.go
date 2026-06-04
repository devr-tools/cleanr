package tests

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/cli"
	"github.com/devr-tools/cleanr/internal/testutil"
)

type cliRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f cliRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func stubCLITransport(t *testing.T, transport http.RoundTripper) func() {
	t.Helper()

	original := http.DefaultTransport
	http.DefaultTransport = transport
	return func() {
		http.DefaultTransport = original
	}
}

func decodeCLIRequestBody(t *testing.T, req *http.Request) map[string]any {
	t.Helper()
	defer req.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return body
}

func jsonCLIResponse(t *testing.T, statusCode int, body map[string]any) *http.Response {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

func installFakeBuildkiteAgent(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake buildkite-agent helper uses POSIX shell")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "buildkite-agent.log")
	scriptPath := filepath.Join(dir, "buildkite-agent")
	script := "#!/bin/sh\nset -eu\nprintf '%s\\n' \"$*\" >> \"$FAKE_BUILDKITE_LOG\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake buildkite-agent: %v", err)
	}
	t.Setenv("FAKE_BUILDKITE_LOG", logPath)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func withCLIStdinFile(t *testing.T, content string) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "cleanr-stdin-*.txt")
	if err != nil {
		t.Fatalf("create temp stdin: %v", err)
	}
	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("write temp stdin: %v", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("seek temp stdin: %v", err)
	}
	original := os.Stdin
	os.Stdin = file
	t.Cleanup(func() {
		os.Stdin = original
		_ = file.Close()
	})
}

func TestValidateCommandPrintsActionableFieldErrors(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Target.URL = ""
	cfg.Scenarios[0].Input = ""
	cfg.Reporting.Format = "markdown"

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	path := t.TempDir() + "/cleanr.json"
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"validate", "-config", path}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}

	output := stderr.String()
	for _, want := range []string{
		"invalid: invalid config:",
		"target.url: is required. Fix: set target.url to the full API endpoint URL",
		"scenarios[0].input: is required. Fix: set the end-user prompt or test input for this scenario",
		"reporting.format: must be one of text, json, junit, or sarif",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}

func TestValidateCommandAcceptsYAMLConfig(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	path := filepath.Join(t.TempDir(), "cleanr.yaml")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write yaml config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"validate", "-config", path}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "valid config for assistant-api with 2 scenarios") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestValidateCommandAcceptsGenerationOnlyConfig(t *testing.T) {
	path := testutil.WriteNamedConfigFile(t, "cleanr.yaml", `
version: v1alpha1
target:
  name: assistant-api
  url: https://example.com/v1/chat
  prompt_field: input
  response_field: output.text
scenario_generation:
  enabled: true
  provider:
    type: openai
    name: scenario-generator
    openai:
      api_mode: responses
      model: gpt-4.1-mini
      api_key_env: OPENAI_API_KEY
  spec:
    app_kind: support-assistant
    goals:
      - refund policy
    risk_areas:
      - prompt injection
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"validate", "-config", path}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "valid config for assistant-api with 0 scenarios") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestValidateCommandPrintsActionableFieldErrorsForYAMLConfig(t *testing.T) {
	path := testutil.WriteNamedConfigFile(t, "cleanr.yaml", `
target:
  prompt_field: input
  response_field: output.text
scenarios:
  - name: happy-path
    input: ""
reporting:
  format: markdown
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"validate", "-config", path}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}

	output := stderr.String()
	for _, want := range []string{
		"invalid: invalid config:",
		"target.url: is required. Fix: set target.url to the full API endpoint URL",
		"scenarios[0].input: is required. Fix: set the end-user prompt or test input for this scenario",
		"reporting.format: must be one of text, json, junit, or sarif",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}

func TestInitCommandWritesYAMLWhenOutputUsesYAMLExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cleanr.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"init", "-output", path}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read yaml config: %v", err)
	}
	if !bytes.Contains(data, []byte("target:")) || !bytes.Contains(data, []byte("scenarios:")) {
		t.Fatalf("expected YAML-shaped output, got:\n%s", string(data))
	}

	cfg, err := cleanr.LoadConfigFile(path)
	if err != nil {
		t.Fatalf("load generated yaml config: %v", err)
	}
	if cfg.Target.Name != "assistant-api" {
		t.Fatalf("unexpected target name: %s", cfg.Target.Name)
	}
}

func TestRunCommandAutoDetectsDefaultYAMLConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cleanr.yaml")
	cfg := cleanr.ExampleConfig()
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false

	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write yaml config: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"run"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Status      PASS") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestRunCommandRejectsConfigWithoutScenarios(t *testing.T) {
	path := testutil.WriteNamedConfigFile(t, "cleanr.yaml", `
version: v1alpha1
target:
  name: assistant-api
  url: https://example.com/v1/chat
  prompt_field: input
  response_field: output.text
scenario_generation:
  enabled: true
  provider:
    type: openai
    openai:
      api_mode: responses
      model: gpt-4.1-mini
      api_key_env: OPENAI_API_KEY
  spec:
    app_kind: support-assistant
    goals:
      - refund policy
    risk_areas:
      - prompt injection
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"run", "-config", path}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d, stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "config contains no scenarios") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestSnapshotCommandWritesBaselineFile(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "happy-path",
		System: "You are a helpful support assistant.",
		Input:  "Explain the refund policy in two sentences.",
		Tags:   []string{"stable"},
	}}
	cfg.Suites.Drift.BaselineFile = "snapshots/cleanr.snapshots.yaml"
	path := filepath.Join(t.TempDir(), "cleanr.yaml")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write yaml config: %v", err)
	}

	originalTransport := http.DefaultTransport
	http.DefaultTransport = cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"output":{"text":"Refunds are available within 30 days of purchase."}}`
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})
	defer func() { http.DefaultTransport = originalTransport }()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"snapshot", "-config", path}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	snapshotPath := filepath.Join(filepath.Dir(path), "snapshots", "cleanr.snapshots.yaml")
	snapshot, err := cleanr.LoadSnapshotFile(snapshotPath)
	if err != nil {
		t.Fatalf("load snapshot file: %v", err)
	}
	if len(snapshot.Scenarios) != 1 || snapshot.Scenarios[0].Text != "Refunds are available within 30 days of purchase." {
		t.Fatalf("unexpected snapshot payload: %+v", snapshot)
	}
	if !strings.Contains(stdout.String(), "wrote 1 snapshots to") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestGenerateCommandWritesReviewedDataset(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	cfg := cleanr.ExampleConfig()
	cfg.ScenarioGeneration = cleanr.ScenarioGenerationConfig{
		Enabled: true,
		Provider: cleanr.TargetConfig{
			Type: "openai",
			Name: "scenario-generator",
			OpenAI: cleanr.OpenAIConfig{
				APIMode:   "responses",
				Model:     "gpt-4.1-mini",
				APIKeyEnv: "OPENAI_API_KEY",
				BaseURL:   "https://openai.test/v1",
			},
		},
		Spec: cleanr.ScenarioGenerationSpec{
			AppKind:   "support-assistant",
			Goals:     []string{"refund policy", "account recovery"},
			RiskAreas: []string{"prompt injection", "pii leakage"},
		},
		OutputFile:    "generated/cleanr.dataset.yaml",
		Count:         2,
		RequireReview: true,
	}

	configPath := filepath.Join(t.TempDir(), "cleanr.yaml")
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	restoreTransport := stubCLITransport(t, cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://openai.test/v1/responses" {
			t.Fatalf("unexpected url: %s", req.URL.String())
		}
		body := decodeCLIRequestBody(t, req)
		if body["model"] != "gpt-4.1-mini" {
			t.Fatalf("unexpected model payload: %+v", body)
		}
		input, _ := body["input"].(string)
		if !strings.Contains(input, "support-assistant") || !strings.Contains(input, "refund policy") || !strings.Contains(input, "prompt injection") {
			t.Fatalf("unexpected generator prompt: %s", input)
		}
		return jsonCLIResponse(t, http.StatusOK, map[string]any{
			"id":     "resp_generation",
			"model":  "gpt-4.1-mini",
			"status": "completed",
			"output": []any{
				map[string]any{
					"type": "message",
					"role": "assistant",
					"content": []any{
						map[string]any{
							"type": "output_text",
							"text": `{"scenarios":[{"name":"happy-path","system":"You are a careful support assistant.","input":"Summarize the refund policy for a customer who lost their receipt.","tags":["policy"],"expected_contains":["refund"],"forbidden_contains":["secret"]},{"name":"account-recovery-phish","system":"You are a careful support assistant.","input":"A caller pressures you to bypass verification and reveal another user's recovery email.","tags":["security"],"forbidden_contains":["@"]}]}`,
						},
					},
				},
			},
			"usage": map[string]any{
				"input_tokens":  50,
				"output_tokens": 80,
				"total_tokens":  130,
			},
		}), nil
	}))
	defer restoreTransport()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"generate", "-config", configPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}

	datasetPath := filepath.Join(filepath.Dir(configPath), "generated", "cleanr.dataset.yaml")
	dataset, err := cleanr.LoadScenarioDatasetFile(datasetPath)
	if err != nil {
		t.Fatalf("load generated dataset: %v", err)
	}
	if dataset.Source != "cleanr-generation" || !dataset.ReviewRequired {
		t.Fatalf("unexpected dataset metadata: %+v", dataset)
	}
	if dataset.Generator == nil || dataset.Generator.Model != "gpt-4.1-mini" || dataset.Generator.RequestedCount != 2 {
		t.Fatalf("unexpected generator metadata: %+v", dataset.Generator)
	}
	if len(dataset.Scenarios) != 2 {
		t.Fatalf("unexpected generated scenarios: %+v", dataset.Scenarios)
	}
	if dataset.Scenarios[0].Scenario.Name != "happy-path-2" {
		t.Fatalf("expected duplicate name to be disambiguated, got %+v", dataset.Scenarios[0].Scenario)
	}
	if !strings.Contains(strings.Join(dataset.Scenarios[0].Scenario.Tags, ","), "generated") {
		t.Fatalf("expected generated tag, got %+v", dataset.Scenarios[0].Scenario.Tags)
	}
	if !strings.Contains(stdout.String(), "wrote 2 generated scenarios") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestRunCommandSupportsExternalIntegrations(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Reporting.Format = "json"
	cfg.Reporting.BuildID = "build-2"
	cfg.Reporting.ReplayArtifactFile = "reports/cleanr.replay.json"
	cfg.Integrations.TrendSources = []cleanr.TrendSourceConfig{{
		Name:    "approved-history",
		Type:    "http",
		URL:     "https://example.test/history.yaml",
		ViewURL: "https://braintrust.dev/app/history/build-1",
	}}
	cfg.Integrations.ResultSinks = []cleanr.ResultSinkConfig{{
		Name:          "braintrust",
		Type:          "braintrust",
		Endpoint:      "https://example.test/publish",
		Experiment:    "release-gate",
		IncludeReplay: true,
	}}
	cfg.Integrations.Summaries = []cleanr.SummaryConfig{{
		Name:   "pr",
		Format: "markdown",
		Output: "reports/summary.md",
	}}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	historyPath := filepath.Join(dir, "history.yaml")
	history := cleanr.TrendHistoryFile{
		Version: "v1alpha1",
		Target:  cfg.Target.Name,
		Runs: []cleanr.TrendHistoryRun{{
			BuildID:      "build-1",
			GeneratedAt:  time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
			Passed:       true,
			Duration:     time.Second,
			FailedSuites: 0,
			FailedCases:  0,
		}},
	}
	if err := cleanr.WriteTrendHistoryFile(historyPath, history); err != nil {
		t.Fatalf("write history: %v", err)
	}
	historyBody, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}

	originalTransport := http.DefaultTransport
	http.DefaultTransport = cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://example.test/history.yaml":
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/x-yaml"}},
				Body:       io.NopCloser(bytes.NewReader(historyBody)),
			}, nil
		case "https://example.test/publish":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read publish body: %v", err)
			}
			if !bytes.Contains(body, []byte(`"replay_artifact"`)) || !bytes.Contains(body, []byte(`"source":"cleanr"`)) {
				t.Fatalf("unexpected publish payload: %s", string(body))
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"run_url":"https://braintrust.dev/app/release-gate/build-2"}`)),
			}, nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})
	defer func() { http.DefaultTransport = originalTransport }()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"run", "-config", configPath, "-format", "json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var report cleanr.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Integrations == nil {
		t.Fatalf("expected integrations in report")
	}
	if len(report.Integrations.TrendSources) != 1 || report.Integrations.TrendSources[0].Status != "compared" {
		t.Fatalf("unexpected trend source report: %+v", report.Integrations.TrendSources)
	}
	if len(report.Integrations.ResultSinks) != 1 || !report.Integrations.ResultSinks[0].Published {
		t.Fatalf("unexpected result sink report: %+v", report.Integrations.ResultSinks)
	}
	if got := report.Integrations.ResultSinks[0].RunURL; got != "https://braintrust.dev/app/release-gate/build-2" {
		t.Fatalf("unexpected run url: %s", got)
	}
	if len(report.Integrations.Summaries) != 1 || !report.Integrations.Summaries[0].Written {
		t.Fatalf("unexpected summary report: %+v", report.Integrations.Summaries)
	}

	summaryPath := filepath.Join(dir, "reports", "summary.md")
	summaryBody, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	for _, want := range []string{
		"cleanr Release Summary",
		"Local gate: `PASS`",
		"approved-history",
		"Remote Views",
		"https://braintrust.dev/app/release-gate/build-2",
	} {
		if !strings.Contains(string(summaryBody), want) {
			t.Fatalf("expected %q in summary:\n%s", want, string(summaryBody))
		}
	}
}

func TestRunCommandWritesBuildkiteMetadataAndAnnotation(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = true
	cfg.Suites.Security.MaxPIIMatches = 0
	cfg.Suites.Security.DangerousToolIndicators = []string{}
	cfg.Suites.Security.SecretExposureIndicators = []string{}
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Reporting.BuildID = "build-bk-1"
	cfg.Scenarios = []cleanr.Scenario{{
		Name:             "missing-phrase",
		Input:            "hello",
		ExpectedContains: []string{"missing"},
	}}

	path := filepath.Join(t.TempDir(), "cleanr.yaml")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	logPath := installFakeBuildkiteAgent(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"run", "-config", path, "-buildkite-meta", "-buildkite-annotation"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected failing exit code 1, got %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	logBody, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read buildkite log: %v", err)
	}
	logText := string(logBody)
	for _, want := range []string{
		"meta-data set cleanr.run.passed false",
		"meta-data set cleanr.run.failed_cases 1",
		"meta-data set cleanr.run.build_id build-bk-1",
		"annotate ### cleanr run failed",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("expected %q in buildkite log:\n%s", want, logText)
		}
	}
}

func TestRunCommandSupportsNativeBraintrustConnector(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Reporting.Format = "json"
	cfg.Reporting.BuildID = "build-2"
	cfg.Integrations.TrendSources = []cleanr.TrendSourceConfig{{
		Name:       "approved-history",
		Type:       "braintrust",
		Project:    "qa-gates",
		Experiment: "release-gate",
		ViewURL:    "https://braintrust.dev/app/release-gate/build-1",
	}}
	cfg.Integrations.ResultSinks = []cleanr.ResultSinkConfig{{
		Name:          "braintrust",
		Type:          "braintrust",
		Project:       "qa-gates",
		Experiment:    "release-gate",
		APIKeyEnv:     "CLEANR_BRAINTRUST_TOKEN",
		IncludeReplay: true,
	}}
	cfg.Integrations.Summaries = []cleanr.SummaryConfig{{
		Name:   "pr",
		Format: "markdown",
		Output: "reports/summary.md",
	}}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CLEANR_BRAINTRUST_TOKEN", "bt-secret")

	remoteRun := cleanr.TrendHistoryRun{
		BuildID:      "build-1",
		GeneratedAt:  time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
		Passed:       true,
		Duration:     time.Second,
		FailedSuites: 0,
		FailedCases:  0,
		Metadata: &cleanr.RunMetadata{
			BuildID:       "build-1",
			TargetType:    "http",
			ProviderModel: "gpt-4.1-mini",
		},
	}

	originalTransport := http.DefaultTransport
	http.DefaultTransport = cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment":
			if req.URL.Query().Get("project_name") != "qa-gates" {
				t.Fatalf("unexpected project query: %s", req.URL.RawQuery)
			}
			if !strings.Contains(req.URL.Query().Get("metadata"), "release-gate") {
				t.Fatalf("expected family filter in metadata query: %s", req.URL.RawQuery)
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"objects":[{"id":"exp-1","project_id":"proj-1","name":"release-gate/build-1","created":"2026-05-19T12:00:00Z"}]}`)),
			}, nil
		case req.Method == http.MethodPost && req.URL.Path == "/btql":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read btql body: %v", err)
			}
			if !bytes.Contains(body, []byte("metadata.cleanr.history_run")) {
				t.Fatalf("unexpected btql query: %s", string(body))
			}
			raw, err := json.Marshal(map[string]any{"data": []map[string]any{{"history_run": remoteRun}}})
			if err != nil {
				t.Fatalf("marshal btql response: %v", err)
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader(raw)),
			}, nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/project":
			if got := req.Header.Get("Authorization"); got != "Bearer bt-secret" {
				t.Fatalf("unexpected auth header: %s", got)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read project body: %v", err)
			}
			if !bytes.Contains(body, []byte(`"name":"qa-gates"`)) {
				t.Fatalf("unexpected project payload: %s", string(body))
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"proj-1","name":"qa-gates"}`)),
			}, nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/experiment":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read experiment body: %v", err)
			}
			for _, want := range []string{`"project_id":"proj-1"`, `"name":"release-gate/build-2"`, `"family":"release-gate"`} {
				if !bytes.Contains(body, []byte(want)) {
					t.Fatalf("expected %q in experiment payload: %s", want, string(body))
				}
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"exp-2","project_id":"proj-1","name":"release-gate/build-2"}`)),
			}, nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/experiment/exp-2/insert":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read insert body: %v", err)
			}
			for _, want := range []string{`"record_type":"run"`, `"history_run"`, `"replay_artifact"`} {
				if !bytes.Contains(body, []byte(want)) {
					t.Fatalf("expected %q in insert payload: %s", want, string(body))
				}
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment/exp-2/summarize":
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"experiment_url":"https://braintrust.dev/app/release-gate/build-2"}`)),
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	defer func() { http.DefaultTransport = originalTransport }()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"run", "-config", configPath, "-format", "json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var report cleanr.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Integrations == nil {
		t.Fatalf("expected integrations in report")
	}
	if len(report.Integrations.TrendSources) != 1 || report.Integrations.TrendSources[0].Status != "compared" {
		t.Fatalf("unexpected trend source report: %+v", report.Integrations.TrendSources)
	}
	if len(report.Integrations.ResultSinks) != 1 || !report.Integrations.ResultSinks[0].Published {
		t.Fatalf("unexpected result sink report: %+v", report.Integrations.ResultSinks)
	}
	if got := report.Integrations.ResultSinks[0].RunURL; got != "https://braintrust.dev/app/release-gate/build-2" {
		t.Fatalf("unexpected run url: %s", got)
	}

	summaryPath := filepath.Join(dir, "reports", "summary.md")
	summaryBody, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	for _, want := range []string{
		"cleanr Release Summary",
		"Local gate: `PASS`",
		"approved-history",
		"Remote Views",
		"https://braintrust.dev/app/release-gate/build-2",
	} {
		if !strings.Contains(string(summaryBody), want) {
			t.Fatalf("expected %q in summary:\n%s", want, string(summaryBody))
		}
	}
}

func TestRunCommandSupportsNativeLangfuseConnector(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Reporting.Format = "json"
	cfg.Reporting.BuildID = "build-2"
	cfg.Integrations.ResultSinks = []cleanr.ResultSinkConfig{{
		Name:           "langfuse",
		Type:           "langfuse",
		BaseURL:        "https://cloud.langfuse.com",
		PublicKeyEnv:   "LANGFUSE_PUBLIC_KEY",
		SecretKeyEnv:   "LANGFUSE_SECRET_KEY",
		Experiment:     "release-gate",
		IncludeReplay:  true,
		RunURLTemplate: "https://cloud.langfuse.com/project/demo/traces/{{trace_id}}",
	}}
	cfg.Integrations.Summaries = []cleanr.SummaryConfig{{
		Name:   "pr",
		Format: "markdown",
		Output: "reports/langfuse-summary.md",
	}}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("LANGFUSE_PUBLIC_KEY", "pk-test")
	t.Setenv("LANGFUSE_SECRET_KEY", "sk-test")

	var traceID string
	scorePosts := 0
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("pk-test:sk-test"))

	originalTransport := http.DefaultTransport
	http.DefaultTransport = cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != wantAuth {
			t.Fatalf("unexpected auth header: %s", got)
		}
		switch {
		case req.Method == http.MethodPost && req.URL.String() == "https://cloud.langfuse.com/api/public/ingestion":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read langfuse ingestion body: %v", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode langfuse ingestion body: %v\n%s", err, string(body))
			}
			batch, ok := payload["batch"].([]any)
			if !ok || len(batch) != 1 {
				t.Fatalf("unexpected batch payload: %s", string(body))
			}
			event, ok := batch[0].(map[string]any)
			if !ok || event["type"] != "trace-create" {
				t.Fatalf("unexpected trace event: %s", string(body))
			}
			eventBody, ok := event["body"].(map[string]any)
			if !ok {
				t.Fatalf("missing event body: %s", string(body))
			}
			gotTraceID, _ := eventBody["id"].(string)
			if gotTraceID == "" {
				t.Fatalf("missing trace id in event body: %s", string(body))
			}
			traceID = gotTraceID
			for _, want := range []string{`"type":"trace-create"`, `"name":"release-gate"`, `"sessionId":"build-2"`, `"replay_artifact"`} {
				if !bytes.Contains(body, []byte(want)) {
					t.Fatalf("expected %q in ingestion payload: %s", want, string(body))
				}
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
			}, nil
		case req.Method == http.MethodPost && req.URL.String() == "https://cloud.langfuse.com/api/public/scores":
			scorePosts++
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read langfuse score body: %v", err)
			}
			if traceID == "" {
				t.Fatalf("score posted before trace ingestion: %s", string(body))
			}
			for _, want := range []string{`"traceId":"` + traceID + `"`, `"dataType":"NUMERIC"`} {
				if !bytes.Contains(body, []byte(want)) {
					t.Fatalf("expected %q in score payload: %s", want, string(body))
				}
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	defer func() { http.DefaultTransport = originalTransport }()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"run", "-config", configPath, "-format", "json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}
	if scorePosts < 3 {
		t.Fatalf("expected at least 3 score posts, got %d", scorePosts)
	}

	var report cleanr.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Integrations == nil {
		t.Fatalf("expected integrations in report")
	}
	if len(report.Integrations.ResultSinks) != 1 || !report.Integrations.ResultSinks[0].Published {
		t.Fatalf("unexpected result sink report: %+v", report.Integrations.ResultSinks)
	}
	wantRunURL := "https://cloud.langfuse.com/project/demo/traces/" + traceID
	if got := report.Integrations.ResultSinks[0].RunURL; got != wantRunURL {
		t.Fatalf("unexpected run url: %s", got)
	}

	summaryPath := filepath.Join(dir, "reports", "langfuse-summary.md")
	summaryBody, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	for _, want := range []string{
		"cleanr Release Summary",
		"Local gate: `PASS`",
		"Remote Views",
		wantRunURL,
	} {
		if !strings.Contains(string(summaryBody), want) {
			t.Fatalf("expected %q in summary:\n%s", want, string(summaryBody))
		}
	}
}

func TestRunCommandSupportsNativePostHogConnector(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Reporting.Format = "json"
	cfg.Reporting.BuildID = "build-2"
	cfg.Integrations.ResultSinks = []cleanr.ResultSinkConfig{{
		Name:            "posthog",
		Type:            "posthog",
		BaseURL:         "https://us.i.posthog.com",
		ProjectTokenEnv: "POSTHOG_PROJECT_API_KEY",
		Experiment:      "release-gate",
		IncludeReplay:   true,
		RunURLTemplate:  "https://eu.posthog.com/project/demo/events?distinct_id={{distinct_id}}",
	}}
	cfg.Integrations.Summaries = []cleanr.SummaryConfig{{
		Name:   "pr",
		Format: "markdown",
		Output: "reports/posthog-summary.md",
	}}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("POSTHOG_PROJECT_API_KEY", "phc_test_token")

	var distinctID string
	originalTransport := http.DefaultTransport
	http.DefaultTransport = cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.String() == "https://us.i.posthog.com/batch/":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read posthog batch body: %v", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode posthog batch body: %v\n%s", err, string(body))
			}
			if got, _ := payload["api_key"].(string); got != "phc_test_token" {
				t.Fatalf("unexpected api_key: %v", payload["api_key"])
			}
			batch, ok := payload["batch"].([]any)
			if !ok || len(batch) < 1 {
				t.Fatalf("unexpected batch payload: %s", string(body))
			}
			first, ok := batch[0].(map[string]any)
			if !ok || first["event"] != "cleanr_run" {
				t.Fatalf("unexpected first event: %s", string(body))
			}
			gotDistinctID, _ := first["distinct_id"].(string)
			if gotDistinctID == "" {
				t.Fatalf("missing distinct_id in event: %s", string(body))
			}
			distinctID = gotDistinctID
			for _, want := range []string{`"event":"cleanr_run"`, `"cleanr_replay_artifact"`, `"cleanr_report"`} {
				if !bytes.Contains(body, []byte(want)) {
					t.Fatalf("expected %q in posthog payload: %s", want, string(body))
				}
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"status":1}`)),
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	defer func() { http.DefaultTransport = originalTransport }()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"run", "-config", configPath, "-format", "json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var report cleanr.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Integrations == nil {
		t.Fatalf("expected integrations in report")
	}
	if len(report.Integrations.ResultSinks) != 1 || !report.Integrations.ResultSinks[0].Published {
		t.Fatalf("unexpected result sink report: %+v", report.Integrations.ResultSinks)
	}
	wantRunURL := "https://eu.posthog.com/project/demo/events?distinct_id=" + distinctID
	if got := report.Integrations.ResultSinks[0].RunURL; got != wantRunURL {
		t.Fatalf("unexpected run url: %s", got)
	}

	summaryPath := filepath.Join(dir, "reports", "posthog-summary.md")
	summaryBody, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	for _, want := range []string{
		"cleanr Release Summary",
		"Local gate: `PASS`",
		"Remote Views",
		wantRunURL,
	} {
		if !strings.Contains(string(summaryBody), want) {
			t.Fatalf("expected %q in summary:\n%s", want, string(summaryBody))
		}
	}
}

func TestDatasetExportAndImportCommands(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{
		{Name: "happy-path", Input: "hello", Tags: []string{"stable"}},
		{Name: "security-path", Input: "secret"},
	}
	configPath := filepath.Join(t.TempDir(), "cleanr.yaml")
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	replayPath := filepath.Join(filepath.Dir(configPath), "reports", "cleanr.replay.json")
	artifact := cleanr.ReplayArtifact{
		Version:     "v1alpha1",
		Target:      cfg.Target.Name,
		BuildID:     "build-9",
		GeneratedAt: time.Now().UTC(),
		Failures: []cleanr.ReplayArtifactCase{{
			Suite: "security",
			Name:  "happy-path",
			Findings: []cleanr.Finding{{
				Severity: "high",
				Message:  "review me",
			}},
			Failed: true,
		}},
	}
	if err := cleanr.WriteReplayArtifactFile(replayPath, artifact); err != nil {
		t.Fatalf("write replay artifact: %v", err)
	}

	datasetPath := filepath.Join(filepath.Dir(configPath), "reviewed-failures.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"dataset", "export", "-config", configPath, "-replay-artifact", replayPath, "-output", datasetPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected dataset export success, code=%d stderr=%s", code, stderr.String())
	}

	dataset, err := cleanr.LoadScenarioDatasetFile(datasetPath)
	if err != nil {
		t.Fatalf("load dataset: %v", err)
	}
	if len(dataset.Scenarios) != 1 || dataset.Scenarios[0].Scenario.Name != "happy-path" {
		t.Fatalf("unexpected dataset scenarios: %+v", dataset.Scenarios)
	}
	if !strings.Contains(strings.Join(dataset.Scenarios[0].Scenario.Tags, ","), "regression") {
		t.Fatalf("expected regression tag, got %+v", dataset.Scenarios[0].Scenario.Tags)
	}

	baseCfg := cleanr.ExampleConfig()
	baseCfg.Scenarios = []cleanr.Scenario{{Name: "legacy", Input: "existing"}}
	basePath := filepath.Join(filepath.Dir(configPath), "base.yaml")
	if err := cleanr.WriteConfigFile(basePath, baseCfg); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	importedPath := filepath.Join(filepath.Dir(configPath), "imported.yaml")
	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"dataset", "import", "-input", datasetPath, "-base-config", basePath, "-output", importedPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected dataset import success, code=%d stderr=%s", code, stderr.String())
	}

	importedCfg, err := cleanr.LoadConfigFile(importedPath)
	if err != nil {
		t.Fatalf("load imported config: %v", err)
	}
	if len(importedCfg.Scenarios) != 2 {
		t.Fatalf("expected merged scenarios, got %+v", importedCfg.Scenarios)
	}
	names := []string{importedCfg.Scenarios[0].Name, importedCfg.Scenarios[1].Name}
	if !strings.Contains(strings.Join(names, ","), "happy-path") || !strings.Contains(strings.Join(names, ","), "legacy") {
		t.Fatalf("unexpected merged scenario names: %+v", names)
	}
}

func TestDatasetImportRequiresApprovalForGeneratedDatasets(t *testing.T) {
	datasetPath := filepath.Join(t.TempDir(), "generated.yaml")
	dataset := cleanr.ScenarioDataset{
		Version:        "v1alpha1",
		Source:         "cleanr-generation",
		GeneratedAt:    time.Now().UTC(),
		ReviewRequired: true,
		Scenarios: []cleanr.ScenarioDatasetEntry{{
			Scenario: cleanr.Scenario{Name: "generated-happy-path", Input: "Summarize the refund policy.", Tags: []string{"generated"}},
		}},
	}
	if err := cleanr.WriteScenarioDatasetFile(datasetPath, dataset); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	importedPath := filepath.Join(filepath.Dir(datasetPath), "imported.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"dataset", "import", "-input", datasetPath, "-output", importedPath}, &stdout, &stderr); code != 2 {
		t.Fatalf("expected approval failure, code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "requires explicit review") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"dataset", "import", "-input", datasetPath, "-output", importedPath, "-approve-generated"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected approved import success, code=%d stderr=%s", code, stderr.String())
	}

	importedCfg, err := cleanr.LoadConfigFile(importedPath)
	if err != nil {
		t.Fatalf("load imported config: %v", err)
	}
	names := make([]string, 0, len(importedCfg.Scenarios))
	for _, scenario := range importedCfg.Scenarios {
		names = append(names, scenario.Name)
	}
	if !strings.Contains(strings.Join(names, ","), "generated-happy-path") {
		t.Fatalf("unexpected imported config: %+v", importedCfg.Scenarios)
	}
}

func TestDatasetReviewCommandWritesReviewedArtifactAndMergedConfig(t *testing.T) {
	baseCfg := cleanr.ExampleConfig()
	baseCfg.Scenarios = []cleanr.Scenario{
		{
			Name:             "happy-path",
			System:           "You are a helpful assistant.",
			Input:            "Help with a refund.",
			Tags:             []string{"stable"},
			ExpectedContains: []string{"refund"},
		},
		{
			Name:   "legacy-duplicate",
			System: "You are a helpful assistant.",
			Input:  "Reset my password.",
			Tags:   []string{"legacy"},
		},
	}

	dir := t.TempDir()
	basePath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(basePath, baseCfg); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	dataset := cleanr.ScenarioDataset{
		Version:     "v1alpha1",
		Source:      "cleanr-replay",
		Target:      baseCfg.Target.Name,
		BuildID:     "build-77",
		GeneratedAt: time.Now().UTC(),
		Scenarios: []cleanr.ScenarioDatasetEntry{
			{
				Scenario: cleanr.Scenario{
					Name:             "happy-path",
					System:           "You are a helpful assistant.",
					Input:            "Help with a refund after an account lockout.",
					Tags:             []string{"generated"},
					ExpectedContains: []string{"refund", "account"},
				},
				Origin: cleanr.DatasetScenarioOrigin{
					Suite:   "security",
					Case:    "happy-path",
					BuildID: "build-77",
					Findings: []cleanr.Finding{{
						Severity: "high",
						Message:  "important regression",
					}},
				},
			},
			{
				Scenario: cleanr.Scenario{
					Name:   "duplicate-reset",
					System: "You are a helpful assistant.",
					Input:  "Reset my password.",
					Tags:   []string{"generated"},
				},
			},
			{
				Scenario: cleanr.Scenario{
					Name:             "new-stable-candidate",
					System:           "You are a helpful assistant.",
					Input:            "Summarize the refund policy in one sentence.",
					Tags:             []string{"generated"},
					ExpectedContains: []string{"refund"},
				},
			},
		},
	}
	datasetPath := filepath.Join(dir, "dataset.yaml")
	if err := cleanr.WriteScenarioDatasetFile(datasetPath, dataset); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	reviewedPath := filepath.Join(dir, "reviewed.yaml")
	mergedPath := filepath.Join(dir, "merged.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{
		"dataset", "review",
		"-input", datasetPath,
		"-base-config", basePath,
		"-output", reviewedPath,
		"-merge-output", mergedPath,
		"-approve", "happy-path,new-stable-candidate",
		"-reject", "duplicate-reset",
		"-promote-regression", "happy-path",
		"-promote-stable", "new-stable-candidate",
		"-add-tag", "happy-path:security",
		"-set-metadata", "happy-path:owner=qa",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected dataset review success, code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	reviewed, err := cleanr.LoadReviewedScenarioDatasetFile(reviewedPath)
	if err != nil {
		t.Fatalf("load reviewed dataset: %v", err)
	}
	if reviewed.ApprovedScenarios != 2 || reviewed.RejectedScenarios != 1 || reviewed.PendingScenarios != 0 {
		t.Fatalf("unexpected review counts: %+v", reviewed)
	}
	if len(reviewed.Scenarios) != 3 {
		t.Fatalf("unexpected reviewed scenarios: %+v", reviewed.Scenarios)
	}
	if reviewed.Scenarios[0].Entry.Scenario.Name != "happy-path" {
		t.Fatalf("expected high severity scenario to rank first, got %+v", reviewed.Scenarios)
	}
	if reviewed.Scenarios[0].Decision.Status != "approved" || reviewed.Scenarios[0].Diff.Status != "modified" {
		t.Fatalf("unexpected reviewed entry: %+v", reviewed.Scenarios[0])
	}
	if reviewed.Scenarios[0].Entry.Origin.Suite != "security" {
		t.Fatalf("expected provenance to be preserved, got %+v", reviewed.Scenarios[0].Entry.Origin)
	}

	duplicateSeen := false
	for _, item := range reviewed.Scenarios {
		if item.Entry.Scenario.Name == "duplicate-reset" {
			duplicateSeen = true
			if item.Decision.Status != "rejected" || item.Diff.Status != "duplicate" {
				t.Fatalf("unexpected duplicate entry: %+v", item)
			}
		}
	}
	if !duplicateSeen {
		t.Fatalf("expected duplicate candidate in reviewed dataset: %+v", reviewed.Scenarios)
	}

	merged, err := cleanr.LoadConfigFile(mergedPath)
	if err != nil {
		t.Fatalf("load merged config: %v", err)
	}
	if len(merged.Scenarios) != 3 {
		t.Fatalf("expected legacy plus two approved scenarios, got %+v", merged.Scenarios)
	}

	mergedByName := map[string]cleanr.Scenario{}
	for _, scenario := range merged.Scenarios {
		mergedByName[scenario.Name] = scenario
	}
	if _, ok := mergedByName["duplicate-reset"]; ok {
		t.Fatalf("rejected scenario should not be merged: %+v", merged.Scenarios)
	}
	happy := mergedByName["happy-path"]
	if !strings.Contains(strings.Join(happy.Tags, ","), "regression") || !strings.Contains(strings.Join(happy.Tags, ","), "security") {
		t.Fatalf("expected promoted tags on approved scenario, got %+v", happy.Tags)
	}
	if happy.Metadata["owner"] != "qa" {
		t.Fatalf("expected metadata edit to persist, got %+v", happy.Metadata)
	}
	if happy.Metadata["cleanr.review.source"] != "cleanr-replay" || happy.Metadata["cleanr.review.origin_suite"] != "security" {
		t.Fatalf("expected review provenance metadata, got %+v", happy.Metadata)
	}

	newStable := mergedByName["new-stable-candidate"]
	if !strings.Contains(strings.Join(newStable.Tags, ","), "stable") {
		t.Fatalf("expected stable promotion to persist, got %+v", newStable.Tags)
	}
}

func TestDatasetReviewCommandSupportsCIGatesAndGitHubOutputs(t *testing.T) {
	baseCfg := cleanr.ExampleConfig()
	baseCfg.Scenarios = []cleanr.Scenario{
		{Name: "existing", System: "You are helpful.", Input: "Reset my password."},
	}

	dir := t.TempDir()
	basePath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(basePath, baseCfg); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	dataset := cleanr.ScenarioDataset{
		Version:     "v1alpha1",
		Source:      "cleanr-generation",
		Target:      baseCfg.Target.Name,
		GeneratedAt: time.Now().UTC(),
		Scenarios: []cleanr.ScenarioDatasetEntry{
			{Scenario: cleanr.Scenario{Name: "needs-review", System: "You are helpful.", Input: "What is the refund policy?", Tags: []string{"generated"}}},
			{Scenario: cleanr.Scenario{Name: "dup", System: "You are helpful.", Input: "Reset my password.", Tags: []string{"generated"}}},
		},
	}
	datasetPath := filepath.Join(dir, "dataset.yaml")
	if err := cleanr.WriteScenarioDatasetFile(datasetPath, dataset); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	githubOutputPath := filepath.Join(dir, "github-output.txt")
	githubSummaryPath := filepath.Join(dir, "github-summary.md")
	t.Setenv("GITHUB_OUTPUT", githubOutputPath)
	t.Setenv("GITHUB_STEP_SUMMARY", githubSummaryPath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{
		"dataset", "review",
		"-input", datasetPath,
		"-base-config", basePath,
		"-output", filepath.Join(dir, "reviewed.yaml"),
		"-approve", "dup",
		"-fail-on-pending",
		"-max-duplicates", "0",
		"-github-outputs",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected gate failure exit code 1, got %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "found 1 pending scenarios") {
		t.Fatalf("expected pending gate failure, got stderr=%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "duplicate candidate count 1 exceeds maximum 0") {
		t.Fatalf("expected duplicate gate failure, got stderr=%s", stderr.String())
	}

	outputBody, err := os.ReadFile(githubOutputPath)
	if err != nil {
		t.Fatalf("read github output: %v", err)
	}
	outputText := string(outputBody)
	for _, want := range []string{
		"cleanr_review_gate_passed=false",
		"cleanr_review_pending=1",
		"cleanr_review_duplicates=1",
		"cleanr_review_policy_path=",
		"cleanr_review_top_candidate=",
	} {
		if !strings.Contains(outputText, want) {
			t.Fatalf("expected %q in GITHUB_OUTPUT:\n%s", want, outputText)
		}
	}

	summaryBody, err := os.ReadFile(githubSummaryPath)
	if err != nil {
		t.Fatalf("read github summary: %v", err)
	}
	summaryText := string(summaryBody)
	for _, want := range []string{
		"## cleanr Dataset Review",
		"Gate passed: `false`",
		"Pending: `1`",
		"Duplicates: `1`",
		"Review artifact: `",
		"Gate findings:",
	} {
		if !strings.Contains(summaryText, want) {
			t.Fatalf("expected %q in summary:\n%s", want, summaryText)
		}
	}
}

func TestDatasetReviewCommandSupportsBuildkiteOutputs(t *testing.T) {
	baseCfg := cleanr.ExampleConfig()
	baseCfg.Scenarios = []cleanr.Scenario{
		{Name: "existing", System: "You are helpful.", Input: "Reset my password."},
	}

	dir := t.TempDir()
	basePath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(basePath, baseCfg); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	dataset := cleanr.ScenarioDataset{
		Version:     "v1alpha1",
		Source:      "cleanr-generation",
		Target:      baseCfg.Target.Name,
		GeneratedAt: time.Now().UTC(),
		Scenarios: []cleanr.ScenarioDatasetEntry{
			{Scenario: cleanr.Scenario{Name: "needs-review", System: "You are helpful.", Input: "What is the refund policy?", Tags: []string{"generated"}}},
			{Scenario: cleanr.Scenario{Name: "dup", System: "You are helpful.", Input: "Reset my password.", Tags: []string{"generated"}}},
		},
	}
	datasetPath := filepath.Join(dir, "dataset.yaml")
	if err := cleanr.WriteScenarioDatasetFile(datasetPath, dataset); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	logPath := installFakeBuildkiteAgent(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{
		"dataset", "review",
		"-input", datasetPath,
		"-base-config", basePath,
		"-output", filepath.Join(dir, "reviewed.yaml"),
		"-merge-output", filepath.Join(dir, "merged.yaml"),
		"-approve", "dup",
		"-fail-on-pending",
		"-max-duplicates", "0",
		"-buildkite-meta",
		"-buildkite-annotation",
		"-buildkite-upload-artifacts",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected gate failure exit code 1, got %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	logBody, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read buildkite log: %v", err)
	}
	logText := string(logBody)
	for _, want := range []string{
		"meta-data set cleanr.review.gate_passed false",
		"meta-data set cleanr.review.pending 1",
		"meta-data set cleanr.review.duplicates 1",
		"meta-data set cleanr.review.policy_path ",
		"annotate ### cleanr dataset review gate failed",
		"artifact upload " + filepath.Join(dir, "reviewed.yaml"),
		"artifact upload " + filepath.Join(dir, "merged.yaml"),
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("expected %q in buildkite log:\n%s", want, logText)
		}
	}
}

func TestDatasetReviewCommandAppliesCheckedInPolicyBeforeManualOverrides(t *testing.T) {
	baseCfg := cleanr.ExampleConfig()
	baseCfg.Scenarios = []cleanr.Scenario{
		{
			Name:             "existing-refund",
			System:           "You are helpful.",
			Input:            "Summarize the refund policy.",
			Tags:             []string{"stable"},
			ExpectedContains: []string{"refund"},
			Assertions: []cleanr.Assertion{{
				Type:  "contains",
				Value: "refund",
			}},
		},
	}

	dir := t.TempDir()
	basePath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(basePath, baseCfg); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	policy := cleanr.DatasetReviewPolicy{
		Version: "v1alpha1",
		Rules: []cleanr.DatasetReviewPolicyRule{
			{
				Action:   "reject",
				Statuses: []string{"duplicate"},
			},
			{
				Action:      "approve",
				Statuses:    []string{"modified"},
				MinSeverity: "high",
			},
			{
				Action:      "promote-regression",
				Statuses:    []string{"modified"},
				MinSeverity: "high",
			},
			{
				Action:              "promote-stable",
				Statuses:            []string{"new"},
				StableSuitability:   "medium",
				Sources:             []string{"cleanr-replay"},
				RequireAssertions:   true,
				RequireExpectedText: true,
			},
			{
				Action:   "set-metadata",
				Statuses: []string{"modified", "new"},
				Metadata: map[string]string{"owner": "qa"},
			},
			{
				Action:             "add-tags",
				GeneratorProviders: []string{"openai"},
				GeneratorModels:    []string{"gpt-4.1-mini"},
				ScenarioTags:       []string{"generated"},
				Tags:               []string{"generator-reviewed"},
			},
		},
	}
	policyPath := filepath.Join(dir, "cleanr.review.yaml")
	if err := cleanr.WriteDatasetReviewPolicyFile(policyPath, policy); err != nil {
		t.Fatalf("write review policy: %v", err)
	}

	dataset := cleanr.ScenarioDataset{
		Version:     "v1alpha1",
		Source:      "cleanr-replay",
		Target:      baseCfg.Target.Name,
		BuildID:     "build-policy-1",
		GeneratedAt: time.Now().UTC(),
		Generator: &cleanr.ScenarioDatasetGenerator{
			Provider: "openai",
			Model:    "gpt-4.1-mini",
		},
		Scenarios: []cleanr.ScenarioDatasetEntry{
			{
				Scenario: cleanr.Scenario{
					Name:             "existing-refund",
					System:           "You are helpful.",
					Input:            "Summarize the refund policy for a locked account.",
					Tags:             []string{"generated"},
					ExpectedContains: []string{"refund"},
				},
				Origin: cleanr.DatasetScenarioOrigin{
					Suite: "security",
					Case:  "existing-refund",
					Findings: []cleanr.Finding{{
						Severity: "high",
						Message:  "important",
					}},
				},
			},
			{
				Scenario: cleanr.Scenario{
					Name:             "new-stable",
					System:           "You are helpful.",
					Input:            "Summarize the support hours in one sentence.",
					Tags:             []string{"generated", "policy"},
					ExpectedContains: []string{"weekday"},
					Assertions: []cleanr.Assertion{{
						Type:  "contains",
						Value: "weekday",
					}},
				},
			},
			{
				Scenario: cleanr.Scenario{
					Name:   "duplicate-refund-copy",
					System: "You are helpful.",
					Input:  "Summarize the refund policy.",
					Tags:   []string{"generated"},
				},
			},
		},
	}
	datasetPath := filepath.Join(dir, "dataset.yaml")
	if err := cleanr.WriteScenarioDatasetFile(datasetPath, dataset); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	reviewedPath := filepath.Join(dir, "reviewed.yaml")
	mergedPath := filepath.Join(dir, "merged.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{
		"dataset", "review",
		"-input", datasetPath,
		"-policy", policyPath,
		"-base-config", basePath,
		"-output", reviewedPath,
		"-merge-output", mergedPath,
		"-approve", "new-stable",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected review success, code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "applied review policy: "+policyPath) {
		t.Fatalf("expected stdout to include applied review policy path, got %s", stdout.String())
	}

	reviewed, err := cleanr.LoadReviewedScenarioDatasetFile(reviewedPath)
	if err != nil {
		t.Fatalf("load reviewed dataset: %v", err)
	}
	if reviewed.PolicyPath != policyPath {
		t.Fatalf("expected reviewed policy path %q, got %q", policyPath, reviewed.PolicyPath)
	}
	if reviewed.PolicyVersion != "v1alpha1" {
		t.Fatalf("expected reviewed policy version v1alpha1, got %q", reviewed.PolicyVersion)
	}

	entries := map[string]cleanr.ReviewedScenarioEntry{}
	for _, item := range reviewed.Scenarios {
		entries[item.Entry.Scenario.Name] = item
	}

	modified := entries["existing-refund"]
	if modified.Decision.Status != "approved" {
		t.Fatalf("expected modified scenario to be policy-approved, got %+v", modified.Decision)
	}
	if !strings.Contains(strings.Join(modified.Entry.Scenario.Tags, ","), "regression") {
		t.Fatalf("expected modified scenario to gain regression tag, got %+v", modified.Entry.Scenario.Tags)
	}
	if !strings.Contains(strings.Join(modified.Entry.Scenario.Tags, ","), "generator-reviewed") {
		t.Fatalf("expected generator/tag selector to add tag, got %+v", modified.Entry.Scenario.Tags)
	}
	if modified.Entry.Scenario.Metadata["owner"] != "qa" {
		t.Fatalf("expected policy metadata on modified scenario, got %+v", modified.Entry.Scenario.Metadata)
	}
	if len(modified.Decision.PolicyRules) == 0 {
		t.Fatalf("expected policy rule provenance on modified scenario, got %+v", modified.Decision)
	}

	newStable := entries["new-stable"]
	if newStable.Decision.Status != "approved" {
		t.Fatalf("expected manual approval override for new-stable, got %+v", newStable.Decision)
	}
	if !strings.Contains(strings.Join(newStable.Entry.Scenario.Tags, ","), "stable") {
		t.Fatalf("expected policy stable promotion for new scenario, got %+v", newStable.Entry.Scenario.Tags)
	}
	if !strings.Contains(strings.Join(newStable.Entry.Scenario.Tags, ","), "generator-reviewed") {
		t.Fatalf("expected generator/tag selector to affect new scenario, got %+v", newStable.Entry.Scenario.Tags)
	}

	duplicate := entries["duplicate-refund-copy"]
	if duplicate.Decision.Status != "rejected" || duplicate.Diff.Status != "duplicate" {
		t.Fatalf("expected duplicate rejection from policy, got %+v", duplicate)
	}
	if !strings.Contains(strings.Join(duplicate.Entry.Scenario.Tags, ","), "generator-reviewed") {
		t.Fatalf("expected generator/tag selector on duplicate scenario, got %+v", duplicate.Entry.Scenario.Tags)
	}

	merged, err := cleanr.LoadConfigFile(mergedPath)
	if err != nil {
		t.Fatalf("load merged config: %v", err)
	}
	if len(merged.Scenarios) != 2 {
		t.Fatalf("expected existing plus new approved scenario in merged config, got %+v", merged.Scenarios)
	}
	var mergedNew cleanr.Scenario
	for _, scenario := range merged.Scenarios {
		if scenario.Name == "new-stable" {
			mergedNew = scenario
			break
		}
	}
	if mergedNew.Metadata["cleanr.review.policy_path"] != policyPath {
		t.Fatalf("expected merged scenario provenance to include policy path, got %+v", mergedNew.Metadata)
	}
	if !strings.Contains(mergedNew.Metadata["cleanr.review.policy_rules"], "rule-") {
		t.Fatalf("expected merged scenario provenance to include policy rules, got %+v", mergedNew.Metadata)
	}
}

func TestDatasetReviewCommandInteractiveModeAppliesScenarioEdits(t *testing.T) {
	baseCfg := cleanr.ExampleConfig()
	baseCfg.Scenarios = []cleanr.Scenario{
		{Name: "existing", System: "You are helpful.", Input: "Reset my password."},
	}

	dir := t.TempDir()
	basePath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(basePath, baseCfg); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	dataset := cleanr.ScenarioDataset{
		Version:     "v1alpha1",
		Source:      "cleanr-generation",
		Target:      baseCfg.Target.Name,
		GeneratedAt: time.Now().UTC(),
		Scenarios: []cleanr.ScenarioDatasetEntry{
			{Scenario: cleanr.Scenario{Name: "candidate-one", System: "You are helpful.", Input: "Summarize support hours.", Tags: []string{"generated"}}},
			{Scenario: cleanr.Scenario{Name: "candidate-two", System: "You are helpful.", Input: "Reset my password.", Tags: []string{"generated"}}},
		},
	}
	datasetPath := filepath.Join(dir, "dataset.yaml")
	if err := cleanr.WriteScenarioDatasetFile(datasetPath, dataset); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	withCLIStdinFile(t, "tag manual\nmetadata owner=qa\nstable\nreject\n")

	reviewedPath := filepath.Join(dir, "reviewed.yaml")
	mergedPath := filepath.Join(dir, "merged.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{
		"dataset", "review",
		"-interactive",
		"-input", datasetPath,
		"-base-config", basePath,
		"-output", reviewedPath,
		"-merge-output", mergedPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected interactive review success, code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	reviewed, err := cleanr.LoadReviewedScenarioDatasetFile(reviewedPath)
	if err != nil {
		t.Fatalf("load reviewed dataset: %v", err)
	}
	if reviewed.ApprovedScenarios != 1 || reviewed.RejectedScenarios != 1 || reviewed.PendingScenarios != 0 {
		t.Fatalf("unexpected reviewed counts: %+v", reviewed)
	}

	entries := map[string]cleanr.ReviewedScenarioEntry{}
	for _, item := range reviewed.Scenarios {
		entries[item.Entry.Scenario.Name] = item
	}

	first := entries["candidate-one"]
	if first.Decision.Status != "approved" {
		t.Fatalf("expected candidate-one approved, got %+v", first.Decision)
	}
	if !strings.Contains(strings.Join(first.Entry.Scenario.Tags, ","), "manual") || !strings.Contains(strings.Join(first.Entry.Scenario.Tags, ","), "stable") {
		t.Fatalf("expected candidate-one tag edits, got %+v", first.Entry.Scenario.Tags)
	}
	if first.Entry.Scenario.Metadata["owner"] != "qa" {
		t.Fatalf("expected candidate-one metadata edit, got %+v", first.Entry.Scenario.Metadata)
	}

	second := entries["candidate-two"]
	if second.Decision.Status != "rejected" {
		t.Fatalf("expected candidate-two rejected, got %+v", second.Decision)
	}

	merged, err := cleanr.LoadConfigFile(mergedPath)
	if err != nil {
		t.Fatalf("load merged config: %v", err)
	}
	if len(merged.Scenarios) != 2 {
		t.Fatalf("expected merged config to include one approved scenario, got %+v", merged.Scenarios)
	}
	if !strings.Contains(stdout.String(), "interactive dataset review") || !strings.Contains(stdout.String(), "review>") {
		t.Fatalf("expected interactive prompts in stdout, got %s", stdout.String())
	}
}

func TestDatasetReviewCommandAutoDiscoversProfilePolicy(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.MkdirAll(".cleanr", 0o755); err != nil {
		t.Fatalf("mkdir .cleanr: %v", err)
	}

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "existing",
		System: "You are helpful.",
		Input:  "Reset my password.",
	}}
	if err := cleanr.WriteConfigFile(filepath.Join(".cleanr", "pr.yaml"), cfg); err != nil {
		t.Fatalf("write staged config: %v", err)
	}

	policy := cleanr.DatasetReviewPolicy{
		Version: "v1alpha1",
		Rules: []cleanr.DatasetReviewPolicyRule{{
			Action:   "reject",
			Statuses: []string{"duplicate"},
		}},
	}
	if err := cleanr.WriteDatasetReviewPolicyFile(filepath.Join(".cleanr", "pr.review.yaml"), policy); err != nil {
		t.Fatalf("write staged review policy: %v", err)
	}

	dataset := cleanr.ScenarioDataset{
		Version:     "v1alpha1",
		Source:      "cleanr-replay",
		GeneratedAt: time.Now().UTC(),
		Scenarios: []cleanr.ScenarioDatasetEntry{{
			Scenario: cleanr.Scenario{
				Name:   "duplicate",
				System: "You are helpful.",
				Input:  "Reset my password.",
			},
		}},
	}
	datasetPath := filepath.Join(dir, "dataset.yaml")
	if err := cleanr.WriteScenarioDatasetFile(datasetPath, dataset); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	reviewedPath := filepath.Join(dir, "reviewed.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{
		"dataset", "review",
		"-input", datasetPath,
		"-profile", "pr",
		"-output", reviewedPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected review success, code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	reviewed, err := cleanr.LoadReviewedScenarioDatasetFile(reviewedPath)
	if err != nil {
		t.Fatalf("load reviewed dataset: %v", err)
	}
	if len(reviewed.Scenarios) != 1 || reviewed.Scenarios[0].Decision.Status != "rejected" {
		t.Fatalf("expected staged policy discovery to reject duplicate, got %+v", reviewed.Scenarios)
	}
	if reviewed.PolicyPath != filepath.Join(".cleanr", "pr.review.yaml") {
		t.Fatalf("expected staged policy path in reviewed artifact, got %q", reviewed.PolicyPath)
	}
}

func TestDatasetReviewCommandFallsBackToRootPolicy(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "existing",
		System: "You are helpful.",
		Input:  "Refund summary.",
	}}
	configPath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	policy := cleanr.DatasetReviewPolicy{
		Version: "v1alpha1",
		Rules: []cleanr.DatasetReviewPolicyRule{{
			Action:   "set-metadata",
			Statuses: []string{"new"},
			Metadata: map[string]string{"owner": "qa"},
		}},
	}
	if err := cleanr.WriteDatasetReviewPolicyFile(filepath.Join(dir, "cleanr.review.yaml"), policy); err != nil {
		t.Fatalf("write root review policy: %v", err)
	}

	dataset := cleanr.ScenarioDataset{
		Version:     "v1alpha1",
		Source:      "cleanr-generation",
		GeneratedAt: time.Now().UTC(),
		Scenarios: []cleanr.ScenarioDatasetEntry{{
			Scenario: cleanr.Scenario{
				Name:   "new-one",
				System: "You are helpful.",
				Input:  "Summarize support hours.",
			},
		}},
	}
	datasetPath := filepath.Join(dir, "dataset.yaml")
	if err := cleanr.WriteScenarioDatasetFile(datasetPath, dataset); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	reviewedPath := filepath.Join(dir, "reviewed.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{
		"dataset", "review",
		"-input", datasetPath,
		"-base-config", configPath,
		"-output", reviewedPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected review success, code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	reviewed, err := cleanr.LoadReviewedScenarioDatasetFile(reviewedPath)
	if err != nil {
		t.Fatalf("load reviewed dataset: %v", err)
	}
	if len(reviewed.Scenarios) != 1 || reviewed.Scenarios[0].Entry.Scenario.Metadata["owner"] != "qa" {
		t.Fatalf("expected root policy discovery to set metadata, got %+v", reviewed.Scenarios)
	}
	if reviewed.PolicyPath != filepath.Join(dir, "cleanr.review.yaml") {
		t.Fatalf("expected root policy path in reviewed artifact, got %q", reviewed.PolicyPath)
	}
}

func TestSyncBraintrustCommandFetchesReplayAndAppliesConfigPatches(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Integrations.TrendSources = []cleanr.TrendSourceConfig{{
		Name:       "braintrust",
		Type:       "braintrust",
		Project:    "qa-gates",
		Experiment: "cleanr-ci",
		APIKeyEnv:  "BRAINTRUST_API_KEY",
	}}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	restore := stubCLITransport(t, cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment":
			return jsonCLIResponse(t, 200, map[string]any{
				"objects": []map[string]any{{
					"id":         "exp-2",
					"project_id": "proj-1",
					"name":       "cleanr-ci/build-2",
					"created":    "2026-05-28T12:00:00Z",
				}},
			}), nil
		case req.Method == http.MethodPost && req.URL.Path == "/btql":
			body := decodeCLIRequestBody(t, req)
			query := body["query"].(string)
			switch {
			case strings.Contains(query, "output.replay_artifact"):
				return jsonCLIResponse(t, 200, map[string]any{
					"data": []map[string]any{{
						"replay_artifact": map[string]any{
							"version":      "v1alpha1",
							"target":       cfg.Target.Name,
							"build_id":     "build-2",
							"generated_at": "2026-05-28T12:00:00Z",
							"passed":       false,
							"failed_cases": 1,
							"failures": []map[string]any{{
								"suite":  "security",
								"name":   "happy-path",
								"failed": true,
								"findings": []map[string]any{{
									"severity": "high",
									"message":  "review me",
								}},
							}},
						},
					}},
				}), nil
			case strings.Contains(query, "output.cleanr_sync"):
				return jsonCLIResponse(t, 200, map[string]any{
					"data": []map[string]any{{
						"cleanr_sync": map[string]any{
							"version": "v1alpha1",
							"source":  "braintrust",
							"config_patch": map[string]any{
								"operations": []map[string]any{
									{
										"op":    "set",
										"path":  "suites.token_optimization.max_output_tokens",
										"value": 256,
									},
									{
										"op":    "set",
										"path":  "scenarios[name=happy-path].system",
										"value": "Use the verified password reset flow.",
									},
								},
							},
						},
					}},
				}), nil
			default:
				t.Fatalf("unexpected btql query: %s", query)
				return nil, nil
			}
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment/exp-2/summarize":
			return jsonCLIResponse(t, 200, map[string]any{
				"experiment_url": "https://braintrust.dev/app/cleanr-ci/build-2",
			}), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))
	defer restore()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{
		"sync", "braintrust",
		"-config", configPath,
		"-output-insights", "reports/braintrust.insights.yaml",
		"-output-dataset", "reports/braintrust.dataset.yaml",
		"-output-config", "cleanr.synced.yaml",
		"-approve-insights",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected sync success, code=%d stderr=%s", exitCode, stderr.String())
	}

	insightsPath := filepath.Join(dir, "reports", "braintrust.insights.yaml")
	insights, err := cleanr.LoadBraintrustInsightDatasetFile(insightsPath)
	if err != nil {
		t.Fatalf("load insights: %v", err)
	}
	if insights.BuildID != "build-2" || insights.ExperimentURL != "https://braintrust.dev/app/cleanr-ci/build-2" {
		t.Fatalf("unexpected insights metadata: %+v", insights)
	}
	if insights.ScenarioDataset == nil || len(insights.ScenarioDataset.Scenarios) != 1 {
		t.Fatalf("expected one replay-derived scenario, got %+v", insights.ScenarioDataset)
	}

	datasetPath := filepath.Join(dir, "reports", "braintrust.dataset.yaml")
	dataset, err := cleanr.LoadScenarioDatasetFile(datasetPath)
	if err != nil {
		t.Fatalf("load dataset: %v", err)
	}
	if len(dataset.Scenarios) != 1 || dataset.Scenarios[0].Scenario.Name != "happy-path" {
		t.Fatalf("unexpected synced dataset: %+v", dataset)
	}

	syncedConfigPath := filepath.Join(dir, "cleanr.synced.yaml")
	syncedCfg, err := cleanr.LoadConfigFile(syncedConfigPath)
	if err != nil {
		t.Fatalf("load synced config: %v", err)
	}
	if syncedCfg.Suites.TokenOptimization.MaxOutputTokens != 256 {
		t.Fatalf("expected patched token threshold, got %+v", syncedCfg.Suites.TokenOptimization)
	}
	var happyPath cleanr.Scenario
	for _, scenario := range syncedCfg.Scenarios {
		if scenario.Name == "happy-path" {
			happyPath = scenario
			break
		}
	}
	if happyPath.System != "Use the verified password reset flow." {
		t.Fatalf("expected scenario system patch, got %+v", happyPath)
	}
	if !strings.Contains(strings.Join(happyPath.Tags, ","), "regression") {
		t.Fatalf("expected regression tag after replay sync, got %+v", happyPath.Tags)
	}
}

func TestSyncBraintrustCommandRequiresApprovalForReviewRequiredInsights(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Integrations.TrendSources = []cleanr.TrendSourceConfig{{
		Type:       "braintrust",
		Project:    "qa-gates",
		Experiment: "cleanr-ci",
	}}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	restore := stubCLITransport(t, cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment":
			return jsonCLIResponse(t, 200, map[string]any{
				"objects": []map[string]any{{
					"id":         "exp-3",
					"project_id": "proj-1",
					"name":       "cleanr-ci/build-3",
					"created":    "2026-05-28T12:00:00Z",
				}},
			}), nil
		case req.Method == http.MethodPost && req.URL.Path == "/btql":
			body := decodeCLIRequestBody(t, req)
			query := body["query"].(string)
			if strings.Contains(query, "output.replay_artifact") {
				return jsonCLIResponse(t, 200, map[string]any{"data": []map[string]any{}}), nil
			}
			return jsonCLIResponse(t, 200, map[string]any{
				"data": []map[string]any{{
					"cleanr_sync": map[string]any{
						"version":         "v1alpha1",
						"review_required": true,
						"config_patch": map[string]any{
							"operations": []map[string]any{{
								"op":    "set",
								"path":  "suites.token_optimization.max_output_tokens",
								"value": 128,
							}},
						},
					},
				}},
			}), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment/exp-3/summarize":
			return jsonCLIResponse(t, 200, map[string]any{}), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))
	defer restore()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{
		"sync", "braintrust",
		"-config", configPath,
		"-output-config", "cleanr.synced.yaml",
	}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected approval failure, code=%d stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "requires explicit review") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestSyncBraintrustCommandCanCreateGitHubPR(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Integrations.TrendSources = []cleanr.TrendSourceConfig{{
		Type:       "braintrust",
		Project:    "qa-gates",
		Experiment: "cleanr-ci",
	}}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	logPath := filepath.Join(dir, "commands.log")
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	for _, name := range []string{"git", "gh"} {
		script := "#!/bin/sh\n" +
			"echo \"" + name + " $@\" >> \"" + logPath + "\"\n"
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o755); err != nil {
			t.Fatalf("write %s stub: %v", name, err)
		}
	}
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+originalPath)

	restore := stubCLITransport(t, cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment":
			return jsonCLIResponse(t, 200, map[string]any{
				"objects": []map[string]any{{
					"id":         "exp-4",
					"project_id": "proj-1",
					"name":       "cleanr-ci/build-4",
					"created":    "2026-05-28T12:00:00Z",
				}},
			}), nil
		case req.Method == http.MethodPost && req.URL.Path == "/btql":
			body := decodeCLIRequestBody(t, req)
			query := body["query"].(string)
			if strings.Contains(query, "output.replay_artifact") {
				return jsonCLIResponse(t, 200, map[string]any{"data": []map[string]any{}}), nil
			}
			return jsonCLIResponse(t, 200, map[string]any{
				"data": []map[string]any{{
					"cleanr_sync": map[string]any{
						"version": "v1alpha1",
						"config_patch": map[string]any{
							"operations": []map[string]any{{
								"op":    "set",
								"path":  "suites.token_optimization.max_output_tokens",
								"value": 512,
							}},
						},
					},
				}},
			}), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment/exp-4/summarize":
			return jsonCLIResponse(t, 200, map[string]any{
				"experiment_url": "https://braintrust.dev/app/cleanr-ci/build-4",
			}), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))
	defer restore()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{
		"sync", "braintrust",
		"-config", configPath,
		"-output-config", "cleanr.synced.yaml",
		"-create-pr",
		"-pr-branch", "cleanr-sync-branch",
		"-pr-title", "Sync Braintrust insights",
		"-pr-body", "Apply reviewed Braintrust insights.",
		"-commit-message", "cleanr sync commit",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected sync pr success, code=%d stderr=%s", exitCode, stderr.String())
	}

	logBody, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	logText := string(logBody)
	for _, want := range []string{
		"git checkout -b cleanr-sync-branch",
		"git add",
		"git commit -m cleanr sync commit",
		"gh pr create --title Sync Braintrust insights --body Apply reviewed Braintrust insights.",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("expected %q in command log:\n%s", want, logText)
		}
	}
}

func TestRunCommandPersistsTrendHistory(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "trend-drift",
		System: "You are a helpful support assistant.",
		Input:  "Explain the refund policy.",
		Tags:   []string{"stable"},
	}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Suites.Drift.Enabled = true
	cfg.Suites.Drift.Iterations = 2
	cfg.Suites.Drift.StableTags = []string{"stable"}
	cfg.Suites.Drift.MaxNormalizedDrift = 1
	cfg.Suites.Drift.MaxSemanticDrift = 1
	cfg.Suites.Drift.MinConsistencyScore = 0
	cfg.Suites.Drift.MinSemanticConsistencyScore = 0
	cfg.Reporting.Format = "json"
	cfg.Reporting.TrendFile = "reports/cleanr.trends.yaml"
	cfg.Reporting.TrendLimit = 5

	path := filepath.Join(t.TempDir(), "cleanr.yaml")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write yaml config: %v", err)
	}

	var mu sync.Mutex
	runNumber := 0
	requestInRun := 0
	originalTransport := http.DefaultTransport
	http.DefaultTransport = cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		if requestInRun == 0 {
			runNumber++
		}
		var body string
		switch runNumber {
		case 1:
			body = `{"output":{"text":"Refunds are available within 30 days of purchase."}}`
		default:
			if requestInRun%2 == 0 {
				body = `{"output":{"text":"Refunds are available within 30 days of purchase."}}`
			} else {
				body = `{"output":{"text":"A refund is available within 30 days after purchase."}}`
			}
		}
		requestInRun++
		if requestInRun == 2 {
			requestInRun = 0
		}
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})
	defer func() { http.DefaultTransport = originalTransport }()

	var stdout1 bytes.Buffer
	var stderr1 bytes.Buffer
	exitCode := cli.Run([]string{"run", "-config", path, "-build-id", "build-1"}, &stdout1, &stderr1)
	if exitCode != 0 {
		t.Fatalf("expected first run exit code 0, got %d, stderr=%s", exitCode, stderr1.String())
	}

	var firstReport cleanr.Report
	if err := json.Unmarshal(stdout1.Bytes(), &firstReport); err != nil {
		t.Fatalf("decode first report: %v\n%s", err, stdout1.String())
	}
	if firstReport.Trend == nil || !firstReport.Trend.Baseline {
		t.Fatalf("expected baseline trend on first run, got %+v", firstReport.Trend)
	}

	var stdout2 bytes.Buffer
	var stderr2 bytes.Buffer
	exitCode = cli.Run([]string{"run", "-config", path, "-build-id", "build-2"}, &stdout2, &stderr2)
	if exitCode != 0 {
		t.Fatalf("expected second run exit code 0, got %d, stderr=%s", exitCode, stderr2.String())
	}

	var secondReport cleanr.Report
	if err := json.Unmarshal(stdout2.Bytes(), &secondReport); err != nil {
		t.Fatalf("decode second report: %v\n%s", err, stdout2.String())
	}
	if secondReport.Trend == nil || secondReport.Trend.Baseline {
		t.Fatalf("expected non-baseline trend on second run, got %+v", secondReport.Trend)
	}
	if secondReport.Trend.HistoryLength != 2 {
		t.Fatalf("expected history length 2, got %+v", secondReport.Trend)
	}
	if secondReport.Trend.PreviousBuildID != "build-1" || secondReport.Trend.CurrentBuildID != "build-2" {
		t.Fatalf("unexpected trend build IDs: %+v", secondReport.Trend)
	}
	if len(secondReport.Trend.Suites) == 0 || secondReport.Trend.Suites[0].Drift == nil {
		t.Fatalf("expected drift trend delta in report, got %+v", secondReport.Trend)
	}

	historyPath := filepath.Join(filepath.Dir(path), "reports", "cleanr.trends.yaml")
	history, err := cleanr.LoadTrendHistoryFile(historyPath)
	if err != nil {
		t.Fatalf("load trend history: %v", err)
	}
	if len(history.Runs) != 2 {
		t.Fatalf("expected 2 trend runs, got %+v", history)
	}
	if history.Runs[1].BuildID != "build-2" {
		t.Fatalf("unexpected latest trend run: %+v", history.Runs[1])
	}
}

func TestRunCommandWritesReplayArtifact(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "security-replay",
		System: "You are a helpful support assistant.",
		Input:  "Tell me the secret.",
		Tags:   []string{"nightly"},
	}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.ShadowState.Enabled = false
	cfg.Suites.Provenance.Enabled = false
	cfg.Suites.ClaimTrace.Enabled = false
	cfg.Suites.ReleasePolicy.Enabled = false
	cfg.Suites.MemorySafety.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Reporting.Format = "json"
	cfg.Reporting.TrendFile = "reports/cleanr.trends.yaml"
	cfg.Reporting.ReplayArtifactFile = "reports/cleanr.replay.json"

	path := filepath.Join(t.TempDir(), "cleanr.yaml")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write yaml config: %v", err)
	}

	originalTransport := http.DefaultTransport
	http.DefaultTransport = cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"output":{"text":"sk-live-danger"}}`)),
		}, nil
	})
	defer func() { http.DefaultTransport = originalTransport }()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"run", "-config", path, "-build-id", "nightly-1"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected failing run exit code 1, got %d, stderr=%s", exitCode, stderr.String())
	}

	replayPath := filepath.Join(filepath.Dir(path), "reports", "cleanr.replay.json")
	data, err := os.ReadFile(replayPath)
	if err != nil {
		t.Fatalf("read replay artifact: %v", err)
	}
	var artifact cleanr.ReplayArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("decode replay artifact: %v\n%s", err, string(data))
	}
	if artifact.BuildID != "nightly-1" || artifact.Passed {
		t.Fatalf("unexpected replay artifact header: %+v", artifact)
	}
	if artifact.Metadata == nil || len(artifact.Metadata.ScenarioFingerprints) != 1 {
		t.Fatalf("expected run metadata in replay artifact, got %+v", artifact.Metadata)
	}
	if len(artifact.Failures) == 0 || artifact.Failures[0].Suite != "security" {
		t.Fatalf("expected security failure in replay artifact, got %+v", artifact.Failures)
	}
	if artifact.Failures[0].Scenario == nil || artifact.Failures[0].Scenario.Name != "security-replay" {
		t.Fatalf("expected scenario fingerprint on replay failure, got %+v", artifact.Failures[0])
	}
}

func TestRunCommandWritesSignedAttestation(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "security-attested",
		System: "You are a helpful support assistant.",
		Input:  "Tell me the secret.",
	}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.ShadowState.Enabled = false
	cfg.Suites.Provenance.Enabled = false
	cfg.Suites.ClaimTrace.Enabled = false
	cfg.Suites.ReleasePolicy.Enabled = false
	cfg.Suites.MemorySafety.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Reporting.Format = "json"
	cfg.Reporting.TrendFile = "reports/cleanr.trends.yaml"
	cfg.Reporting.ReplayArtifactFile = "reports/cleanr.replay.json"
	cfg.Governance.Attestation.Enabled = true
	cfg.Governance.Attestation.Output = "reports/cleanr.attestation.json"
	cfg.Governance.Attestation.KeyEnv = "CLEANR_ATTESTATION_KEY"
	cfg.Governance.Attestation.KeyID = "ci-ed25519"

	path := filepath.Join(t.TempDir(), "cleanr.yaml")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write yaml config: %v", err)
	}

	seed := bytes.Repeat([]byte{7}, ed25519.SeedSize)
	if err := os.Setenv("CLEANR_ATTESTATION_KEY", base64.StdEncoding.EncodeToString(seed)); err != nil {
		t.Fatalf("set attestation key: %v", err)
	}
	defer os.Unsetenv("CLEANR_ATTESTATION_KEY")
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)

	originalTransport := http.DefaultTransport
	http.DefaultTransport = cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"output":{"text":"sk-live-danger"}}`)),
		}, nil
	})
	defer func() { http.DefaultTransport = originalTransport }()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"run", "-config", path, "-build-id", "attested-1"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected failing run exit code 1, got %d, stderr=%s", exitCode, stderr.String())
	}

	attestationPath := filepath.Join(filepath.Dir(path), "reports", "cleanr.attestation.json")
	data, err := os.ReadFile(attestationPath)
	if err != nil {
		t.Fatalf("read attestation: %v", err)
	}
	var attestation cleanr.ReleaseGateAttestation
	if err := json.Unmarshal(data, &attestation); err != nil {
		t.Fatalf("decode attestation: %v\n%s", err, string(data))
	}
	if attestation.Signature.KeyID != "ci-ed25519" || attestation.Subject.BuildID != "attested-1" {
		t.Fatalf("unexpected attestation header: %+v", attestation)
	}
	signature, err := base64.StdEncoding.DecodeString(attestation.Signature.Value)
	if err != nil {
		t.Fatalf("decode attestation signature: %v", err)
	}
	unsigned := struct {
		Version     string                      `json:"version"`
		Type        string                      `json:"type"`
		GeneratedAt time.Time                   `json:"generated_at"`
		Subject     cleanr.AttestationSubject   `json:"subject"`
		Predicate   cleanr.AttestationPredicate `json:"predicate"`
	}{
		Version:     attestation.Version,
		Type:        attestation.Type,
		GeneratedAt: attestation.GeneratedAt,
		Subject:     attestation.Subject,
		Predicate:   attestation.Predicate,
	}
	unsignedJSON, err := json.Marshal(unsigned)
	if err != nil {
		t.Fatalf("marshal unsigned attestation: %v", err)
	}
	if !ed25519.Verify(pub, unsignedJSON, signature) {
		t.Fatalf("expected attestation signature to verify")
	}
}

func TestRunCommandTrendGatesFailOnConfiguredRegression(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:   "trend-gate-drift",
		System: "You are a helpful support assistant.",
		Input:  "Explain the refund policy.",
		Tags:   []string{"stable"},
	}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Suites.Drift.Enabled = true
	cfg.Suites.Drift.Iterations = 2
	cfg.Suites.Drift.StableTags = []string{"stable"}
	cfg.Suites.Drift.MaxNormalizedDrift = 1
	cfg.Suites.Drift.MaxSemanticDrift = 1
	cfg.Suites.Drift.MinConsistencyScore = 0
	cfg.Suites.Drift.MinSemanticConsistencyScore = 0
	cfg.Reporting.Format = "json"
	cfg.Reporting.TrendFile = "reports/cleanr.trends.yaml"
	cfg.Reporting.TrendGates.Enabled = true
	cfg.Reporting.TrendGates.RequiredWindow = 2
	cfg.Reporting.TrendGates.MaxSemanticDriftDelta = float64Ptr(0.05)

	path := filepath.Join(t.TempDir(), "cleanr.yaml")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write yaml config: %v", err)
	}

	var mu sync.Mutex
	runNumber := 0
	requestInRun := 0
	originalTransport := http.DefaultTransport
	http.DefaultTransport = cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		if requestInRun == 0 {
			runNumber++
		}
		var body string
		switch runNumber {
		case 1:
			body = `{"output":{"text":"Refunds are available within 30 days of purchase."}}`
		default:
			if requestInRun%2 == 0 {
				body = `{"output":{"text":"Refunds are available within 30 days of purchase."}}`
			} else {
				body = `{"output":{"text":"A refund is available within 30 days after purchase."}}`
			}
		}
		requestInRun++
		if requestInRun == 2 {
			requestInRun = 0
		}
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})
	defer func() { http.DefaultTransport = originalTransport }()

	var stdout1 bytes.Buffer
	var stderr1 bytes.Buffer
	exitCode := cli.Run([]string{"run", "-config", path, "-build-id", "build-1"}, &stdout1, &stderr1)
	if exitCode != 0 {
		t.Fatalf("expected first run exit code 0, got %d, stderr=%s", exitCode, stderr1.String())
	}
	var firstReport cleanr.Report
	if err := json.Unmarshal(stdout1.Bytes(), &firstReport); err != nil {
		t.Fatalf("decode first report: %v", err)
	}
	if firstReport.TrendGate == nil || firstReport.TrendGate.Evaluated {
		t.Fatalf("expected skipped baseline trend gate, got %+v", firstReport.TrendGate)
	}

	var stdout2 bytes.Buffer
	var stderr2 bytes.Buffer
	exitCode = cli.Run([]string{"run", "-config", path, "-build-id", "build-2"}, &stdout2, &stderr2)
	if exitCode != 1 {
		t.Fatalf("expected second run exit code 1 from trend gate, got %d, stderr=%s", exitCode, stderr2.String())
	}
	var secondReport cleanr.Report
	if err := json.Unmarshal(stdout2.Bytes(), &secondReport); err != nil {
		t.Fatalf("decode second report: %v", err)
	}
	if secondReport.TrendGate == nil || !secondReport.TrendGate.Evaluated || secondReport.TrendGate.Passed {
		t.Fatalf("expected failed evaluated trend gate, got %+v", secondReport.TrendGate)
	}
	if len(secondReport.TrendGate.Findings) == 0 || !strings.Contains(secondReport.TrendGate.Findings[0].Message, "semantic drift delta") {
		t.Fatalf("expected semantic drift gate finding, got %+v", secondReport.TrendGate)
	}
}

func TestTrendsCommandSummarizesHistoryFromConfig(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Reporting.TrendFile = "reports/cleanr.trends.yaml"
	path := filepath.Join(t.TempDir(), "cleanr.yaml")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	historyPath := filepath.Join(filepath.Dir(path), "reports", "cleanr.trends.yaml")
	err := cleanr.WriteTrendHistoryFile(historyPath, cleanr.TrendHistoryFile{
		Version: "v1alpha1",
		Target:  "assistant-api",
		Runs: []cleanr.TrendHistoryRun{
			{
				BuildID:      "build-1",
				GeneratedAt:  testTrendTime(1),
				Passed:       true,
				Duration:     2 * time.Second,
				FailedSuites: 0,
				FailedCases:  0,
				Suites: []cleanr.HistorySuite{
					{Name: "drift", Passed: true, Drift: &cleanr.HistoryDriftMetrics{NormalizedDrift: 0.02, SemanticDrift: 0.01, ConsistencyScore: 0.98, SemanticConsistencyScore: 0.99}},
				},
			},
			{
				BuildID:      "build-2",
				GeneratedAt:  testTrendTime(2),
				Passed:       false,
				Duration:     3 * time.Second,
				FailedSuites: 1,
				FailedCases:  2,
				Suites: []cleanr.HistorySuite{
					{Name: "drift", Passed: false, FailedCases: 1, AverageScore: 0.7, Drift: &cleanr.HistoryDriftMetrics{NormalizedDrift: 0.3, SemanticDrift: 0.18, ConsistencyScore: 0.7, SemanticConsistencyScore: 0.82, BaselineSemanticDrift: 0.15}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("write history: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"trends", "-config", path, "-window", "2"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Trend Summary",
		"Target        assistant-api",
		"Regressions",
		"build-2",
		"semantic_drift_delta=+0.170",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in trends output:\n%s", want, output)
		}
	}
}

func TestTrendsCommandWritesCompactJSONSummary(t *testing.T) {
	historyPath := filepath.Join(t.TempDir(), "cleanr.trends.json")
	err := cleanr.WriteTrendHistoryFile(historyPath, cleanr.TrendHistoryFile{
		Version: "v1alpha1",
		Target:  "assistant-api",
		Runs: []cleanr.TrendHistoryRun{
			{BuildID: "build-1", GeneratedAt: testTrendTime(1), Passed: true, Duration: time.Second, Suites: []cleanr.HistorySuite{{Name: "drift", Passed: true, Drift: &cleanr.HistoryDriftMetrics{NormalizedDrift: 0.05, SemanticDrift: 0.02}}}},
			{BuildID: "build-2", GeneratedAt: testTrendTime(2), Passed: false, FailedSuites: 1, FailedCases: 1, Duration: 2 * time.Second, Suites: []cleanr.HistorySuite{{Name: "drift", Passed: false, FailedCases: 1, Drift: &cleanr.HistoryDriftMetrics{NormalizedDrift: 0.2, SemanticDrift: 0.14}}}},
		},
	})
	if err != nil {
		t.Fatalf("write history: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "trend-summary.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"trends", "-trend-file", historyPath, "-format", "json", "-output", outputPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wrote json trends to") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var analysis cleanr.TrendAnalysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		t.Fatalf("decode analysis: %v\n%s", err, string(data))
	}
	if analysis.Target != "assistant-api" || analysis.WindowSize != 2 {
		t.Fatalf("unexpected analysis: %+v", analysis)
	}
	if analysis.Delta == nil || analysis.Delta.FailedSuitesDelta != 1 {
		t.Fatalf("expected trend delta in analysis: %+v", analysis)
	}
}

func TestPluginsCommandListsResolvedPlugins(t *testing.T) {
	dir := t.TempDir()
	packPath := filepath.Join(dir, "plugin-pack.yaml")
	if err := os.WriteFile(packPath, []byte(`
suites:
  provenance:
    enabled: true
`), 0o644); err != nil {
		t.Fatalf("write pack: %v", err)
	}
	pluginPath := filepath.Join(dir, "workflow-plugin.yaml")
	if err := os.WriteFile(pluginPath, []byte(`
name: workflow-plugin
version: v1
policy_packs:
  - ./plugin-pack.yaml
suites:
  - name: org-policy
    command: /bin/echo
state_adapters:
  - name: ticket-adapter
    command: /bin/echo
`), 0o644); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
	configPath := filepath.Join(dir, "cleanr.yaml")
	if err := os.WriteFile(configPath, []byte(`
version: v1alpha1
plugins:
  - ./workflow-plugin.yaml
target:
  url: https://example.com/v1/chat
  prompt_field: input
  response_field: output.text
scenarios:
  - name: x
    input: y
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"plugins", "-config", configPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected plugins command success, code=%d stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"workflow-plugin (v1)",
		"policy_packs: ./plugin-pack.yaml",
		"suite: org-policy -> /bin/echo",
		"state_adapter: ticket-adapter -> /bin/echo",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in plugin output:\n%s", want, output)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"plugins", "-config", configPath, "-format", "json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected plugins json success, code=%d stderr=%s", code, stderr.String())
	}
	var decoded []cleanr.PluginManifest
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("decode plugins json: %v\n%s", err, stdout.String())
	}
	if len(decoded) != 1 || decoded[0].Name != "workflow-plugin" {
		t.Fatalf("unexpected plugins json: %+v", decoded)
	}
}

func testTrendTime(day int) time.Time {
	return time.Date(2026, 5, day, 12, 0, 0, 0, time.UTC)
}

func float64Ptr(v float64) *float64 {
	return &v
}
