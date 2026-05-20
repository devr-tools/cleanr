package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cleanr/cleanr"
	"cleanr/internal/mcpserver"
	versionpkg "cleanr/internal/version"
)

var version = versionpkg.Number

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}

	switch args[0] {
	case "run":
		return runCmd(args[1:], stdout, stderr)
	case "validate":
		return validateCmd(args[1:], stdout, stderr)
	case "init":
		return initCmd(args[1:], stdout, stderr)
	case "mcp":
		return mcpCmd(args[1:], stdout, stderr)
	case "version":
		_, _ = fmt.Fprintf(stdout, "cleanr %s\n", version)
		return 0
	default:
		usage(stderr)
		return 2
	}
}

func runCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to cleanr config")
	format := fs.String("format", "", "Report format: text, json, junit")
	output := fs.String("output", "", "Optional output file")
	timeout := fs.Duration("timeout", 0, "Overall execution timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedConfigPath, err := resolveConfigPath(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "config error: %v\n", err)
		return 2
	}

	cfg, err := cleanr.LoadConfigFile(resolvedConfigPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "config error: %v\n", err)
		return 2
	}
	if *format != "" {
		cfg.Reporting.Format = *format
	}
	if *output != "" {
		cfg.Reporting.Output = *output
	}

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	report := cleanr.NewHTTPRunner(cfg).Run(ctx)

	dest := stdout
	if cfg.Reporting.Output != "" {
		f, err := os.Create(cfg.Reporting.Output)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "open report output: %v\n", err)
			return 2
		}
		defer f.Close()
		dest = f
	}
	if err := cleanr.WriteReport(dest, report, cfg.Reporting.Format); err != nil {
		_, _ = fmt.Fprintf(stderr, "write report: %v\n", err)
		return 2
	}
	if cfg.Reporting.Output != "" && cfg.Reporting.Format != "text" {
		_, _ = fmt.Fprintf(stdout, "wrote %s report to %s\n", cfg.Reporting.Format, cfg.Reporting.Output)
	}
	if report.Passed {
		return 0
	}
	return 1
}

func validateCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to cleanr config")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedConfigPath, err := resolveConfigPath(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "invalid: %v\n", err)
		return 2
	}

	cfg, err := cleanr.LoadConfigFile(resolvedConfigPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "invalid: %v\n", err)
		return 2
	}
	_, _ = fmt.Fprintf(stdout, "valid config for %s with %d scenarios\n", cfg.Target.Name, len(cfg.Scenarios))
	return 0
}

func initCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("output", "cleanr.json", "Path to write example config")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg := cleanr.ExampleConfig()
	if err := cleanr.WriteConfigFile(*path, cfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "write example config: %v\n", err)
		return 2
	}
	_, _ = fmt.Fprintf(stdout, "wrote example config to %s at %s\n", *path, time.Now().Format(time.RFC3339))
	return 0
}

func mcpCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := mcpserver.New().Serve(context.Background(), os.Stdin, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "mcp server error: %v\n", err)
		return 2
	}
	return 0
}

func usage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage: cleanr <run|validate|init|mcp|version> [flags]")
}

func resolveConfigPath(configPath string) (string, error) {
	if configPath != "" {
		return configPath, nil
	}

	candidates := []string{"cleanr.json", "cleanr.yaml", "cleanr.yml"}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no config file found; expected one of %s in %s", joinCandidates(candidates), mustGetwd())
}

func joinCandidates(paths []string) string {
	quoted := make([]string, 0, len(paths))
	for _, path := range paths {
		quoted = append(quoted, filepath.Base(path))
	}
	return strings.Join(quoted, ", ")
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
