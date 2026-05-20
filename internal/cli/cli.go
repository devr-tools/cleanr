package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"cleanr/cleanr"
)

const version = "0.1.0"

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
	configPath := fs.String("config", "cleanr.json", "Path to cleanr config")
	format := fs.String("format", "", "Report format: text, json, junit")
	output := fs.String("output", "", "Optional output file")
	timeout := fs.Duration("timeout", 0, "Overall execution timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, err := cleanr.LoadConfigFile(*configPath)
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
	configPath := fs.String("config", "cleanr.json", "Path to cleanr config")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := cleanr.LoadConfigFile(*configPath)
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
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "encode example config: %v\n", err)
		return 2
	}
	if err := os.WriteFile(*path, append(data, '\n'), 0o644); err != nil {
		_, _ = fmt.Fprintf(stderr, "write example config: %v\n", err)
		return 2
	}
	_, _ = fmt.Fprintf(stdout, "wrote example config to %s at %s\n", *path, time.Now().Format(time.RFC3339))
	return 0
}

func usage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage: cleanr <run|validate|init|version> [flags]")
}
