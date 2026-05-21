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
	"strings"
	"sync"
	"testing"
	"time"

	"cleanr/cleanr"
	"cleanr/internal/cli"
	"cleanr/internal/testutil"
)

type cliRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f cliRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
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
