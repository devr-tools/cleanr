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
	trendFile := fs.String("trend-file", "", "Path to trend history file")
	format := fs.String("format", "text", "Output format: text or json")
	output := fs.String("output", "", "Optional output file")
	window := fs.Int("window", 0, "Number of recent retained runs to summarize")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *window < 0 {
		_, _ = fmt.Fprintln(stderr, "trends error: window must be >= 0")
		return 2
	}

	trendPath, err := resolveTrendPath(*configPath, *trendFile)
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

func datasetCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "dataset error: expected one of export or import")
		return 2
	}
	switch args[0] {
	case "export":
		return datasetExportCmd(args[1:], stdout, stderr)
	case "import":
		return datasetImportCmd(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "dataset error: unsupported subcommand %s\n", args[0])
		return 2
	}
}

func datasetExportCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dataset export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to cleanr config")
	replayPath := fs.String("replay-artifact", "", "Path to replay artifact file")
	output := fs.String("output", "cleanr.dataset.yaml", "Path to write the exported scenario dataset")
	includeAll := fs.Bool("all", false, "Include all scenarios instead of only reviewed replay failures")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedConfigPath, err := resolveConfigPath(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset export error: %v\n", err)
		return 2
	}
	cfg, err := cleanr.LoadConfigFile(resolvedConfigPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset export error: %v\n", err)
		return 2
	}
	artifactPath := strings.TrimSpace(*replayPath)
	if artifactPath == "" {
		artifactPath = cfg.Reporting.ReplayArtifactFile
	}
	artifactPath = resolveConfigRelativePath(resolvedConfigPath, artifactPath)
	if strings.TrimSpace(artifactPath) == "" {
		_, _ = fmt.Fprintln(stderr, "dataset export error: no replay artifact configured; pass -replay-artifact or set reporting.replay_artifact_file")
		return 2
	}
	artifact, err := cleanr.LoadReplayArtifactFile(artifactPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset export error: %v\n", err)
		return 2
	}
	dataset := cleanr.ExportScenarioDataset(cfg, artifact, *includeAll)
	outputPath := resolveConfigRelativePath(resolvedConfigPath, *output)
	if err := cleanr.WriteScenarioDatasetFile(outputPath, dataset); err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset export error: %v\n", err)
		return 2
	}
	_, _ = fmt.Fprintf(stdout, "wrote %d scenarios to %s\n", len(dataset.Scenarios), outputPath)
	return 0
}

func datasetImportCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dataset import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	input := fs.String("input", "", "Path to scenario dataset file")
	baseConfig := fs.String("base-config", "", "Optional base cleanr config to merge into")
	output := fs.String("output", "cleanr.imported.yaml", "Path to write the merged cleanr config")
	approveGenerated := fs.Bool("approve-generated", false, "Allow importing generated datasets that require explicit review")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*input) == "" {
		_, _ = fmt.Fprintln(stderr, "dataset import error: -input is required")
		return 2
	}

	dataset, err := cleanr.LoadScenarioDatasetFile(*input)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset import error: %v\n", err)
		return 2
	}
	if len(dataset.Scenarios) == 0 {
		_, _ = fmt.Fprintln(stderr, "dataset import error: dataset contains no scenarios")
		return 2
	}
	if dataset.Source == "cleanr-generation" && dataset.ReviewRequired && !*approveGenerated {
		_, _ = fmt.Fprintln(stderr, "dataset import error: generated dataset requires explicit review; rerun with -approve-generated after review")
		return 2
	}

	cfg := cleanr.ExampleConfig()
	basePath := strings.TrimSpace(*baseConfig)
	if basePath != "" {
		cfg, err = cleanr.LoadConfigFile(basePath)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "dataset import error: %v\n", err)
			return 2
		}
	}
	cfg = cleanr.MergeDatasetIntoConfig(cfg, dataset)
	if err := cleanr.WriteConfigFile(*output, cfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset import error: %v\n", err)
		return 2
	}
	_, _ = fmt.Fprintf(stdout, "wrote merged config with %d scenarios to %s\n", len(cfg.Scenarios), *output)
	return 0
}

func pluginsCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("plugins", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to cleanr config")
	format := fs.String("format", "text", "Output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedConfigPath, err := resolveConfigPath(*configPath)
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
