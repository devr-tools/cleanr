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
