package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"cleanr/cleanr"
	"cleanr/internal/testutil"
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
