package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/mcpserver"
	versionpkg "github.com/devr-tools/cleanr/internal/version"
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
	case "trends":
		return trendsCmd(args[1:], stdout, stderr)
	case "dataset":
		return datasetCmd(args[1:], stdout, stderr)
	case "plugins":
		return pluginsCmd(args[1:], stdout, stderr)
	case "snapshot":
		return snapshotCmd(args[1:], stdout, stderr)
	case "validate":
		return validateCmd(args[1:], stdout, stderr)
	case "init":
		return initCmd(args[1:], stdout, stderr)
	case "setup":
		return setupCmd(args[1:], stdout, stderr)
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
	trendFile := fs.String("trend-file", "", "Optional trend history file")
	replayArtifactPath := fs.String("replay-artifact", "", "Optional replay artifact file")
	buildID := fs.String("build-id", "", "Optional build identifier for trend history")
	trendLimit := fs.Int("trend-limit", 0, "Maximum number of trend history runs to keep")
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
	if *trendFile != "" {
		cfg.Reporting.TrendFile = *trendFile
	}
	if *replayArtifactPath != "" {
		cfg.Reporting.ReplayArtifactFile = *replayArtifactPath
	}
	if *buildID != "" {
		cfg.Reporting.BuildID = *buildID
	}
	if *trendLimit != 0 {
		cfg.Reporting.TrendLimit = *trendLimit
	}
	cfg.Suites.Drift.BaselineFile = resolveConfigRelativePath(resolvedConfigPath, cfg.Suites.Drift.BaselineFile)
	cfg.Reporting.TrendFile = resolveConfigRelativePath(resolvedConfigPath, cfg.Reporting.TrendFile)
	cfg.Reporting.ReplayArtifactFile = resolveConfigRelativePath(resolvedConfigPath, cfg.Reporting.ReplayArtifactFile)

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	report := cleanr.NewHTTPRunner(cfg).Run(ctx)
	if err := cleanr.AttachTrendHistory(&report, cfg.Reporting.TrendFile, cfg.Reporting.BuildID, cfg.Reporting.TrendLimit); err != nil {
		_, _ = fmt.Fprintf(stderr, "trend history error: %v\n", err)
		return 2
	}
	cleanr.EvaluateTrendGates(&report, cfg.Reporting.TrendGates)
	replayArtifact := cleanr.ReplayArtifact{}
	hasReplayArtifact := false
	if needsReplayArtifact(cfg) {
		replayArtifact = cleanr.BuildReplayArtifact(report)
		hasReplayArtifact = true
	}
	if strings.TrimSpace(cfg.Reporting.ReplayArtifactFile) != "" {
		if err := cleanr.WriteReplayArtifactFile(cfg.Reporting.ReplayArtifactFile, replayArtifact); err != nil {
			_, _ = fmt.Fprintf(stderr, "replay artifact error: %v\n", err)
			return 2
		}
	}
	var attestation *cleanr.ReleaseGateAttestation
	if cfg.Governance.Attestation.Enabled {
		rawKey := os.Getenv(cfg.Governance.Attestation.KeyEnv)
		if !hasReplayArtifact {
			replayArtifact = cleanr.BuildReplayArtifact(report)
		}
		builtAttestation, err := cleanr.BuildReleaseGateAttestation(report, replayArtifact, rawKey, cfg.Governance.Attestation.KeyID)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "attestation error: %v\n", err)
			return 2
		}
		attestation = &builtAttestation
		outputPath := resolveConfigRelativePath(resolvedConfigPath, cfg.Governance.Attestation.Output)
		if err := cleanr.WriteReleaseGateAttestationFile(outputPath, builtAttestation); err != nil {
			_, _ = fmt.Fprintf(stderr, "attestation error: %v\n", err)
			return 2
		}
	}
	if hasConfiguredIntegrations(cfg.Integrations) {
		integrationReport := cleanr.EnsureIntegrationReport(&report)
		integrationReport.TrendSources = cleanr.CompareTrendSources(ctx, cfg.Integrations, report, resolvedConfigPath)
		if hasReplayArtifact {
			integrationReport.ResultSinks = cleanr.PublishResultSinks(ctx, cfg.Integrations, report, &replayArtifact, attestation)
		} else {
			integrationReport.ResultSinks = cleanr.PublishResultSinks(ctx, cfg.Integrations, report, nil, attestation)
		}
		integrationReport.Summaries = cleanr.WriteSummaries(cfg.Integrations, report, resolvedConfigPath)
		printIntegrationWarnings(stderr, integrationReport)
	}

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

func snapshotCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to cleanr config")
	output := fs.String("output", "", "Path to write snapshot baseline")
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

	outputPath := strings.TrimSpace(*output)
	if outputPath == "" {
		outputPath = strings.TrimSpace(cfg.Suites.Drift.BaselineFile)
	}
	if outputPath == "" {
		outputPath = "cleanr.snapshots.yaml"
	}
	outputPath = resolveConfigRelativePath(resolvedConfigPath, outputPath)
	cfg.Suites.Drift.BaselineFile = outputPath

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	target := cleanr.NewTarget(cfg.Target, &http.Client{Timeout: cfg.Target.Timeout()})
	snapshot, err := cleanr.CaptureSnapshots(ctx, cfg, target)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "snapshot error: %v\n", err)
		return 2
	}
	if err := cleanr.WriteSnapshotFile(outputPath, snapshot); err != nil {
		_, _ = fmt.Fprintf(stderr, "write snapshot: %v\n", err)
		return 2
	}
	_, _ = fmt.Fprintf(stdout, "wrote %d snapshots to %s\n", len(snapshot.Scenarios), outputPath)
	return 0
}

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

func usage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage: cleanr <run|trends|dataset|plugins|snapshot|validate|init|setup|mcp|version> [flags]")
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

func resolveConfigRelativePath(configPath, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(filepath.Dir(configPath), path)
}

func resolveTrendPath(configPath, explicitTrendPath string) (string, error) {
	if strings.TrimSpace(explicitTrendPath) != "" {
		return explicitTrendPath, nil
	}
	resolvedConfigPath, err := resolveConfigPath(configPath)
	if err != nil {
		return "", err
	}
	cfg, err := cleanr.LoadConfigFile(resolvedConfigPath)
	if err != nil {
		return "", err
	}
	trendPath := resolveConfigRelativePath(resolvedConfigPath, cfg.Reporting.TrendFile)
	if strings.TrimSpace(trendPath) == "" {
		return "", fmt.Errorf("no trend file configured; set reporting.trend_file or pass -trend-file")
	}
	return trendPath, nil
}

func needsReplayArtifact(cfg cleanr.Config) bool {
	if strings.TrimSpace(cfg.Reporting.ReplayArtifactFile) != "" || cfg.Governance.Attestation.Enabled {
		return true
	}
	for _, sink := range cfg.Integrations.ResultSinks {
		if sink.IncludeReplay {
			return true
		}
	}
	return false
}

func hasConfiguredIntegrations(cfg cleanr.IntegrationsConfig) bool {
	return len(cfg.ResultSinks) > 0 || len(cfg.TrendSources) > 0 || len(cfg.Summaries) > 0
}

func printIntegrationWarnings(stderr io.Writer, report *cleanr.IntegrationReport) {
	if report == nil {
		return
	}
	for _, item := range report.TrendSources {
		if item.Status == "error" && strings.TrimSpace(item.Message) != "" {
			_, _ = fmt.Fprintf(stderr, "integration warning: trend source %s: %s\n", item.Name, item.Message)
		}
	}
	for _, item := range report.ResultSinks {
		if !item.Published && strings.TrimSpace(item.Message) != "" {
			_, _ = fmt.Fprintf(stderr, "integration warning: result sink %s: %s\n", item.Name, item.Message)
		}
	}
	for _, item := range report.Summaries {
		if !item.Written && strings.TrimSpace(item.Message) != "" {
			_, _ = fmt.Fprintf(stderr, "integration warning: summary %s: %s\n", item.Name, item.Message)
		}
	}
}

func writeJSON(w io.Writer, value any) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return 2
	}
	return 0
}
