package tests

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/internal/devtools"
)

func TestDevtoolsCIPassWithLocalBaseRef(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/local-ci")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n\nconst localCI = true\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CI(context.Background(), devtools.CIOptions{BaseRef: "main"}); err != nil {
		t.Fatalf("ci: %v\n%s", err, stdout.String())
	}

	if !strings.Contains(stdout.String(), "local ci: ok") {
		t.Fatalf("expected successful ci output, got: %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(repo, "dist", "cleanr-linux-amd64")); err != nil {
		t.Fatalf("expected ci build artifact: %v", err)
	}
	semgrepArgs, err := os.ReadFile(filepath.Join(repo, ".semgrep.log"))
	if err != nil {
		t.Fatalf("read semgrep log: %v", err)
	}
	if !strings.Contains(string(semgrepArgs), "--baseline-commit") {
		t.Fatalf("expected semgrep baseline commit args, got: %s", string(semgrepArgs))
	}
}

func TestDevtoolsCIFailsWithoutTestUpdate(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/missing-test")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	err := runner.CI(context.Background(), devtools.CIOptions{BaseRef: "main"})
	if err == nil || !strings.Contains(err.Error(), "test-presence failed") {
		t.Fatalf("expected test-presence failure, got %v\n%s", err, stdout.String())
	}
}

func TestDevtoolsCIAllowsReleaseVersionBumpWithoutTestUpdate(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/release-version-bump")

	mustWriteFile(t, filepath.Join(repo, "internal", "version", "version.go"), "package version\n\nconst Number = \"0.1.1\" // x-release-please-version\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CI(context.Background(), devtools.CIOptions{BaseRef: "main"}); err != nil {
		t.Fatalf("expected release version bump to bypass test-presence failure, got %v\n%s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "No Go source changes that require a test presence check.") {
		t.Fatalf("expected test-presence skip output, got: %s", stdout.String())
	}
}

func TestDevtoolsCIHandlesNoMergeBase(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")

	runGit(t, repo, "checkout", "--orphan", "feature/orphan-ci")
	writeCIBaseFiles(t, repo)
	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n\nconst orphanCI = true\n")
	gitCommitAll(t, repo, "orphan ci commit\n\nSigned-off-by: Test User <test@example.com>\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CI(context.Background(), devtools.CIOptions{BaseRef: "main"}); err != nil {
		t.Fatalf("expected ci to fall back when no merge base exists, got %v\n%s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "warning: no merge base with main; falling back to direct diff") {
		t.Fatalf("expected no-merge-base warning, got: %s", stdout.String())
	}
}

func TestDevtoolsCIDevelopRequiresDocs(t *testing.T) {
	repo := initGitRepo(t, "develop")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/doc-review")

	mustWriteFile(t, filepath.Join(repo, "internal", "devtools", "note.go"), "package devtools\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n\nconst docReview = true\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	err := runner.CI(context.Background(), devtools.CIOptions{BaseRef: "develop"})
	if err == nil || !strings.Contains(err.Error(), "doc-review failed") {
		t.Fatalf("expected doc-review failure, got %v\n%s", err, stdout.String())
	}
}

func TestDevtoolsCIDevelopChecksDCO(t *testing.T) {
	repo := initGitRepo(t, "develop")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/dco")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n\nconst dcoCI = true\n")
	mustWriteFile(t, filepath.Join(repo, "docs", "development.md"), "# Updated\n")
	gitCommitAll(t, repo, "feature commit without signoff")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	err := runner.CI(context.Background(), devtools.CIOptions{BaseRef: "develop"})
	if err == nil || !strings.Contains(err.Error(), "dco failed") {
		t.Fatalf("expected dco failure, got %v\n%s", err, stdout.String())
	}
}

func TestDevtoolsCIToleratesBaselineGocycloDebt(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/gocyclo-baseline")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n\nconst gocycloBaseline = true\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	t.Setenv("GOCYCLO_OUTPUT_CURRENT", "21 cleanr Value cleanr/app.go:3:1")
	t.Setenv("GOCYCLO_OUTPUT_BASE", "21 cleanr Value cleanr/app.go:3:1")

	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CI(context.Background(), devtools.CIOptions{BaseRef: "main"}); err != nil {
		t.Fatalf("expected baseline gocyclo debt to be tolerated, got %v\n%s", err, stdout.String())
	}
}

func TestDevtoolsCIToleratesBaselineSCCDebt(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/scc-baseline")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n\nconst sccBaseline = true\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	t.Setenv("SCC_OUTPUT_CURRENT", `[{"Files":[{"Code":401,"Lines":430,"Location":"cleanr/app.go"}]}]`)
	t.Setenv("SCC_OUTPUT_BASE", `[{"Files":[{"Code":401,"Lines":430,"Location":"cleanr/app.go"}]}]`)

	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CI(context.Background(), devtools.CIOptions{BaseRef: "main"}); err != nil {
		t.Fatalf("expected baseline scc debt to be tolerated, got %v\n%s", err, stdout.String())
	}
}

func TestDevtoolsCodeGuardSummarizesSections(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/codeguard-summary")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CodeGuard(context.Background(), devtools.CIOptions{BaseRef: "main"}); err != nil {
		t.Fatalf("expected codeguard to pass, got %v\n%s", err, stdout.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"Code Guard",
		"| Check",
		"God Files",
		"Cyclomatic Complexity",
		"Maintainability Lint",
		"DRY",
		"Clean Code",
		"Design Principles (SOLID/SoC)",
		"Security: Go Vulnerabilities",
		"Security: Critical Files",
		"RESULT: PASS",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}

func TestDevtoolsCodeGuardReportsQualityWarnings(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/codeguard-quality-warning")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "quality.go"), `package cleanr

func duplicateOne(enabled, verbose bool) int {
	if enabled {
		if verbose {
			if enabled {
				if verbose {
					if enabled {
						return 1
					}
				}
			}
		}
	}
	return 0
}

func duplicateTwo(enabled, verbose bool) int {
	if enabled {
		if verbose {
			if enabled {
				if verbose {
					if enabled {
						return 1
					}
				}
			}
		}
	}
	return 0
}
`)

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	t.Setenv("MAX_NESTING_DEPTH", "2")
	t.Setenv("MAX_BOOL_PARAMS", "1")
	t.Setenv("DRY_MIN_FUNCTION_LINES", "3")

	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CodeGuard(context.Background(), devtools.CIOptions{BaseRef: "main"}); err != nil {
		t.Fatalf("expected warning-only codeguard to pass, got %v\n%s", err, stdout.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"DRY",
		"Clean Code",
		"Design Principles (SOLID/SoC)",
		"WARN",
		"Warnings (non-blocking):",
		"duplicate function bodies",
		"bool params (>1)",
		"RESULT: PASS",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}

func TestDevtoolsCodeGuardReportsSecurityWarnings(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/codeguard-security-warning")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	t.Setenv("GOVULNCHECK_EXIT", "1")
	t.Setenv("GOVULNCHECK_OUTPUT", "stdlib package vulnerable")

	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CodeGuard(context.Background(), devtools.CIOptions{BaseRef: "main"}); err != nil {
		t.Fatalf("expected warning-only codeguard to pass, got %v\n%s", err, stdout.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"Security: Go Vulnerabilities",
		"WARN",
		"Warnings (non-blocking):",
		"stdlib package vulnerable",
		"RESULT: PASS",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}

func TestDevtoolsCodeGuardAllowsGodFileAllowlist(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/codeguard-allowlist")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")
	mustWriteFile(t, filepath.Join(repo, ".codeguard-godfiles-allowlist"), "cleanr/app.go\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	t.Setenv("SCC_OUTPUT_CURRENT", `[{"Files":[{"Code":405,"Lines":440,"Location":"cleanr/app.go"}]}]`)
	t.Setenv("SCC_OUTPUT_BASE", `[{"Files":[{"Code":120,"Lines":140,"Location":"cleanr/app.go"}]}]`)

	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CodeGuard(context.Background(), devtools.CIOptions{BaseRef: "main"}); err != nil {
		t.Fatalf("expected allowlisted god file to pass, got %v\n%s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "RESULT: PASS") {
		t.Fatalf("expected passing codeguard output, got:\n%s", stdout.String())
	}
}

func TestDevtoolsCodeGuardHonorsFunctionComplexityThresholdEnv(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/codeguard-complexity-threshold")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	t.Setenv("MAX_FUNCTION_COMPLEXITY", "10")
	t.Setenv("GOCYCLO_OUTPUT_CURRENT", "11 cleanr Value cleanr/app.go:3:1")
	t.Setenv("GOCYCLO_OUTPUT_BASE", "")

	runner := devtools.NewRunner(repo, &stdout, &stdout)
	err := runner.CodeGuard(context.Background(), devtools.CIOptions{BaseRef: "main"})
	if err == nil || !strings.Contains(err.Error(), "codeguard failed") {
		t.Fatalf("expected codeguard complexity failure, got %v\n%s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "functions above complexity 10") {
		t.Fatalf("expected threshold note in output, got:\n%s", stdout.String())
	}
}

func TestDevtoolsCIFailsOnNewSCCGodFile(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/scc-failure")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n\nconst sccFailure = true\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	t.Setenv("SCC_OUTPUT_CURRENT", `[{"Files":[{"Code":405,"Lines":440,"Location":"cleanr/app.go"}]}]`)
	t.Setenv("SCC_OUTPUT_BASE", `[{"Files":[{"Code":120,"Lines":140,"Location":"cleanr/app.go"}]}]`)

	runner := devtools.NewRunner(repo, &stdout, &stdout)
	err := runner.CI(context.Background(), devtools.CIOptions{BaseRef: "main"})
	if err == nil || !strings.Contains(err.Error(), "codeguard failed") {
		t.Fatalf("expected scc failure, got %v\n%s", err, stdout.String())
	}
}

func TestDevtoolsCISCCFallsBackToPrebuiltBinary(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/scc-fallback")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n\nconst sccFallback = true\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	t.Setenv("SCC_INSTALL_ERROR", "go.mod:3: invalid go version '1.25.2': must match format 1.23")
	archivePath := filepath.Join(repo, "scc.tar.gz")
	if err := os.WriteFile(archivePath, buildBinaryTarGz(t, "scc", runtime.GOOS, runtime.GOARCH, "#!/bin/sh\nif [ \"$(pwd)\" = \"$WORKTREE_DIR\" ]; then\n  if [ -n \"$SCC_OUTPUT_CURRENT\" ]; then\n    printf '%s\\n' \"$SCC_OUTPUT_CURRENT\"\n  else\n    printf '%s\\n' '[]'\n  fi\nelse\n  if [ -n \"$SCC_OUTPUT_BASE\" ]; then\n    printf '%s\\n' \"$SCC_OUTPUT_BASE\"\n  else\n    printf '%s\\n' '[]'\n  fi\nfi\nexit 0\n"), 0o644); err != nil {
		t.Fatalf("write scc archive: %v", err)
	}
	t.Setenv("CLEANR_SCC_ARCHIVE_PATH", archivePath)

	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CISCC(context.Background(), devtools.CIOptions{BaseRef: "main"}); err != nil {
		t.Fatalf("expected scc fallback download to succeed, got %v\n%s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "falling back to prebuilt scc binary") {
		t.Fatalf("expected fallback message, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "scc: no new file-size regressions") {
		t.Fatalf("expected scc success output, got: %s", stdout.String())
	}
}

func TestDevtoolsCIFailsOnNewGolangCIIssue(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/golangci-failure")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n\nconst golangciFailure = true\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	t.Setenv("GOLANGCI_LINT_EXIT", "1")

	runner := devtools.NewRunner(repo, &stdout, &stdout)
	err := runner.CI(context.Background(), devtools.CIOptions{BaseRef: "main"})
	if err == nil || !strings.Contains(err.Error(), "codeguard failed") {
		t.Fatalf("expected golangci-lint failure, got %v\n%s", err, stdout.String())
	}
}

func TestDevtoolsCIGolangCILintFallsBackToPrebuiltBinary(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/golangci-fallback")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n\nconst golangciFallback = true\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	t.Setenv("GOLANGCI_LINT_INSTALL_ERROR", "go.mod:6: invalid go version '1.25.0': must match format 1.23\ngo.mod:8: unknown block type: ignore")
	archivePath := filepath.Join(repo, "golangci-lint.tar.gz")
	if err := os.WriteFile(archivePath, buildGolangCILintTarGz(t), 0o644); err != nil {
		t.Fatalf("write golangci-lint archive: %v", err)
	}
	t.Setenv("CLEANR_GOLANGCI_LINT_ARCHIVE_PATH", archivePath)
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CIGolangCILint(context.Background(), devtools.CIOptions{BaseRef: "main"}); err != nil {
		t.Fatalf("expected fallback download to succeed, got %v\n%s", err, stdout.String())
	}

	logData, err := os.ReadFile(filepath.Join(repo, ".golangci-lint.log"))
	if err != nil {
		t.Fatalf("read golangci-lint log: %v", err)
	}
	logText := string(logData)
	if !strings.Contains(logText, "--new-from-rev") || !strings.Contains(logText, "--whole-files") {
		t.Fatalf("unexpected golangci-lint args: %s", logText)
	}
	if !strings.Contains(stdout.String(), "falling back to prebuilt golangci-lint binary") {
		t.Fatalf("expected fallback message, got: %s", stdout.String())
	}
}

func initGitRepo(t *testing.T, branch string) string {
	t.Helper()

	repo := t.TempDir()
	runGit(t, repo, "init", "-b", branch)
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "user.email", "test@example.com")
	return repo
}

func writeCIBaseFiles(t *testing.T, repo string) {
	t.Helper()

	mustWriteFile(t, filepath.Join(repo, "go.mod"), "module example.com/cleanrtest\n\ngo 1.20\n")
	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 1 }\n")
	mustWriteFile(t, filepath.Join(repo, "cmd", "cleanr", "main.go"), "package main\n\nfunc main() {}\n")
	mustWriteFile(t, filepath.Join(repo, "internal", "thing.go"), "package internal\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n")
	mustWriteFile(t, filepath.Join(repo, "README.md"), "# cleanr test\n")
}

func gitCheckoutNewBranch(t *testing.T, repo, branch string) {
	t.Helper()
	runGit(t, repo, "checkout", "-b", branch)
}

func gitCommitAll(t *testing.T, repo, message string) {
	t.Helper()
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", message)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func configureFakeCIToolchain(t *testing.T, repo string) {
	t.Helper()

	fakeBin := scriptDir(t, map[string]string{
		"go": fakeGoScript(),
		"gofmt": `#!/bin/sh
if [ "$1" = "-l" ] && [ -n "$GOFMT_OUTPUT" ]; then
  printf '%s\n' "$GOFMT_OUTPUT"
fi
exit 0
`,
		"semgrep": `#!/bin/sh
printf '%s\n' "$*" >> "$SEMGREP_LOG"
exit "${SEMGREP_EXIT:-0}"
`,
	})

	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))
	t.Setenv("FAKE_GOPATH", filepath.Join(repo, ".fake-gopath"))
	t.Setenv("FAKE_GO_LOG", filepath.Join(repo, ".fake-go.log"))
	t.Setenv("FAKE_COVERAGE_TOTAL", "70.0")
	t.Setenv("SEMGREP_LOG", filepath.Join(repo, ".semgrep.log"))
	t.Setenv("GOLANGCI_LINT_LOG", filepath.Join(repo, ".golangci-lint.log"))
	t.Setenv("GOCACHE", filepath.Join(repo, ".gocache"))
	worktreeDir, err := filepath.EvalSymlinks(repo)
	if err != nil {
		worktreeDir = repo
	}
	t.Setenv("WORKTREE_DIR", worktreeDir)
}

func fakeGoScript() string {
	return `#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_GO_LOG"

case "$1" in
  env)
    if [ "$2" = "GOPATH" ]; then
      printf '%s\n' "$FAKE_GOPATH"
      exit 0
    fi
    ;;
  install)
    mkdir -p "$FAKE_GOPATH/bin"
    case "$2" in
      github.com/fzipp/gocyclo/cmd/gocyclo@*)
        cat > "$FAKE_GOPATH/bin/gocyclo" <<'EOF'
#!/bin/sh
if [ "$(pwd)" = "$WORKTREE_DIR" ]; then
  if [ -n "$GOCYCLO_OUTPUT_CURRENT" ]; then
    printf '%s\n' "$GOCYCLO_OUTPUT_CURRENT"
  elif [ -n "$GOCYCLO_OUTPUT" ]; then
    printf '%s\n' "$GOCYCLO_OUTPUT"
  fi
else
  if [ -n "$GOCYCLO_OUTPUT_BASE" ]; then
    printf '%s\n' "$GOCYCLO_OUTPUT_BASE"
  elif [ -n "$GOCYCLO_OUTPUT" ]; then
    printf '%s\n' "$GOCYCLO_OUTPUT"
  fi
fi
exit 0
EOF
        chmod +x "$FAKE_GOPATH/bin/gocyclo"
        exit 0
        ;;
      github.com/boyter/scc/v3@*)
        if [ -n "$SCC_INSTALL_ERROR" ]; then
          printf '%s\n' "$SCC_INSTALL_ERROR" >&2
          exit 1
        fi
        cat > "$FAKE_GOPATH/bin/scc" <<'EOF'
#!/bin/sh
if [ "$(pwd)" = "$WORKTREE_DIR" ]; then
  if [ -n "$SCC_OUTPUT_CURRENT" ]; then
    printf '%s\n' "$SCC_OUTPUT_CURRENT"
  else
    printf '%s\n' '[]'
  fi
else
  if [ -n "$SCC_OUTPUT_BASE" ]; then
    printf '%s\n' "$SCC_OUTPUT_BASE"
  else
    printf '%s\n' '[]'
  fi
fi
exit 0
EOF
        chmod +x "$FAKE_GOPATH/bin/scc"
        exit 0
        ;;
      github.com/golangci/golangci-lint/v2/cmd/golangci-lint@*)
        if [ -n "$GOLANGCI_LINT_INSTALL_ERROR" ]; then
          printf '%s\n' "$GOLANGCI_LINT_INSTALL_ERROR" >&2
          exit 1
        fi
        cat > "$FAKE_GOPATH/bin/golangci-lint" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >> "$GOLANGCI_LINT_LOG"
if [ -n "$GOLANGCI_LINT_OUTPUT" ]; then
  printf '%s\n' "$GOLANGCI_LINT_OUTPUT"
fi
exit "${GOLANGCI_LINT_EXIT:-0}"
EOF
        chmod +x "$FAKE_GOPATH/bin/golangci-lint"
        exit 0
        ;;
      golang.org/x/vuln/cmd/govulncheck@*)
        cat > "$FAKE_GOPATH/bin/govulncheck" <<'EOF'
#!/bin/sh
if [ -n "$GOVULNCHECK_OUTPUT" ]; then
  printf '%s\n' "$GOVULNCHECK_OUTPUT"
fi
exit "${GOVULNCHECK_EXIT:-0}"
EOF
        chmod +x "$FAKE_GOPATH/bin/govulncheck"
        exit 0
        ;;
    esac
    ;;
  vet)
    exit 0
    ;;
  build)
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
    ;;
  test)
    for arg in "$@"; do
      case "$arg" in
        -coverprofile=*)
          profile="${arg#-coverprofile=}"
          mkdir -p "$(dirname "$profile")"
          printf 'mode: set\n' > "$profile"
          exit 0
          ;;
      esac
    done
    if [ "$2" = "-json" ]; then
      printf '%s\n' '{"Time":"2026-01-01T00:00:00Z","Action":"pass","Package":"example.com/cleanrtest/tests","Test":"TestPass"}'
      printf '%s\n' '{"Time":"2026-01-01T00:00:00Z","Action":"pass","Package":"example.com/cleanrtest/tests"}'
      exit 0
    fi
    exit 0
    ;;
  tool)
    if [ "$2" = "cover" ]; then
      printf 'total: (statements) %s%%\n' "${FAKE_COVERAGE_TOTAL:-70.0}"
      exit 0
    fi
    ;;
esac

exit 0
`
}

func buildGolangCILintTarGz(t *testing.T) []byte {
	t.Helper()

	return buildBinaryTarGz(
		t,
		"golangci-lint",
		runtime.GOOS,
		runtime.GOARCH,
		"#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$GOLANGCI_LINT_LOG\"\nif [ -n \"$GOLANGCI_LINT_OUTPUT\" ]; then\n  printf '%s\\n' \"$GOLANGCI_LINT_OUTPUT\"\nfi\nexit \"${GOLANGCI_LINT_EXIT:-0}\"\n",
	)
}

func buildBinaryTarGz(t *testing.T, binaryName, goos, goarch, binaryContents string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	files := map[string]string{
		fmt.Sprintf("%s-bundle-%s-%s/LICENSE", binaryName, goos, goarch):        "license text",
		fmt.Sprintf("%s-bundle-%s-%s/%s", binaryName, goos, goarch, binaryName): binaryContents,
	}
	for name, contents := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(contents)),
		}); err != nil {
			t.Fatalf("write tar header for %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(contents)); err != nil {
			t.Fatalf("write tar contents for %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buf.Bytes()
}
