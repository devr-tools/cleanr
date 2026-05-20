package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func WriteConfigFile(t testing.TB, payload string) string {
	t.Helper()

	return WriteNamedConfigFile(t, "config.json", payload)
}

func WriteNamedConfigFile(t testing.TB, name, payload string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
