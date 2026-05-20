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

func TestDevtoolsHomebrewFormula(t *testing.T) {
	repo := t.TempDir()
	tag := "v1.2.3"
	checksumPath := filepath.Join(repo, "dist", "releases", tag, "SHA256SUMS")
	mustWriteFile(t, checksumPath, strings.Join([]string{
		"aaaa1111 cleanr_v1.2.3_darwin_amd64.tar.gz",
		"bbbb2222 cleanr_v1.2.3_darwin_arm64.tar.gz",
		"cccc3333 cleanr_v1.2.3_linux_amd64.tar.gz",
		"dddd4444 cleanr_v1.2.3_linux_arm64.tar.gz",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.HomebrewFormula(devtools.HomebrewFormulaOptions{
		Version:    tag,
		Checksums:  "dist/releases/v1.2.3/SHA256SUMS",
		Repository: "alxxjohn/cleanr",
		Output:     "dist/homebrew/cleanr.rb",
	}); err != nil {
		t.Fatalf("homebrew formula: %v", err)
	}

	formulaPath := filepath.Join(repo, "dist", "homebrew", "cleanr.rb")
	data, err := os.ReadFile(formulaPath)
	if err != nil {
		t.Fatalf("read formula: %v", err)
	}
	formula := string(data)
	if !strings.Contains(formula, `homepage "https://github.com/alxxjohn/cleanr"`) {
		t.Fatalf("formula missing homepage: %s", formula)
	}
	if !strings.Contains(formula, `version "1.2.3"`) {
		t.Fatalf("formula missing version: %s", formula)
	}
	if !strings.Contains(formula, `url "https://github.com/alxxjohn/cleanr/releases/download/v1.2.3/cleanr_v1.2.3_darwin_arm64.tar.gz"`) {
		t.Fatalf("formula missing darwin arm url: %s", formula)
	}
	if !strings.Contains(formula, `sha256 "dddd4444"`) {
		t.Fatalf("formula missing linux arm checksum: %s", formula)
	}
	if !strings.Contains(stdout.String(), "wrote Homebrew formula") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}

	if err := runner.HomebrewFormula(devtools.HomebrewFormulaOptions{
		Version:    tag,
		Checksums:  "dist/releases/v1.2.3/missing.txt",
		Repository: "alxxjohn/cleanr",
		Output:     "dist/homebrew/missing.rb",
	}); err == nil {
		t.Fatalf("expected missing checksum file error")
	}
}

func TestDevtoolsTestFiltersNoTestFilesOutput(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("GOCACHE", filepath.Join(repo, ".gocache"))
	mustWriteFile(t, filepath.Join(repo, "go.mod"), "module example.com/filtertest\n\ngo 1.20\n")
	mustWriteFile(t, filepath.Join(repo, "pkg", "notest.go"), "package pkg\n")
	mustWriteFile(t, filepath.Join(repo, "pkgtest", "pkgtest.go"), "package pkgtest\n")
	mustWriteFile(t, filepath.Join(repo, "pkgtest", "pkgtest_test.go"), "package pkgtest\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n")

	var stdout bytes.Buffer
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.Test(context.Background()); err != nil {
		t.Fatalf("test: %v", err)
	}

	output := stdout.String()
	if strings.Contains(output, "[no test files]") {
		t.Fatalf("expected no-test-files output to be filtered: %s", output)
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
