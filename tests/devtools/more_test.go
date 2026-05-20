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

func TestDevtoolsHomebrewValidationErrors(t *testing.T) {
	repo := t.TempDir()
	var stdout bytes.Buffer
	runner := devtools.NewRunner(repo, &stdout, &stdout)

	if err := runner.HomebrewFormula(devtools.HomebrewFormulaOptions{}); err == nil || !strings.Contains(err.Error(), "version is required") {
		t.Fatalf("expected missing version error, got %v", err)
	}
	if err := runner.HomebrewFormula(devtools.HomebrewFormulaOptions{Version: "v1.0.0"}); err == nil || !strings.Contains(err.Error(), "repository is required") {
		t.Fatalf("expected missing repository error, got %v", err)
	}
	if err := runner.HomebrewFormula(devtools.HomebrewFormulaOptions{
		Version:    "v1.0.0",
		Repository: "owner/repo",
	}); err == nil || !strings.Contains(err.Error(), "source SHA256 is required") {
		t.Fatalf("expected missing source SHA256 error, got %v", err)
	}
}

func TestDevtoolsGoTestFilteredDecodeAndLookupErrors(t *testing.T) {
	repo := t.TempDir()
	mustWriteFile(t, filepath.Join(repo, "cleanr", "main.go"), "package cleanr\n")

	var stdout bytes.Buffer
	runner := devtools.NewRunner(repo, &stdout, &stdout)

	t.Setenv("PATH", scriptDir(t, map[string]string{
		"go": "#!/bin/sh\nprintf 'not-json\\n'\n",
	})+":"+os.Getenv("PATH"))
	if err := runner.Test(context.Background()); err == nil || !strings.Contains(err.Error(), "decode go test json") {
		t.Fatalf("expected decode json error, got %v", err)
	}

	t.Setenv("PATH", t.TempDir())
	if err := runner.Test(context.Background()); err == nil || !strings.Contains(err.Error(), "find go") {
		t.Fatalf("expected find go error, got %v", err)
	}
}
