package tests

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
	profilepkg "github.com/devr-tools/cleanr/cleanr/profile"
	"github.com/devr-tools/cleanr/internal/cli"
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
	if cfg.Reporting.TrendGates.Preset != "moderate" {
		t.Fatalf("expected default trend gate preset moderate, got %+v", cfg.Reporting.TrendGates)
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
	if !cfg.Reporting.TrendGates.Enabled {
		t.Fatalf("expected trend gates to be enabled in generated agent config")
	}
	if cfg.Reporting.TrendGates.RequiredWindow != 2 {
		t.Fatalf("unexpected trend gate required window: %d", cfg.Reporting.TrendGates.RequiredWindow)
	}
	if cfg.Reporting.TrendGates.MaxSemanticDriftDelta == nil || *cfg.Reporting.TrendGates.MaxSemanticDriftDelta != 0.08 {
		t.Fatalf("unexpected semantic drift gate: %+v", cfg.Reporting.TrendGates)
	}
	if cfg.Reporting.TrendGates.Preset != "moderate" {
		t.Fatalf("expected moderate preset in generated agent config, got %+v", cfg.Reporting.TrendGates)
	}
}

func TestSetupCommandSupportsExploratoryTrendGatePreset(t *testing.T) {
	t.Setenv("CLEANR_HOME", filepath.Join(t.TempDir(), "state"))
	configPath := filepath.Join(t.TempDir(), "cleanr.yaml")

	restoreStdin := swapStdin(t, "openai\nresponses\ngpt-4.1-mini\nOPENAI_API_KEY\nsk-test\n")
	defer restoreStdin()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"setup", "-output", configPath, "-trend-gate-preset", "exploratory"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	cfg, err := cleanr.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	if cfg.Reporting.TrendGates.Preset != "exploratory" {
		t.Fatalf("expected exploratory preset, got %+v", cfg.Reporting.TrendGates)
	}
	if cfg.Reporting.TrendGates.Enabled {
		t.Fatalf("expected exploratory preset to be non-blocking, got %+v", cfg.Reporting.TrendGates)
	}
}

func TestSetupCommandCIModePRProfileAppliesLightweightDefaults(t *testing.T) {
	t.Setenv("CLEANR_HOME", filepath.Join(t.TempDir(), "state"))
	configPath := filepath.Join(t.TempDir(), "cleanr.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{
		"setup",
		"--ci",
		"-output", configPath,
		"-provider", "openai",
		"-model", "gpt-4.1-mini",
		"-profile", "pr",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	cfg, err := cleanr.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	if cfg.Suites.Load.Enabled || cfg.Suites.Chaos.Enabled {
		t.Fatalf("expected pr profile to disable load and chaos, got %+v", cfg.Suites)
	}
	if !cfg.Suites.Drift.Enabled || cfg.Suites.Drift.Iterations != 2 {
		t.Fatalf("expected pr profile light drift, got %+v", cfg.Suites.Drift)
	}
	if cfg.Suites.Drift.BaselineFile != filepath.Join("snapshots", "openai-responses.snapshots.yaml") {
		t.Fatalf("unexpected pr baseline path: %s", cfg.Suites.Drift.BaselineFile)
	}
	if cfg.Reporting.TrendGates.Preset != "exploratory" {
		t.Fatalf("expected pr profile exploratory gates, got %+v", cfg.Reporting.TrendGates)
	}
	if cfg.Reporting.ReplayArtifactFile != filepath.Join("reports", "openai-responses.replay.json") {
		t.Fatalf("unexpected pr replay path: %s", cfg.Reporting.ReplayArtifactFile)
	}
}

func TestSetupCommandCIModeWritesConfigWithoutProfile(t *testing.T) {
	t.Setenv("CLEANR_HOME", filepath.Join(t.TempDir(), "state"))
	configPath := filepath.Join(t.TempDir(), "cleanr.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{
		"setup",
		"--ci",
		"-output", configPath,
		"-provider", "anthropic",
		"-model", "claude-sonnet-4-20250514",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	cfg, err := cleanr.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	if cfg.Target.Type != "anthropic" || cfg.Target.Anthropic.Model != "claude-sonnet-4-20250514" {
		t.Fatalf("unexpected ci config target: %+v", cfg.Target)
	}
	if _, err := profilepkg.Load(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no saved profile in ci mode, got %v", err)
	}
	if !strings.Contains(stdout.String(), "wrote CI starter config") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestSetupCommandRejectsInvalidProfile(t *testing.T) {
	t.Setenv("CLEANR_HOME", filepath.Join(t.TempDir(), "state"))
	configPath := filepath.Join(t.TempDir(), "cleanr.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{
		"setup",
		"--ci",
		"-output", configPath,
		"-provider", "openai",
		"-model", "gpt-4.1-mini",
		"-profile", "bad-profile",
	}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "profile must be one of pr, main, or release") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestSetupCommandCIModeCanEmitIntegrationReadyConfig(t *testing.T) {
	t.Setenv("CLEANR_HOME", filepath.Join(t.TempDir(), "state"))
	configPath := filepath.Join(t.TempDir(), "cleanr.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{
		"setup",
		"--ci",
		"-output", configPath,
		"-provider", "openai",
		"-model", "gpt-4.1-mini",
		"-with-braintrust",
		"-braintrust-project", "support-ai",
		"-with-langfuse",
		"-with-posthog",
		"-with-webhook",
		"-webhook-endpoint", "https://example.com/cleanr",
		"-with-attestation",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	cfg, err := cleanr.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	if len(cfg.Integrations.ResultSinks) != 4 {
		t.Fatalf("expected 4 result sinks, got %+v", cfg.Integrations.ResultSinks)
	}
	if len(cfg.Integrations.TrendSources) != 1 || cfg.Integrations.TrendSources[0].Type != "braintrust" {
		t.Fatalf("expected braintrust trend source, got %+v", cfg.Integrations.TrendSources)
	}
	if len(cfg.Integrations.Summaries) != 2 {
		t.Fatalf("expected markdown and json summaries, got %+v", cfg.Integrations.Summaries)
	}
	if !cfg.Governance.Attestation.Enabled || cfg.Governance.Attestation.KeyEnv != "CLEANR_ATTESTATION_KEY" {
		t.Fatalf("unexpected attestation config: %+v", cfg.Governance.Attestation)
	}

	var sawBraintrust bool
	var sawLangfuse bool
	var sawPostHog bool
	var sawWebhook bool
	for _, sink := range cfg.Integrations.ResultSinks {
		switch sink.Type {
		case "braintrust":
			sawBraintrust = sink.Project == "support-ai" && sink.APIKeyEnv == "BRAINTRUST_API_KEY"
		case "langfuse":
			sawLangfuse = sink.PublicKeyEnv == "LANGFUSE_PUBLIC_KEY" && sink.SecretKeyEnv == "LANGFUSE_SECRET_KEY"
		case "posthog":
			sawPostHog = sink.ProjectTokenEnv == "POSTHOG_PROJECT_TOKEN"
		case "http":
			sawWebhook = sink.Endpoint == "https://example.com/cleanr" && sink.APIKeyEnv == "CLEANR_RESULTS_WEBHOOK_TOKEN"
		}
	}
	if !sawBraintrust || !sawLangfuse || !sawPostHog || !sawWebhook {
		t.Fatalf("missing expected sink wiring: %+v", cfg.Integrations.ResultSinks)
	}
}

func TestSetupCommandRejectsBraintrustWithoutProject(t *testing.T) {
	t.Setenv("CLEANR_HOME", filepath.Join(t.TempDir(), "state"))
	configPath := filepath.Join(t.TempDir(), "cleanr.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{
		"setup",
		"--ci",
		"-output", configPath,
		"-provider", "openai",
		"-model", "gpt-4.1-mini",
		"-with-braintrust",
	}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "braintrust project is required") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestSetupAgentCommandCIModeUsesFlags(t *testing.T) {
	t.Setenv("CLEANR_HOME", filepath.Join(t.TempDir(), "state"))
	configPath := filepath.Join(t.TempDir(), "cleanr.agent.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{
		"setup", "agent",
		"--ci",
		"-output", configPath,
		"-provider", "openai",
		"-model", "gpt-4.1-mini",
		"-system-prompt", "You are a safe support agent.",
		"-user-prompt", "Reset the password and confirm the email.",
		"-name", "support-agent",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	cfg, err := cleanr.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	if cfg.Target.Type != "openai" || cfg.Target.Name != "support-agent" {
		t.Fatalf("unexpected agent ci target: %+v", cfg.Target)
	}
	if cfg.Scenarios[0].System != "You are a safe support agent." {
		t.Fatalf("unexpected system prompt: %+v", cfg.Scenarios[0])
	}
	if !strings.Contains(stdout.String(), "wrote CI agent config") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestSetupAgentCommandCIModeReleaseProfileAppliesHeavyweightDefaults(t *testing.T) {
	t.Setenv("CLEANR_HOME", filepath.Join(t.TempDir(), "state"))
	configPath := filepath.Join(t.TempDir(), "cleanr.agent.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{
		"setup", "agent",
		"--ci",
		"-output", configPath,
		"-provider", "openai",
		"-model", "gpt-4.1-mini",
		"-system-prompt", "You are a safe support agent.",
		"-user-prompt", "Reset the password and confirm the email.",
		"-name", "support-agent",
		"-profile", "release",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	cfg, err := cleanr.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	if !cfg.Suites.Load.Enabled || !cfg.Suites.Chaos.Enabled {
		t.Fatalf("expected release profile to enable load and chaos, got %+v", cfg.Suites)
	}
	if !cfg.Suites.ReleasePolicy.Enabled || len(cfg.Suites.ReleasePolicy.Rules) == 0 {
		t.Fatalf("expected release profile release policy, got %+v", cfg.Suites.ReleasePolicy)
	}
	if cfg.Reporting.ReplayArtifactFile != filepath.Join("reports", "support-agent.replay.json") {
		t.Fatalf("unexpected release replay path: %s", cfg.Reporting.ReplayArtifactFile)
	}
	if !cfg.Governance.Attestation.Enabled || cfg.Governance.Attestation.Output != filepath.Join("reports", "support-agent.attestation.json") {
		t.Fatalf("expected release profile attestation, got %+v", cfg.Governance.Attestation)
	}
}

func TestSetupAgentCommandCIModeReadsIntegrationFlagsFromEnv(t *testing.T) {
	t.Setenv("CLEANR_HOME", filepath.Join(t.TempDir(), "state"))
	t.Setenv("CLEANR_WITH_BRAINTRUST", "true")
	t.Setenv("CLEANR_BRAINTRUST_PROJECT", "agent-evals")
	t.Setenv("CLEANR_WITH_ATTESTATION", "true")
	configPath := filepath.Join(t.TempDir(), "cleanr.agent.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{
		"setup", "agent",
		"--ci",
		"-output", configPath,
		"-provider", "openai",
		"-model", "gpt-4.1-mini",
		"-system-prompt", "You are a safe support agent.",
		"-user-prompt", "Reset the password and confirm the email.",
		"-name", "support-agent",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	cfg, err := cleanr.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	if len(cfg.Integrations.ResultSinks) != 1 || cfg.Integrations.ResultSinks[0].Type != "braintrust" {
		t.Fatalf("expected env-driven braintrust sink, got %+v", cfg.Integrations.ResultSinks)
	}
	if !cfg.Governance.Attestation.Enabled {
		t.Fatalf("expected env-driven attestation, got %+v", cfg.Governance.Attestation)
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
