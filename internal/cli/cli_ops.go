package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/mcpserver"
)

func trendsCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("trends", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to cleanr config")
	profile := fs.String("profile", "", "Optional staged config profile: pr, main, or release")
	trendFile := fs.String("trend-file", "", "Path to trend history file")
	format := fs.String("format", "text", "Output format: text, json, or html")
	output := fs.String("output", "", "Optional output file")
	window := fs.Int("window", 0, "Number of recent retained runs to summarize")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *window < 0 {
		_, _ = fmt.Fprintln(stderr, "trends error: window must be >= 0")
		return 2
	}

	trendPath, err := resolveTrendPath(*configPath, *profile, *trendFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "trends error: %v\n", err)
		return 2
	}

	analysis, err := cleanr.AnalyzeTrendHistoryFile(trendPath, *window)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "trends error: %v\n", err)
		return 2
	}

	dest := stdout
	if strings.TrimSpace(*output) != "" {
		f, err := os.Create(*output)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "open trends output: %v\n", err)
			return 2
		}
		defer f.Close()
		dest = f
	}
	if err := cleanr.WriteTrendAnalysis(dest, analysis, *format); err != nil {
		_, _ = fmt.Fprintf(stderr, "write trends: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*output) != "" && strings.ToLower(strings.TrimSpace(*format)) != "text" {
		_, _ = fmt.Fprintf(stdout, "wrote %s trends to %s\n", *format, *output)
	}
	return 0
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func pluginsCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("plugins", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to cleanr config")
	profile := fs.String("profile", "", "Optional staged config profile: pr, main, or release")
	format := fs.String("format", "text", "Output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedConfigPath, err := resolveConfigPath(*configPath, *profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "plugins error: %v\n", err)
		return 2
	}
	cfg, err := cleanr.LoadConfigFile(resolvedConfigPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "plugins error: %v\n", err)
		return 2
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "", "text":
		if len(cfg.ResolvedPlugins) == 0 {
			_, _ = fmt.Fprintln(stdout, "No plugins configured.")
			return 0
		}
		for _, plugin := range cfg.ResolvedPlugins {
			_, _ = fmt.Fprintf(stdout, "%s", plugin.Name)
			if plugin.Version != "" {
				_, _ = fmt.Fprintf(stdout, " (%s)", plugin.Version)
			}
			_, _ = fmt.Fprintln(stdout)
			if len(plugin.PolicyPacks) > 0 {
				_, _ = fmt.Fprintf(stdout, "  policy_packs: %s\n", strings.Join(plugin.PolicyPacks, ", "))
			}
			for _, suite := range plugin.Suites {
				_, _ = fmt.Fprintf(stdout, "  suite: %s -> %s\n", suite.Name, suite.Command)
			}
			for _, adapter := range plugin.StateAdapters {
				_, _ = fmt.Fprintf(stdout, "  state_adapter: %s -> %s\n", adapter.Name, adapter.Command)
			}
			for _, probe := range plugin.Probes {
				_, _ = fmt.Fprintf(stdout, "  probe: %s -> %s\n", probe.Name, probe.Command)
			}
		}
		return 0
	case "json":
		return writeJSON(stdout, cfg.ResolvedPlugins)
	default:
		_, _ = fmt.Fprintf(stderr, "plugins error: unsupported format %s\n", *format)
		return 2
	}
}

func validateCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to cleanr config")
	profile := fs.String("profile", "", "Optional staged config profile: pr, main, or release")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedConfigPath, err := resolveConfigPath(*configPath, *profile)
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
