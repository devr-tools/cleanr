package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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
		"reporting.format: must be one of text, json, or junit",
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
		"reporting.format: must be one of text, json, or junit",
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
