package tests

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

func TestDevtoolsPackageCodeGuardRunsExternalCLI(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/package-codeguard")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.PackageCodeGuard(context.Background(), devtools.CIOptions{BaseRef: "main", CodeGuardVersion: "v0.2.0"}); err != nil {
		t.Fatalf("package codeguard: %v\n%s", err, stdout.String())
	}

	logData, err := os.ReadFile(filepath.Join(repo, ".codeguard.log"))
	if err != nil {
		t.Fatalf("read codeguard log: %v", err)
	}
	logText := string(logData)
	for _, want := range []string{
		"scan -config .codeguard -mode diff -base-ref main -format text",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("expected %q in codeguard log:\n%s", want, logText)
		}
	}
}

func TestDevtoolsPackageCodeGuardAddsGoBinToPath(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/package-codeguard-path")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.PackageCodeGuard(context.Background(), devtools.CIOptions{BaseRef: "main", CodeGuardVersion: "v0.2.0"}); err != nil {
		t.Fatalf("package codeguard: %v\n%s", err, stdout.String())
	}

	logData, err := os.ReadFile(filepath.Join(repo, ".codeguard.log"))
	if err != nil {
		t.Fatalf("read codeguard log: %v", err)
	}
	goBin := filepath.Join(repo, ".fake-gopath", "bin")
	logText := string(logData)
	if !strings.Contains(logText, "PATH="+goBin) {
		t.Fatalf("expected codeguard PATH to include GOPATH/bin, got:\n%s", logText)
	}
}

func TestDevtoolsLegacyCodeGuardAPIsUsePackageCodeGuard(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/legacy-codeguard-apis")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")

	for _, tc := range []struct {
		name string
		run  func(devtools.Runner) error
	}{
		{
			name: "CodeGuard",
			run: func(r devtools.Runner) error {
				return r.CodeGuard(context.Background(), devtools.CIOptions{BaseRef: "main", CodeGuardVersion: "v0.2.0"})
			},
		},
		{
			name: "CISCC",
			run: func(r devtools.Runner) error {
				return r.CISCC(context.Background(), devtools.CIOptions{BaseRef: "main", CodeGuardVersion: "v0.2.0"})
			},
		},
		{
			name: "CIGolangCILint",
			run: func(r devtools.Runner) error {
				return r.CIGolangCILint(context.Background(), devtools.CIOptions{BaseRef: "main", CodeGuardVersion: "v0.2.0"})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			configureFakeCIToolchain(t, repo)
			runner := devtools.NewRunner(repo, &stdout, &stdout)
			if err := tc.run(runner); err != nil {
				t.Fatalf("%s: %v\n%s", tc.name, err, stdout.String())
			}

			logData, err := os.ReadFile(filepath.Join(repo, ".codeguard.log"))
			if err != nil {
				t.Fatalf("read codeguard log: %v", err)
			}
			if !strings.Contains(string(logData), "scan -config .codeguard -mode diff -base-ref main -format text") {
				t.Fatalf("expected package codeguard invocation in log, got:\n%s", string(logData))
			}
			if err := os.WriteFile(filepath.Join(repo, ".codeguard.log"), nil, 0o644); err != nil {
				t.Fatalf("reset codeguard log: %v", err)
			}
		})
	}
}

func TestDevtoolsCISkipCodeGuardSkipsBuiltInStep(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/skip-codeguard")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n\nconst skipCodeguard = true\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CI(context.Background(), devtools.CIOptions{BaseRef: "main", SkipCodeGuard: true}); err != nil {
		t.Fatalf("ci skip codeguard: %v\n%s", err, stdout.String())
	}

	if strings.Contains(stdout.String(), "==> codeguard") {
		t.Fatalf("expected ci output to skip codeguard step, got:\n%s", stdout.String())
	}
}

func TestDevtoolsCISemgrepFallsBackToPythonModule(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/python-semgrep")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")
	mustWriteFile(t, filepath.Join(repo, "tests", "app_test.go"), "package tests\n\nconst pythonSemgrep = true\n")

	var stdout bytes.Buffer
	configureFakeCIToolchainWithoutSemgrepBinary(t, repo)
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	if err := runner.CI(context.Background(), devtools.CIOptions{BaseRef: "main"}); err != nil {
		t.Fatalf("ci: %v\n%s", err, stdout.String())
	}

	semgrepArgs, err := os.ReadFile(filepath.Join(repo, ".semgrep.log"))
	if err != nil {
		t.Fatalf("read semgrep log: %v", err)
	}
	if !strings.Contains(string(semgrepArgs), "-m semgrep scan --config auto --baseline-commit") {
		t.Fatalf("expected python semgrep fallback args, got: %s", string(semgrepArgs))
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

func TestDevtoolsCIFailsWithMisplacedPackageTest(t *testing.T) {
	repo := initGitRepo(t, "main")
	writeCIBaseFiles(t, repo)
	gitCommitAll(t, repo, "base commit\n\nSigned-off-by: Test User <test@example.com>\n")
	gitCheckoutNewBranch(t, repo, "feature/misplaced-test")

	mustWriteFile(t, filepath.Join(repo, "cleanr", "app.go"), "package cleanr\n\nfunc Value() int { return 2 }\n")
	mustWriteFile(t, filepath.Join(repo, "cleanr", "app_test.go"), "package cleanr\n")

	var stdout bytes.Buffer
	configureFakeCIToolchain(t, repo)
	runner := devtools.NewRunner(repo, &stdout, &stdout)
	err := runner.CI(context.Background(), devtools.CIOptions{BaseRef: "main"})
	if err == nil || !strings.Contains(err.Error(), "Move these Go test files into tests/") {
		t.Fatalf("expected misplaced-test note, got %v\n%s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "must be moved under tests/") || !strings.Contains(stdout.String(), "cleanr/app_test.go") {
		t.Fatalf("expected misplaced-test output note, got:\n%s", stdout.String())
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
	mustWriteFile(t, filepath.Join(repo, ".codeguard", "codeguard.yaml"), "name: test\n")
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
		"python3": `#!/bin/sh
if [ "$1" = "-c" ] && [ "$2" = "import semgrep" ]; then
  exit 0
fi
if [ "$1" = "-m" ] && [ "$2" = "semgrep" ]; then
  shift 2
  printf '%s\n' "-m semgrep $*" >> "$SEMGREP_LOG"
  exit "${SEMGREP_EXIT:-0}"
fi
exit 1
`,
	})

	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))
	t.Setenv("FAKE_GOPATH", filepath.Join(repo, ".fake-gopath"))
	t.Setenv("FAKE_GO_LOG", filepath.Join(repo, ".fake-go.log"))
	t.Setenv("FAKE_COVERAGE_TOTAL", "70.0")
	t.Setenv("CODEGUARD_LOG", filepath.Join(repo, ".codeguard.log"))
	t.Setenv("SEMGREP_LOG", filepath.Join(repo, ".semgrep.log"))
	t.Setenv("GOCACHE", filepath.Join(repo, ".gocache"))
	worktreeDir, err := filepath.EvalSymlinks(repo)
	if err != nil {
		worktreeDir = repo
	}
	t.Setenv("WORKTREE_DIR", worktreeDir)
}

func configureFakeCIToolchainWithoutSemgrepBinary(t *testing.T, repo string) {
	t.Helper()

	fakeBin := scriptDir(t, map[string]string{
		"go": fakeGoScript(),
		"gofmt": `#!/bin/sh
if [ "$1" = "-l" ] && [ -n "$GOFMT_OUTPUT" ]; then
  printf '%s\n' "$GOFMT_OUTPUT"
fi
exit 0
`,
		"python3": `#!/bin/sh
if [ "$1" = "-c" ] && [ "$2" = "import semgrep" ]; then
  exit 0
fi
if [ "$1" = "-m" ] && [ "$2" = "semgrep" ]; then
  shift 2
  printf '%s\n' "-m semgrep $*" >> "$SEMGREP_LOG"
  exit "${SEMGREP_EXIT:-0}"
fi
exit 1
`,
	})

	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))
	t.Setenv("FAKE_GOPATH", filepath.Join(repo, ".fake-gopath"))
	t.Setenv("FAKE_GO_LOG", filepath.Join(repo, ".fake-go.log"))
	t.Setenv("FAKE_COVERAGE_TOTAL", "70.0")
	t.Setenv("CODEGUARD_LOG", filepath.Join(repo, ".codeguard.log"))
	t.Setenv("SEMGREP_LOG", filepath.Join(repo, ".semgrep.log"))
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
      github.com/devr-tools/codeguard/cmd/codeguard@*)
        cat > "$FAKE_GOPATH/bin/codeguard" <<'EOF'
#!/bin/sh
printf 'PATH=%s\n' "$PATH" >> "$CODEGUARD_LOG"
printf '%s\n' "$*" >> "$CODEGUARD_LOG"
if [ -n "$CODEGUARD_OUTPUT" ]; then
  printf '%s\n' "$CODEGUARD_OUTPUT"
fi
exit "${CODEGUARD_EXIT:-0}"
EOF
        chmod +x "$FAKE_GOPATH/bin/codeguard"
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
