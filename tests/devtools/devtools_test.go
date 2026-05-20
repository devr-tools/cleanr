package tests

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cleanr/internal/devtools"
)

func TestDevtoolsGoFileLayoutAndFormatting(t *testing.T) {
	repo := t.TempDir()
	mustWriteFile(t, filepath.Join(repo, "cleanr", "main.go"), "package cleanr\n")
	mustWriteFile(t, filepath.Join(repo, "internal", "thing.go"), "package internal\n")
	mustWriteFile(t, filepath.Join(repo, "cmd", "app", "main.go"), "package main\n")
	mustWriteFile(t, filepath.Join(repo, "img", "banner.go"), "package img\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n")
	mustWriteFile(t, filepath.Join(repo, ".git", "ignored.go"), "package ignored\n")
	mustWriteFile(t, filepath.Join(repo, "dist", "ignored.go"), "package ignored\n")

	var stdout bytes.Buffer
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CheckGoFiles(); err != nil {
		t.Fatalf("check go files: %v", err)
	}
	if err := runner.ListGoFiles(); err != nil {
		t.Fatalf("list go files: %v", err)
	}
	if !strings.Contains(stdout.String(), "cleanr/main.go") || !strings.Contains(stdout.String(), "img/banner.go") || strings.Contains(stdout.String(), "ignored.go") {
		t.Fatalf("unexpected gofiles output: %s", stdout.String())
	}

	t.Setenv("PATH", scriptDir(t, map[string]string{
		"gofmt": "#!/bin/sh\nif [ \"$1\" = \"-l\" ]; then\n  if [ -n \"$GOFMT_OUTPUT\" ]; then\n    printf '%s\\n' \"$GOFMT_OUTPUT\"\n  fi\n  exit 0\nfi\nexit 0\n",
	})+":"+os.Getenv("PATH"))

	stdout.Reset()
	if err := runner.Format(context.Background()); err != nil {
		t.Fatalf("format: %v", err)
	}
	if !strings.Contains(stdout.String(), "formatting Go files") {
		t.Fatalf("unexpected format output: %s", stdout.String())
	}

	t.Setenv("GOFMT_OUTPUT", "")
	stdout.Reset()
	if err := runner.FormatCheck(context.Background()); err != nil {
		t.Fatalf("format check: %v", err)
	}
	if !strings.Contains(stdout.String(), "format check: ok") {
		t.Fatalf("unexpected format check output: %s", stdout.String())
	}

	t.Setenv("GOFMT_OUTPUT", "cleanr/main.go")
	if err := runner.FormatCheck(context.Background()); err == nil || !strings.Contains(err.Error(), "unformatted Go files") {
		t.Fatalf("expected unformatted file error, got %v", err)
	}
}

func TestDevtoolsCommandExecutionBuildReleaseAndFailures(t *testing.T) {
	repo := t.TempDir()
	mustWriteFile(t, filepath.Join(repo, "cleanr", "main.go"), "package cleanr\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "main_test.go"), "package tests\n")

	scripts := map[string]string{
		"go": `#!/bin/sh
printf '%s %s\n' "$0" "$*" >> "$CMD_LOG"
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then
    out="$arg"
  fi
  prev="$arg"
done
if [ -n "$out" ]; then
  mkdir -p "$(dirname "$out")"
  printf 'binary' > "$out"
fi
exit 0
`,
		"gofmt": "#!/bin/sh\nexit 0\n",
	}
	logPath := filepath.Join(t.TempDir(), "cmd.log")
	t.Setenv("CMD_LOG", logPath)
	t.Setenv("PATH", scriptDir(t, scripts)+":"+os.Getenv("PATH"))

	var stdout bytes.Buffer
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.Lint(context.Background()); err != nil {
		t.Fatalf("lint: %v", err)
	}
	if err := runner.Test(context.Background()); err != nil {
		t.Fatalf("test: %v", err)
	}
	if err := runner.Build(context.Background(), "dist/cleanr"); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := runner.Release(context.Background(), devtools.ReleaseOptions{Version: "v1.2.3", Output: "artifacts"}); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := runner.Check(context.Background()); err != nil {
		t.Fatalf("check: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "artifacts", "v1.2.3", "SHA256SUMS")); err != nil {
		t.Fatalf("expected release checksums: %v", err)
	}

	badRepo := t.TempDir()
	mustWriteFile(t, filepath.Join(badRepo, "bad.go"), "package bad\n")
	badRunner := devtools.NewRunner(badRepo, &stdout, &stdout)
	if err := badRunner.CheckGoFiles(); err == nil || !strings.Contains(err.Error(), "unexpected Go file location") {
		t.Fatalf("expected layout error, got %v", err)
	}
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func scriptDir(t *testing.T, scripts map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, contents := range scripts {
		mustWriteFile(t, filepath.Join(dir, name), contents)
	}
	return dir
}
