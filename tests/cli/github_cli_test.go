package tests

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/cli"
)

func TestRunCommandWritesGitHubPRReviewOutputs(t *testing.T) {
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

	githubOutputPath := filepath.Join(dir, "github-output.txt")
	githubSummaryPath := filepath.Join(dir, "github-summary.md")
	if err := os.WriteFile(githubOutputPath, []byte("existing-output=true\n"), 0o644); err != nil {
		t.Fatalf("seed github output: %v", err)
	}
	if err := os.WriteFile(githubSummaryPath, []byte("Existing summary\n"), 0o644); err != nil {
		t.Fatalf("seed github summary: %v", err)
	}
	t.Setenv("GITHUB_OUTPUT", githubOutputPath)
	t.Setenv("GITHUB_STEP_SUMMARY", githubSummaryPath)

	restoreTransport := stubCLITransport(t, cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"output":{"text":"hello there"}}`
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}))
	defer restoreTransport()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"run", "-config", configPath, "-github-outputs"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected failing run exit code 1, got %d stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}

	outputBody, err := os.ReadFile(githubOutputPath)
	if err != nil {
		t.Fatalf("read github output: %v", err)
	}
	outputText := string(outputBody)
	for _, want := range []string{
		"existing-output=true",
		"cleanr_run_gate_passed=false",
		"cleanr_run_failed_suites=1",
		"cleanr_run_failed_cases=1",
		"cleanr_run_new_failures=0",
		"cleanr_run_worsened_drift=0",
		"cleanr_run_gate_summary=local gate fail, 1 failed suites, 1 failed cases",
		"cleanr_run_pr_comment=## cleanr PR Review",
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
		"Existing summary",
		"## cleanr PR Review",
		"- Gate: `FAIL`",
		"### Gate Explanation",
		"### New Failures",
		"- none",
		"### Recommended Scenarios To Review",
		"- none",
	} {
		if !strings.Contains(summaryText, want) {
			t.Fatalf("expected %q in GITHUB_STEP_SUMMARY:\n%s", want, summaryText)
		}
	}
}

func TestRunCommandPostsGitHubPRComment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh helper uses POSIX shell")
	}

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

	logPath := installFakeGHCLI(t)
	eventPath := filepath.Join(dir, "event.json")
	if err := os.WriteFile(eventPath, []byte(`{"number":17}`), 0o644); err != nil {
		t.Fatalf("write event: %v", err)
	}
	t.Setenv("GITHUB_EVENT_PATH", eventPath)

	restoreTransport := stubCLITransport(t, cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"output":{"text":"hello there"}}`
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}))
	defer restoreTransport()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"run", "-config", configPath, "-github-pr-comment"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected failing run exit code 1, got %d stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "posted GitHub PR comment to #17") {
		t.Fatalf("expected post confirmation in stdout: %s", stdout.String())
	}

	logBody, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read gh log: %v", err)
	}
	logText := string(logBody)
	for _, want := range []string{
		"gh pr comment 17 --body-file",
		"## cleanr PR Review",
		"### Gate Explanation",
		"### Recommended Scenarios To Review",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("expected %q in gh log:\n%s", want, logText)
		}
	}
}

func TestGitHubDoctorCommandReportsAuthenticatedSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh helper uses POSIX shell")
	}

	logPath := installFakeGHCLI(t)
	t.Setenv("FAKE_GH_AUTH_STATUS_OUTPUT", "github.com\n  ✓ Logged in to github.com account alex")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"github", "doctor"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected doctor success, got %d stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"GitHub integration doctor",
		"- GitHub CLI: available",
		"- Auth: ok",
		"Logged in to github.com account alex",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected %q in stdout:\n%s", want, stdout.String())
		}
	}

	logBody, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read gh log: %v", err)
	}
	if !strings.Contains(string(logBody), "gh auth status") {
		t.Fatalf("expected gh auth status in log:\n%s", string(logBody))
	}
}

func TestGitHubAuthCommandInvokesGHLogin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh helper uses POSIX shell")
	}

	logPath := installFakeGHCLI(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"github", "auth"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected auth success, got %d stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"launching GitHub CLI authentication",
		"GitHub CLI authentication completed",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected %q in stdout:\n%s", want, stdout.String())
		}
	}

	logBody, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read gh log: %v", err)
	}
	if !strings.Contains(string(logBody), "gh auth login") {
		t.Fatalf("expected gh auth login in log:\n%s", string(logBody))
	}
}

func installFakeGHCLI(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "gh.log")
	scriptPath := filepath.Join(dir, "gh")
	script := `#!/bin/sh
set -eu
printf 'gh' >> "$FAKE_GH_LOG"
for arg in "$@"; do
  printf ' %s' "$arg" >> "$FAKE_GH_LOG"
done
printf '\n' >> "$FAKE_GH_LOG"

if [ "$#" -ge 2 ] && [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  if [ "${FAKE_GH_AUTH_STATUS_OUTPUT:-}" != "" ]; then
    printf '%s\n' "$FAKE_GH_AUTH_STATUS_OUTPUT"
  fi
  exit "${FAKE_GH_AUTH_STATUS_EXIT:-0}"
fi

if [ "$#" -ge 2 ] && [ "$1" = "auth" ] && [ "$2" = "login" ]; then
  exit "${FAKE_GH_AUTH_LOGIN_EXIT:-0}"
fi

while [ "$#" -gt 0 ]; do
  if [ "$1" = "--body-file" ]; then
    shift
    printf '%s\n' '---body---' >> "$FAKE_GH_LOG"
    cat "$1" >> "$FAKE_GH_LOG"
    printf '\n%s\n' '---end-body---' >> "$FAKE_GH_LOG"
    exit 0
  fi
  shift
done
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("FAKE_GH_LOG", logPath)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}
