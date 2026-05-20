package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cleanr/cleanr"
	profilepkg "cleanr/cleanr/profile"
	"cleanr/internal/cli"
)

func TestSetupCommandStoresProviderAndWritesConfig(t *testing.T) {
	t.Setenv("CLEANR_HOME", filepath.Join(t.TempDir(), "state"))
	configPath := filepath.Join(t.TempDir(), "cleanr.yaml")

	restoreStdin := swapStdin(t, "openai\nresponses\ngpt-4.1-mini\nOPENAI_API_KEY\nsk-test\n")
	defer restoreStdin()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"setup", "-output", configPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	profile, err := profilepkg.Load()
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	openaiProfile, ok := profile.Providers["openai"]
	if !ok {
		t.Fatalf("expected stored openai profile, got %+v", profile.Providers)
	}
	if openaiProfile.APIKey != "sk-test" || openaiProfile.Model != "gpt-4.1-mini" {
		t.Fatalf("unexpected stored provider: %+v", openaiProfile)
	}

	cfg, err := cleanr.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	if cfg.Target.Type != "openai" || cfg.Target.OpenAI.Model != "gpt-4.1-mini" {
		t.Fatalf("unexpected generated config target: %+v", cfg.Target)
	}
	if !strings.Contains(stdout.String(), "stored openai credentials") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestSetupAgentCommandUsesStoredProviderAndInjectsPrompt(t *testing.T) {
	t.Setenv("CLEANR_HOME", filepath.Join(t.TempDir(), "state"))
	if err := profilepkg.UpsertProvider(profilepkg.Provider{
		Name:      "openai",
		Model:     "gpt-4.1-mini",
		APIMode:   "responses",
		APIKeyEnv: "OPENAI_API_KEY",
		APIKey:    "sk-test",
	}); err != nil {
		t.Fatalf("seed provider profile: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "cleanr.agent.yaml")
	restoreStdin := swapStdin(t, "support-agent\nYou are a support agent that must stay within policy.\nReset the customer password and confirm the reset email.\n")
	defer restoreStdin()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"setup", "agent", "-output", configPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	cfg, err := cleanr.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load agent config: %v", err)
	}
	if cfg.Target.Type != "openai" || cfg.Target.Name != "support-agent" {
		t.Fatalf("unexpected agent target: %+v", cfg.Target)
	}
	if len(cfg.Scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(cfg.Scenarios))
	}
	if cfg.Scenarios[0].System != "You are a support agent that must stay within policy." {
		t.Fatalf("unexpected system prompt: %+v", cfg.Scenarios[0])
	}
	if cfg.Scenarios[0].Input != "Reset the customer password and confirm the reset email." {
		t.Fatalf("unexpected primary user prompt: %+v", cfg.Scenarios[0])
	}
	if cfg.Suites.Drift.BaselineFile != filepath.Join("snapshots", "support-agent.snapshots.yaml") {
		t.Fatalf("unexpected baseline file: %s", cfg.Suites.Drift.BaselineFile)
	}
	if cfg.Reporting.TrendFile != filepath.Join("reports", "support-agent.trends.yaml") {
		t.Fatalf("unexpected trend file: %s", cfg.Reporting.TrendFile)
	}
}

func swapStdin(t *testing.T, input string) func() {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	if _, err := writer.WriteString(input); err != nil {
		t.Fatalf("write stdin pipe: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}

	oldStdin := os.Stdin
	os.Stdin = reader
	return func() {
		os.Stdin = oldStdin
		_ = reader.Close()
	}
}
