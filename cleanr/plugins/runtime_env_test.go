package plugins

import (
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func hasEnvKey(env []string, key string) bool {
	for _, kv := range env {
		if strings.HasPrefix(kv, key+"=") {
			return true
		}
	}
	return false
}

func TestBuildEntryEnvDoesNotLeakHostSecrets(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-secret")
	t.Setenv("CLEANR_PLUGIN_TOKEN", "allowed")

	entry := Entry{
		Env:    map[string]string{"DECLARED": "1"},
		Plugin: core.PluginManifest{Name: "p", BaseDir: "/tmp/p"},
	}
	env := buildEntryEnv(entry)

	if hasEnvKey(env, "OPENAI_API_KEY") {
		t.Fatal("host secret leaked into plugin subprocess env")
	}
	if !hasEnvKey(env, "DECLARED") {
		t.Fatal("declared env var missing")
	}
	if !hasEnvKey(env, "CLEANR_PLUGIN_TOKEN") {
		t.Fatal("CLEANR_PLUGIN_* passthrough missing for subprocess")
	}
	if !hasEnvKey(env, "CLEANR_PLUGIN_DIR") {
		t.Fatal("plugin context var missing")
	}
}

func TestBuildWASMEnvInjectsOnlyManifestDeclared(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-secret")
	t.Setenv("CLEANR_PLUGIN_TOKEN", "allowed")

	entry := Entry{
		Env:    map[string]string{"DECLARED": "1"},
		Plugin: core.PluginManifest{Name: "p", BaseDir: "/tmp/p"},
	}
	env := buildWASMEnv(entry)

	if hasEnvKey(env, "OPENAI_API_KEY") {
		t.Fatal("host secret leaked into WASM sandbox")
	}
	if hasEnvKey(env, "CLEANR_PLUGIN_TOKEN") {
		t.Fatal("WASM sandbox must not receive host CLEANR_PLUGIN_* passthrough")
	}
	if !hasEnvKey(env, "DECLARED") {
		t.Fatal("declared env var missing from WASM env")
	}
	if !hasEnvKey(env, "CLEANR_PLUGIN_DIR") {
		t.Fatal("manifest-derived plugin context var missing from WASM env")
	}
}
