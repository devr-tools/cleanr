package toolkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
)

func TestGuardMCPConfigRejectsExecSurfaces(t *testing.T) {
	t.Setenv(MCPAllowExecEnv, "")

	cases := []struct {
		name string
		cfg  cleanr.Config
	}{
		{
			name: "cli target",
			cfg:  cleanr.Config{Target: cleanr.TargetConfig{Type: "cli"}},
		},
		{
			name: "cli target mixed case",
			cfg:  cleanr.Config{Target: cleanr.TargetConfig{Type: "CLI"}},
		},
		{
			name: "declared plugins",
			cfg:  cleanr.Config{Plugins: []string{"./evil-plugin"}},
		},
		{
			name: "resolved probes",
			cfg: cleanr.Config{ResolvedPlugins: []cleanr.PluginManifest{
				{Probes: []cleanr.PluginProbe{{Name: "p"}}},
			}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := GuardMCPConfig(tc.cfg); err == nil {
				t.Fatalf("expected guard to reject config %#v", tc.cfg)
			}
		})
	}
}

func TestGuardMCPConfigAllowsSafeConfig(t *testing.T) {
	t.Setenv(MCPAllowExecEnv, "")
	cfg := cleanr.Config{Target: cleanr.TargetConfig{Type: "http", URL: "https://example.test"}}
	if err := GuardMCPConfig(cfg); err != nil {
		t.Fatalf("expected safe config to pass, got %v", err)
	}
}

func TestGuardMCPConfigOptIn(t *testing.T) {
	t.Setenv(MCPAllowExecEnv, "1")
	cfg := cleanr.Config{Target: cleanr.TargetConfig{Type: "cli"}, Plugins: []string{"./p"}}
	if err := GuardMCPConfig(cfg); err != nil {
		t.Fatalf("expected opt-in to allow exec config, got %v", err)
	}
}

func TestSecureLocalPathRejectsTraversal(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	rejected := []string{
		"/etc/passwd",
		"../../etc/passwd",
		"..",
		filepath.Join(cwd, "..", "outside.json"),
		"",
	}
	for _, p := range rejected {
		if _, err := secureLocalPath(p); err == nil {
			t.Fatalf("expected %q to be rejected", p)
		}
	}
}

func TestSecureLocalPathErrorDoesNotEchoContent(t *testing.T) {
	_, err := secureLocalPath("/etc/passwd")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "passwd") {
		t.Fatalf("error should not echo path/content: %v", err)
	}
}

func TestSecureLocalPathAllowsWithinCWD(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	resolved, err := secureLocalPath("sub/dir/config.json")
	if err != nil {
		t.Fatalf("expected in-CWD path to be allowed, got %v", err)
	}
	if !strings.HasPrefix(resolved, cwd) {
		t.Fatalf("resolved path %q not within cwd %q", resolved, cwd)
	}
}
