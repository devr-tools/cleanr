package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/devr-tools/cleanr/internal/devtools"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}

	wd, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "getwd: %v\n", err)
		return 2
	}
	runner := devtools.NewRunner(wd, os.Stdout, os.Stderr)
	ctx := context.Background()

	switch args[0] {
	case "check":
		return runRunnerCommand("check", func() error { return runner.Check(ctx) })
	case "ci":
		return runCIDevCommand(ctx, runner, args[1:])
	case "ci-package-codeguard":
		return runCIPackageCodeGuardDevCommand(ctx, runner, args[1:])
	case "fmt":
		return runRunnerCommand("fmt", func() error { return runner.Format(ctx) })
	case "fmt-check":
		return runRunnerCommand("fmt-check", func() error { return runner.FormatCheck(ctx) })
	case "lint":
		return runRunnerCommand("lint", func() error { return runner.Lint(ctx) })
	case "test":
		return runRunnerCommand("test", func() error { return runner.Test(ctx) })
	case "test-review-ui":
		return runRunnerCommand("test-review-ui", func() error { return runner.TestReviewUI(ctx) })
	case "preview-review-ui":
		return runRunnerCommand("preview-review-ui", func() error { return runner.ReviewUIPreview(ctx) })
	case "gofiles":
		return runGoFilesCommand(runner)
	case "build":
		return runBuildDevCommand(ctx, runner, args[1:])
	case "release":
		return runReleaseDevCommand(ctx, runner, args[1:])
	case "homebrew-formula":
		return runHomebrewFormulaDevCommand(runner, args[1:])
	case "report":
		return runReportDevCommand(ctx, runner, args[1:])
	default:
		usage(os.Stderr)
		return 2
	}
}

func runRunnerCommand(name string, fn func() error) int {
	if err := fn(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s failed: %v\n", name, err)
		return 1
	}
	return 0
}

func runCIDevCommand(ctx context.Context, runner devtools.Runner, args []string) int {
	fs := flag.NewFlagSet("ci", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	baseRef := fs.String("base-ref", "", "Base Git ref to diff against, for example origin/main")
	buildOutput := fs.String("build-output", "dist/cleanr-linux-amd64", "Output path for the Linux amd64 snapshot build")
	codeGuardVersion := fs.String("codeguard-version", "", "codeguard version to install")
	govulncheckMode := fs.String("govulncheck-mode", "", "govulncheck mode: required or off")
	govulncheckVersion := fs.String("govulncheck-version", "", "govulncheck version to install")
	minCoverage := fs.Float64("min-internal-coverage", 0, "Minimum internal coverage percentage")
	semgrepCommand := fs.String("semgrep-command", "", "Semgrep executable name or path")
	skipCodeGuard := fs.Bool("skip-codeguard", false, "Skip the built-in cleanr codeguard step")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return runRunnerCommand("ci", func() error {
		return runner.CI(ctx, devtools.CIOptions{
			BaseRef:             *baseRef,
			BuildOutput:         *buildOutput,
			CodeGuardVersion:    *codeGuardVersion,
			GovulncheckMode:     *govulncheckMode,
			GovulncheckVersion:  *govulncheckVersion,
			MinInternalCoverage: *minCoverage,
			SemgrepCommand:      *semgrepCommand,
			SkipCodeGuard:       *skipCodeGuard,
		})
	})
}

func runCIPackageCodeGuardDevCommand(ctx context.Context, runner devtools.Runner, args []string) int {
	fs := flag.NewFlagSet("ci-package-codeguard", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	baseRef := fs.String("base-ref", "", "Base Git ref to diff against, for example origin/main")
	codeGuardVersion := fs.String("codeguard-version", "", "codeguard version to install")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return runRunnerCommand("ci-package-codeguard", func() error {
		return runner.PackageCodeGuard(ctx, devtools.CIOptions{
			BaseRef:          *baseRef,
			CodeGuardVersion: *codeGuardVersion,
		})
	})
}

func runGoFilesCommand(runner devtools.Runner) int {
	if err := runner.CheckGoFiles(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "gofiles failed: %v\n", err)
		return 1
	}
	if err := runner.ListGoFiles(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "gofiles failed: %v\n", err)
		return 1
	}
	return 0
}

func runBuildDevCommand(ctx context.Context, runner devtools.Runner, args []string) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	output := fs.String("output", "dist/cleanr", "Output binary path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return runRunnerCommand("build", func() error { return runner.Build(ctx, *output) })
}

func runReleaseDevCommand(ctx context.Context, runner devtools.Runner, args []string) int {
	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	version := fs.String("version", "dev", "Version for release artifacts")
	output := fs.String("output", "dist/releases", "Release output directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return runRunnerCommand("release", func() error {
		return runner.Release(ctx, devtools.ReleaseOptions{
			Version: *version,
			Output:  *output,
		})
	})
}

func runHomebrewFormulaDevCommand(runner devtools.Runner, args []string) int {
	fs := flag.NewFlagSet("homebrew-formula", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	version := fs.String("version", "", "Release tag version, for example v1.2.3")
	repository := fs.String("repository", "", "GitHub repository in owner/name form")
	sourceSHA256 := fs.String("source-sha256", "", "SHA256 for the tagged source tarball used by Homebrew")
	license := fs.String("license", "", "Optional SPDX license identifier for the formula")
	output := fs.String("output", "", "Output path for the generated formula")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return runRunnerCommand("homebrew-formula", func() error {
		return runner.HomebrewFormula(devtools.HomebrewFormulaOptions{
			Version:      *version,
			Repository:   *repository,
			SourceSHA256: *sourceSHA256,
			License:      *license,
			Output:       *output,
		})
	})
}

func runReportDevCommand(ctx context.Context, runner devtools.Runner, args []string) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	input := fs.String("input", "", "Optional path to a JSON cleanr report to render")
	format := fs.String("format", "text", "Report format: text, json, junit, sarif, agent, or html")
	preset := fs.String("preset", "fail", "Built-in preview preset when -input is omitted: fail or pass")
	output := fs.String("output", "", "Optional output file for the rendered report")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return runRunnerCommand("report", func() error {
		return runner.Report(devtools.ReportOptions{
			Input:  *input,
			Format: *format,
			Preset: *preset,
			Output: *output,
		})
	})
}

func usage(w *os.File) {
	_, _ = fmt.Fprintln(w, "usage: cleanr-dev <check|ci|ci-package-codeguard|fmt|fmt-check|lint|test|test-review-ui|preview-review-ui|gofiles|build|release|homebrew-formula|report>")
}
