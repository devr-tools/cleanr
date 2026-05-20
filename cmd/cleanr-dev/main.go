package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"cleanr/internal/devtools"
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
		if err := runner.Check(ctx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "check failed: %v\n", err)
			return 1
		}
		return 0
	case "fmt":
		if err := runner.Format(ctx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "fmt failed: %v\n", err)
			return 1
		}
		return 0
	case "fmt-check":
		if err := runner.FormatCheck(ctx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "fmt-check failed: %v\n", err)
			return 1
		}
		return 0
	case "lint":
		if err := runner.Lint(ctx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "lint failed: %v\n", err)
			return 1
		}
		return 0
	case "test":
		if err := runner.Test(ctx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "test failed: %v\n", err)
			return 1
		}
		return 0
	case "gofiles":
		if err := runner.CheckGoFiles(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "gofiles failed: %v\n", err)
			return 1
		}
		if err := runner.ListGoFiles(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "gofiles failed: %v\n", err)
			return 1
		}
		return 0
	case "build":
		fs := flag.NewFlagSet("build", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		output := fs.String("output", "dist/cleanr", "Output binary path")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if err := runner.Build(ctx, *output); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
			return 1
		}
		return 0
	case "release":
		fs := flag.NewFlagSet("release", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		version := fs.String("version", "dev", "Version for release artifacts")
		output := fs.String("output", "dist/releases", "Release output directory")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if err := runner.Release(ctx, devtools.ReleaseOptions{
			Version: *version,
			Output:  *output,
		}); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "release failed: %v\n", err)
			return 1
		}
		return 0
	case "homebrew-formula":
		fs := flag.NewFlagSet("homebrew-formula", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		version := fs.String("version", "", "Release tag version, for example v1.2.3")
		checksums := fs.String("checksums", "", "Path to the SHA256SUMS file for the release")
		repository := fs.String("repository", "", "GitHub repository in owner/name form")
		output := fs.String("output", "", "Output path for the generated formula")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if err := runner.HomebrewFormula(devtools.HomebrewFormulaOptions{
			Version:    *version,
			Checksums:  *checksums,
			Repository: *repository,
			Output:     *output,
		}); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "homebrew-formula failed: %v\n", err)
			return 1
		}
		return 0
	case "report":
		fs := flag.NewFlagSet("report", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		input := fs.String("input", "", "Optional path to a JSON cleanr report to render")
		format := fs.String("format", "text", "Report format: text, json, junit")
		preset := fs.String("preset", "fail", "Built-in preview preset when -input is omitted: fail or pass")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if err := runner.Report(devtools.ReportOptions{
			Input:  *input,
			Format: *format,
			Preset: *preset,
		}); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "report failed: %v\n", err)
			return 1
		}
		return 0
	default:
		usage(os.Stderr)
		return 2
	}
}

func usage(w *os.File) {
	_, _ = fmt.Fprintln(w, "usage: cleanr-dev <check|fmt|fmt-check|lint|test|gofiles|build|release|homebrew-formula|report>")
}
