package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cleanr/cleanr"
	"cleanr/internal/cli"
)

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
	path := writeNamedConfigFile(t, "cleanr.yaml", `
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
	if !strings.Contains(stdout.String(), "cleanr PASS") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}
