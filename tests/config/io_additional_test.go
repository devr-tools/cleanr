package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/testutil"
)

func TestConfigMarshalAndWriteSupportJSONAndYAML(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()

	jsonPath := filepath.Join(t.TempDir(), "cleanr.json")
	if err := cleanr.WriteConfigFile(jsonPath, cfg); err != nil {
		t.Fatalf("write json config: %v", err)
	}
	jsonCfg, err := cleanr.LoadConfigFile(jsonPath)
	if err != nil {
		t.Fatalf("load json config: %v", err)
	}
	if jsonCfg.Target.Name != cfg.Target.Name {
		t.Fatalf("unexpected json config round trip: %+v", jsonCfg)
	}

	yamlPath := filepath.Join(t.TempDir(), "cleanr.yaml")
	if err := cleanr.WriteConfigFile(yamlPath, cfg); err != nil {
		t.Fatalf("write yaml config: %v", err)
	}
	yamlCfg, err := cleanr.LoadConfigFile(yamlPath)
	if err != nil {
		t.Fatalf("load yaml config: %v", err)
	}
	if yamlCfg.Target.Name != cfg.Target.Name {
		t.Fatalf("unexpected yaml config round trip: %+v", yamlCfg)
	}
}

func TestConfigMarshalAndLoadCoverErrorAndFormatBranches(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.Target.RequestTemplate = func() {}
	if _, err := cleanr.MarshalConfig(cfg, "json"); err == nil {
		t.Fatal("expected json marshal failure")
	}
	if _, err := cleanr.MarshalConfig(cfg, "yaml"); err == nil {
		t.Fatal("expected yaml marshal failure")
	}

	if _, err := cleanr.LoadConfigData([]byte(`{"target":{"url":"https://example.com","prompt_field":"input","response_field":"output.text"},"scenarios":[{"name":"x","input":"y"}]}`), ""); err != nil {
		t.Fatalf("load inline json: %v", err)
	}
	if _, err := cleanr.LoadConfigData([]byte("target:\n  url: https://example.com\n  prompt_field: input\n  response_field: output.text\nscenarios:\n  - name: x\n    input: y\n"), " yml "); err != nil {
		t.Fatalf("load inline yaml: %v", err)
	}

	path := testutil.WriteNamedConfigFile(t, "broken.yaml", "key: [unterminated")
	if _, err := cleanr.LoadConfigFile(path); err == nil || !strings.Contains(err.Error(), "decode config:") {
		t.Fatalf("expected yaml decode failure, got %v", err)
	}
}

func TestLoadConfigFileAppliesPolicyPacksBeforeValidation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	packPath := filepath.Join(dir, "support-strict.yaml")
	if err := os.WriteFile(packPath, []byte(`
suites:
  claim_trace:
    enabled: true
  release_policy:
    enabled: true
    rules:
      - type: tool
        mode: require_approval
        tools:
          - send_email
reporting:
  trend_file: reports/cleanr.trends.yaml
  trend_gates:
    preset: moderate
`), 0o644); err != nil {
		t.Fatalf("write policy pack: %v", err)
	}

	configPath := filepath.Join(dir, "cleanr.yaml")
	if err := os.WriteFile(configPath, []byte(`
version: v1alpha1
policy_packs:
  - ./support-strict.yaml
target:
  url: https://example.com/v1/chat
  prompt_field: input
  response_field: output.text
scenarios:
  - name: refund-summary
    input: Summarize the refund policy.
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := cleanr.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load config with policy pack: %v", err)
	}
	if !cfg.Suites.ClaimTrace.Enabled || !cfg.Suites.ReleasePolicy.Enabled {
		t.Fatalf("expected policy pack suites to apply, got %+v", cfg.Suites)
	}
	if len(cfg.Suites.ReleasePolicy.Rules) != 1 || cfg.Suites.ReleasePolicy.Rules[0].Tools[0] != "send_email" {
		t.Fatalf("expected pack release-policy rules, got %+v", cfg.Suites.ReleasePolicy.Rules)
	}
	if cfg.Reporting.TrendGates.Preset != "moderate" || !cfg.Reporting.TrendGates.Enabled {
		t.Fatalf("expected policy pack trend gate preset, got %+v", cfg.Reporting.TrendGates)
	}
}

func TestLoadConfigFileResolvesPluginManifest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	packPath := filepath.Join(dir, "plugin-pack.yaml")
	if err := os.WriteFile(packPath, []byte(`
suites:
  provenance:
    enabled: true
  release_policy:
    enabled: true
    rules:
      - type: tool
        mode: deny
        tools:
          - send_email
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
probes:
  - name: db-ticket-probe
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

	cfg, err := cleanr.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.ResolvedPlugins) != 1 || cfg.ResolvedPlugins[0].Name != "workflow-plugin" {
		t.Fatalf("expected resolved plugin manifest, got %+v", cfg.ResolvedPlugins)
	}
	if len(cfg.ResolvedPlugins[0].Probes) != 1 || cfg.ResolvedPlugins[0].Probes[0].Name != "db-ticket-probe" {
		t.Fatalf("expected resolved plugin probes, got %+v", cfg.ResolvedPlugins[0].Probes)
	}
	if !cfg.Suites.Provenance.Enabled {
		t.Fatalf("expected plugin policy pack to apply, got %+v", cfg.Suites.Provenance)
	}
	if !cfg.Suites.ReleasePolicy.Enabled {
		t.Fatalf("expected plugin release policy to apply, got %+v", cfg.Suites.ReleasePolicy)
	}
	if len(cfg.Suites.ReleasePolicy.Rules) != 1 || len(cfg.Suites.ReleasePolicy.Rules[0].Tools) != 1 || cfg.Suites.ReleasePolicy.Rules[0].Tools[0] != "send_email" {
		t.Fatalf("expected plugin policy rules, got %+v", cfg.Suites.ReleasePolicy.Rules)
	}
}

func TestLoadConfigFileRejectsPluginProbeWithoutCommand(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "workflow-plugin.yaml")
	if err := os.WriteFile(pluginPath, []byte(`
name: workflow-plugin
probes:
  - name: db-ticket-probe
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

	_, err := cleanr.LoadConfigFile(configPath)
	if err == nil || !strings.Contains(err.Error(), "probe[0] is missing command") {
		t.Fatalf("expected missing probe command error, got %v", err)
	}
}

func TestScenarioGenerationConfigRoundTrips(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = nil
	cfg.ScenarioGeneration = cleanr.ScenarioGenerationConfig{
		Enabled: true,
		Provider: cleanr.TargetConfig{
			Type: "openai",
			OpenAI: cleanr.OpenAIConfig{
				APIMode:   "responses",
				Model:     "gpt-4.1-mini",
				APIKeyEnv: "OPENAI_API_KEY",
			},
		},
		Spec: cleanr.ScenarioGenerationSpec{
			AppKind:      "support-assistant",
			Goals:        []string{"refund policy"},
			RiskAreas:    []string{"prompt injection"},
			Instructions: "Focus on realistic customer prompts.",
		},
		OutputFile:    "generated/cleanr.dataset.yaml",
		Count:         4,
		RequireReview: true,
	}

	path := filepath.Join(t.TempDir(), "cleanr.yaml")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loaded, err := cleanr.LoadConfigFile(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !loaded.ScenarioGeneration.Enabled || loaded.ScenarioGeneration.Spec.AppKind != "support-assistant" {
		t.Fatalf("unexpected scenario generation config: %+v", loaded.ScenarioGeneration)
	}
	if loaded.ScenarioGeneration.Provider.OpenAI.Model != "gpt-4.1-mini" || loaded.ScenarioGeneration.OutputFile != "generated/cleanr.dataset.yaml" {
		t.Fatalf("unexpected provider/output config: %+v", loaded.ScenarioGeneration)
	}
}
