package tests

import (
	"os"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/mcpserver/toolkit"
)

func TestGuardMCPConfigRejectsExecSurfaces(t *testing.T) {
	t.Setenv(toolkit.MCPAllowExecEnv, "")

	cases := []struct {
		name string
		cfg  cleanr.Config
	}{
		{name: "cli target", cfg: cleanr.Config{Target: cleanr.TargetConfig{Type: "cli"}}},
		{name: "cli target mixed case", cfg: cleanr.Config{Target: cleanr.TargetConfig{Type: "CLI"}}},
		{name: "declared plugins", cfg: cleanr.Config{Plugins: []string{"./evil-plugin"}}},
		{name: "resolved probes", cfg: cleanr.Config{ResolvedPlugins: []cleanr.PluginManifest{
			{Probes: []cleanr.PluginProbe{{Name: "p"}}},
		}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := toolkit.GuardMCPConfig(tc.cfg); err == nil {
				t.Fatalf("expected guard to reject config %#v", tc.cfg)
			}
		})
	}
}

func TestGuardMCPConfigAllowsSafeConfig(t *testing.T) {
	t.Setenv(toolkit.MCPAllowExecEnv, "")
	cfg := cleanr.Config{Target: cleanr.TargetConfig{Type: "http", URL: "https://example.test"}}
	if err := toolkit.GuardMCPConfig(cfg); err != nil {
		t.Fatalf("expected safe config to pass, got %v", err)
	}
}

func TestGuardMCPConfigOptIn(t *testing.T) {
	t.Setenv(toolkit.MCPAllowExecEnv, "1")
	cfg := cleanr.Config{Target: cleanr.TargetConfig{Type: "cli"}, Plugins: []string{"./p"}}
	if err := toolkit.GuardMCPConfig(cfg); err != nil {
		t.Fatalf("expected opt-in to allow exec config, got %v", err)
	}
}

func TestLoadConfigSourceRejectsTraversal(t *testing.T) {
	rejected := []string{"/etc/passwd", "../../etc/passwd", ".."}
	for _, p := range rejected {
		_, err := toolkit.LoadConfigSource(toolkit.ConfigSource{ConfigPath: p})
		if err == nil {
			t.Fatalf("expected path %q to be rejected", p)
		}
		if strings.Contains(err.Error(), "passwd") {
			t.Fatalf("error should not echo path/content: %v", err)
		}
	}
}

func TestLoadConfigSourceAllowsWithinCWD(t *testing.T) {
	// The MCP path guard confines config_path to the process working directory,
	// so a CWD-relative config must load successfully.
	relPath := "cleanr_guard_test_config.json"
	if err := cleanr.WriteConfigFile(relPath, cleanr.ExampleConfig()); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(relPath) })

	if _, err := toolkit.LoadConfigSource(toolkit.ConfigSource{ConfigPath: relPath}); err != nil {
		t.Fatalf("expected in-CWD config to load, got %v", err)
	}
}
