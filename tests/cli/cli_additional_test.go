package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/cli"
)

func TestCLIRunUsageVersionAndMissingConfigPaths(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run(nil, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "usage: cleanr") {
		t.Fatalf("unexpected no-arg result: code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"unknown"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "usage: cleanr") {
		t.Fatalf("unexpected unknown-command result: code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"version"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "cleanr ") {
		t.Fatalf("unexpected version output: code=%d stdout=%q", code, stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"watch", "-max-runs", "1"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "no config file found; expected one of cleanr.json, cleanr.yaml, cleanr.yml") {
		t.Fatalf("unexpected missing-watch-config result: code=%d stderr=%q", code, stderr.String())
	}

	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"run"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "no config file found; expected one of cleanr.json, cleanr.yaml, cleanr.yml") {
		t.Fatalf("unexpected missing-config result: code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"trends"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "no config file found; expected one of cleanr.json, cleanr.yaml, cleanr.yml") {
		t.Fatalf("unexpected missing-trends-config result: code=%d stderr=%q", code, stderr.String())
	}
}

func TestCLIWatchRerunsOnFileChanges(t *testing.T) {
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
	cfg.Scenarios = []cleanr.Scenario{{
		Name:             "missing-phrase",
		Input:            "hello",
		ExpectedContains: []string{"missing"},
	}}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	triggerPath := filepath.Join(dir, "trigger.txt")
	if err := os.WriteFile(triggerPath, []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("write trigger: %v", err)
	}

	var requests atomic.Int32
	restoreTransport := stubCLITransport(t, cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		body := `{"output":{"text":"hello there"}}`
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}))
	defer restoreTransport()

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(triggerPath, []byte("changed\n"), 0o644)
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{
		"watch",
		"-config", configPath,
		"-watch", dir,
		"-interval", "10ms",
		"-max-runs", "2",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected watch to return the last run exit code 1, got %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("expected two watched runs, got %d stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "watch run #1") || !strings.Contains(stdout.String(), "watch run #2") {
		t.Fatalf("expected watch run markers in stdout:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "detected changes in") {
		t.Fatalf("expected file change detection in stdout:\n%s", stdout.String())
	}
}

func TestCLIRunSupportsOutputFileAndFailureExitCodes(t *testing.T) {
	t.Parallel()

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
	cfg.Scenarios = []cleanr.Scenario{{
		Name:              "missing-phrase",
		Input:             "hello",
		ExpectedContains:  []string{"missing"},
		ForbiddenContains: []string{},
	}}

	path := filepath.Join(t.TempDir(), "cleanr.json")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	output := filepath.Join(t.TempDir(), "report.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"run", "-config", path, "-format", "json", "-output", output}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected failing exit code 1, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wrote json report to") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("expected report output file: %v", err)
	}

	htmlOutput := filepath.Join(t.TempDir(), "report.html")
	stdout.Reset()
	stderr.Reset()
	code = cli.Run([]string{"run", "-config", path, "-format", "html", "-output", htmlOutput}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected failing exit code 1 for html report, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wrote html report to") {
		t.Fatalf("unexpected html stdout: %s", stdout.String())
	}
	htmlData, err := os.ReadFile(htmlOutput)
	if err != nil {
		t.Fatalf("read html report: %v", err)
	}
	if !strings.Contains(string(htmlData), "<!DOCTYPE html>") || !strings.Contains(string(htmlData), "Static cleanr report dashboard") || !strings.Contains(string(htmlData), "devr-tools / cleanr") || !strings.Contains(string(htmlData), `aria-label="cleanr ascii logo"`) || !strings.Contains(string(htmlData), `▄▄`) {
		t.Fatalf("unexpected html report:\n%s", string(htmlData))
	}

	stdout.Reset()
	stderr.Reset()
	code = cli.Run([]string{"run", "-config", path, "-format", "bogus"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "write report: unsupported report format: bogus") {
		t.Fatalf("unexpected unsupported-format result: code=%d stderr=%q", code, stderr.String())
	}
}

func TestCLIRunWritesHTMLReport(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false

	path := filepath.Join(t.TempDir(), "cleanr.json")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	output := filepath.Join(t.TempDir(), "report.html")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{"run", "-config", path, "-format", "html", "-output", output}, &stdout, &stderr)
	if code != 0 && code != 1 {
		t.Fatalf("expected report generation exit code 0 or 1, got %d stderr=%s", code, stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if text := string(data); !strings.Contains(text, "<!DOCTYPE html>") || !strings.Contains(text, "Static cleanr report dashboard") || !strings.Contains(text, "devr-tools / cleanr") || !strings.Contains(text, `aria-label="cleanr ascii logo"`) || !strings.Contains(text, `▄▄`) {
		t.Fatalf("unexpected html report:\n%s", text)
	}
}

func TestCLIMCPCommandReturnsOnEOF(t *testing.T) {
	t.Parallel()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_ = writer.Close()
	oldStdin := os.Stdin
	os.Stdin = reader
	defer func() {
		os.Stdin = oldStdin
		_ = reader.Close()
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"mcp"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected mcp command to exit cleanly on EOF, got %d stderr=%s", code, stderr.String())
	}
}

func TestCLIValidateSupportsStagedProfileConfigs(t *testing.T) {
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
	cfg.Target.Name = "pr-profile"
	if err := cleanr.WriteConfigFile(filepath.Join(".cleanr", "pr.yaml"), cfg); err != nil {
		t.Fatalf("write staged config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"validate", "-profile", "pr"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected validate to succeed, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "valid config for pr-profile with 2 scenarios") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestCLIValidateSupportsStagedProfileFromEnv(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Setenv("CLEANR_PROFILE", "release")

	if err := os.MkdirAll(".cleanr", 0o755); err != nil {
		t.Fatalf("mkdir .cleanr: %v", err)
	}

	cfg := cleanr.ExampleConfig()
	cfg.Target.Name = "release-profile"
	if err := cleanr.WriteConfigFile(filepath.Join(".cleanr", "release.yaml"), cfg); err != nil {
		t.Fatalf("write staged config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"validate"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected validate to succeed, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "valid config for release-profile with 2 scenarios") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestCLIValidateHintsWhenStagedConfigsNeedProfileSelection(t *testing.T) {
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
	if err := cleanr.WriteConfigFile(filepath.Join(".cleanr", "pr.yaml"), cfg); err != nil {
		t.Fatalf("write pr config: %v", err)
	}
	if err := cleanr.WriteConfigFile(filepath.Join(".cleanr", "main.yaml"), cfg); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"validate"}, &stdout, &stderr); code != 2 {
		t.Fatalf("expected validate to fail, got %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "found staged configs under .cleanr, rerun with -profile pr|main|release or set CLEANR_PROFILE") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestWriteReplayArtifactFileRedactsSensitiveEvidence(t *testing.T) {
	replayPath := filepath.Join(t.TempDir(), "cleanr.replay.json")
	artifact := cleanr.ReplayArtifact{
		Version:     "v1alpha1",
		Target:      "assistant",
		GeneratedAt: time.Now().UTC(),
		Passed:      false,
		Failures: []cleanr.ReplayArtifactCase{{
			Suite: "security",
			Name:  "leak",
			Findings: []cleanr.Finding{{
				Severity: "high",
				Message:  "Authorization: Bearer sk-very-secret-token",
			}},
			Evidence: map[string]any{
				"authorization": "Bearer sk-very-secret-token",
				"nested": map[string]any{
					"api_key": "sk-test-secret",
				},
				"provider_note": "aws key AKIA1234567890ABCDEF should be scrubbed",
			},
			Failed: true,
		}},
	}

	if err := cleanr.WriteReplayArtifactFile(replayPath, artifact); err != nil {
		t.Fatalf("write replay artifact: %v", err)
	}
	data, err := os.ReadFile(replayPath)
	if err != nil {
		t.Fatalf("read replay artifact: %v", err)
	}
	body := string(data)
	for _, forbidden := range []string{
		"sk-very-secret-token",
		"sk-test-secret",
		"AKIA1234567890ABCDEF",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("expected replay artifact redaction for %q:\n%s", forbidden, body)
		}
	}
	for _, want := range []string{
		`"authorization": "[REDACTED]"`,
		`"api_key": "[REDACTED]"`,
		`"provider_note": "aws key [REDACTED] should be scrubbed"`,
		`"message": "Authorization: Bearer [REDACTED]"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in replay artifact:\n%s", want, body)
		}
	}
}

func TestCLIValidateRejectsInvalidProfile(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"validate", "-profile", "nightly"}, &stdout, &stderr); code != 2 {
		t.Fatalf("expected validate to fail, got %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `invalid: unsupported profile "nightly"; expected pr, main, or release`) {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestCLIRunSupportsAgentReportFormat(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false

	path := filepath.Join(t.TempDir(), "cleanr.yaml")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"run", "-config", path, "-format", "agent"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected run to succeed, got %d stderr=%s", code, stderr.String())
	}

	var report cleanr.AgentReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode agent report: %v\n%s", err, stdout.String())
	}
	if report.Contract.Format != "agent" || report.Summary.Target != cfg.Target.Name {
		t.Fatalf("unexpected agent report: %+v", report)
	}
}

func TestCLIGenerateAuthorsScenarioFromNaturalLanguagePrompt(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "authored.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"generate", "test that refunds require manager approval before issuance", "-output", outputPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected authoring flow to succeed, got %d stderr=%s", code, stderr.String())
	}

	cfg, err := cleanr.LoadConfigFile(outputPath)
	if err != nil {
		t.Fatalf("load authored config: %v", err)
	}
	if len(cfg.Scenarios) != 1 {
		t.Fatalf("expected one authored scenario, got %+v", cfg.Scenarios)
	}
	scenario := cfg.Scenarios[0]
	if scenario.Name != "refunds-require-manager-approval-before-issuance" {
		t.Fatalf("unexpected scenario name: %+v", scenario)
	}
	if scenario.Input != "refunds require manager approval before issuance" {
		t.Fatalf("unexpected scenario input: %+v", scenario)
	}
	if scenario.Metadata["authoring.mode"] != "natural_language" {
		t.Fatalf("unexpected scenario metadata: %+v", scenario.Metadata)
	}
	if !strings.Contains(stdout.String(), "wrote authored scenario") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestCLIExplainReadsReplayArtifact(t *testing.T) {
	t.Parallel()

	replayPath := filepath.Join(t.TempDir(), "cleanr.replay.json")
	artifact := cleanr.ReplayArtifact{
		Version:     "v1alpha1",
		Target:      "assistant-api",
		GeneratedAt: time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
		Failures: []cleanr.ReplayArtifactCase{{
			Suite: "claim_trace",
			Name:  "refunds-policy",
			Findings: []cleanr.Finding{{
				Severity: "high",
				Message:  "claimed tool execution with no matching invocation: lookup_policy",
			}},
			Evidence: map[string]any{
				"claimed_tools": []string{"lookup_policy"},
			},
			Failed: true,
		}},
	}
	if err := cleanr.WriteReplayArtifactFile(replayPath, artifact); err != nil {
		t.Fatalf("write replay artifact: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"explain", "claim_trace/refunds-policy", "-replay-artifact", replayPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected explain to succeed, got %d stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Failure ID  claim_trace/refunds-policy",
		"Summary     claimed tool execution with no matching invocation: lookup_policy",
		"Fix Suggestions",
		"Align claimed tool or citation behavior with trace evidence",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}
