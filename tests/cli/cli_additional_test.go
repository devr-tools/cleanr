package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cleanr/cleanr"
	"cleanr/internal/cli"
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

	stdout.Reset()
	stderr.Reset()
	code = cli.Run([]string{"run", "-config", path, "-format", "bogus"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "write report: unsupported report format: bogus") {
		t.Fatalf("unexpected unsupported-format result: code=%d stderr=%q", code, stderr.String())
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
