package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

type runOptions struct {
	configPath         string
	profile            string
	format             string
	output             string
	trendFile          string
	replayArtifactPath string
	buildID            string
	trendLimit         int
	timeout            time.Duration
	githubOutputs      bool
	githubPRComment    bool
	githubPRNumber     int
	buildkite          buildkiteOptions
}

func runCmd(args []string, stdout, stderr io.Writer) int {
	opts, err := parseRunOptions(args, stderr)
	if err != nil {
		return 2
	}

	resolvedConfigPath, err := resolveConfigPath(opts.configPath, opts.profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "config error: %v\n", err)
		return 2
	}

	cfg, err := cleanr.LoadConfigFile(resolvedConfigPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "config error: %v\n", err)
		return 2
	}
	applyRunOptions(&cfg, opts)
	if len(cfg.Scenarios) == 0 {
		_, _ = fmt.Fprintln(stderr, "run error: config contains no scenarios; generate or import scenarios before running tests")
		return 2
	}
	resolveRunPaths(&cfg, resolvedConfigPath)

	ctx, cancel := runContext(opts.timeout)
	defer cancel()
	report := cleanr.NewHTTPRunner(cfg).Run(ctx)
	if err := cleanr.AttachTrendHistory(&report, cfg.Reporting.TrendFile, cfg.Reporting.BuildID, cfg.Reporting.TrendLimit); err != nil {
		_, _ = fmt.Fprintf(stderr, "trend history error: %v\n", err)
		return 2
	}
	cleanr.EvaluateTrendGates(&report, cfg.Reporting.TrendGates)
	replayArtifact, hasReplayArtifact, err := buildAndWriteReplayArtifact(cfg, report, stderr)
	if err != nil {
		return 2
	}
	attestation, err := buildAndWriteAttestation(cfg, report, replayArtifact, hasReplayArtifact, resolvedConfigPath, stderr)
	if err != nil {
		return 2
	}
	publishIntegrations(ctx, cfg, &report, replayArtifact, hasReplayArtifact, attestation, resolvedConfigPath, stderr)

	if err := writeRunReport(stdout, stderr, cfg.Reporting, report); err != nil {
		_, _ = fmt.Fprintf(stderr, "write report: %v\n", err)
		return 2
	}
	if opts.githubOutputs {
		if err := writeRunGitHubOutputs(report); err != nil {
			_, _ = fmt.Fprintf(stderr, "github output warning: %v\n", err)
		}
	}
	if opts.githubPRComment {
		number, err := postRunGitHubPRComment(report, opts.githubPRNumber)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "github pr comment error: %v\n", err)
			return 2
		}
		_, _ = fmt.Fprintf(stdout, "posted GitHub PR comment to #%d\n", number)
	}
	if err := maybeWriteBuildkiteRunOutputs(opts.buildkite, cfg.Reporting, report, resolvedConfigPath); err != nil {
		_, _ = fmt.Fprintf(stderr, "buildkite warning: %v\n", err)
	}
	if report.Passed {
		return 0
	}
	return 1
}

func parseRunOptions(args []string, stderr io.Writer) (runOptions, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := runOptions{}
	fs.StringVar(&opts.configPath, "config", "", "Path to cleanr config")
	fs.StringVar(&opts.profile, "profile", "", "Optional staged config profile: pr, main, or release")
	fs.StringVar(&opts.format, "format", "", "Report format: text, json, junit")
	fs.StringVar(&opts.output, "output", "", "Optional output file")
	fs.StringVar(&opts.trendFile, "trend-file", "", "Optional trend history file")
	fs.StringVar(&opts.replayArtifactPath, "replay-artifact", "", "Optional replay artifact file")
	fs.StringVar(&opts.buildID, "build-id", "", "Optional build identifier for trend history")
	fs.IntVar(&opts.trendLimit, "trend-limit", 0, "Maximum number of trend history runs to keep")
	fs.DurationVar(&opts.timeout, "timeout", 0, "Overall execution timeout")
	fs.BoolVar(&opts.githubOutputs, "github-outputs", false, "Write PR-oriented run metrics to $GITHUB_OUTPUT and $GITHUB_STEP_SUMMARY when available")
	fs.BoolVar(&opts.githubPRComment, "github-pr-comment", false, "Post the generated PR review body to GitHub using gh")
	fs.IntVar(&opts.githubPRNumber, "github-pr-number", 0, "GitHub pull request number to comment on; defaults to GitHub Actions pull_request context when available")
	fs.BoolVar(&opts.buildkite.Meta, "buildkite-meta", false, "Write run metrics to Buildkite metadata when buildkite-agent is available")
	fs.BoolVar(&opts.buildkite.Annotation, "buildkite-annotation", false, "Write a Buildkite annotation when the run fails and buildkite-agent is available")
	return opts, fs.Parse(args)
}

func applyRunOptions(cfg *cleanr.Config, opts runOptions) {
	if opts.format != "" {
		cfg.Reporting.Format = opts.format
	}
	if opts.output != "" {
		cfg.Reporting.Output = opts.output
	}
	if opts.trendFile != "" {
		cfg.Reporting.TrendFile = opts.trendFile
	}
	if opts.replayArtifactPath != "" {
		cfg.Reporting.ReplayArtifactFile = opts.replayArtifactPath
	}
	if opts.buildID != "" {
		cfg.Reporting.BuildID = opts.buildID
	}
	if opts.trendLimit != 0 {
		cfg.Reporting.TrendLimit = opts.trendLimit
	}
}

func resolveRunPaths(cfg *cleanr.Config, resolvedConfigPath string) {
	cfg.Suites.Drift.BaselineFile = resolveConfigRelativePath(resolvedConfigPath, cfg.Suites.Drift.BaselineFile)
	cfg.Reporting.TrendFile = resolveConfigRelativePath(resolvedConfigPath, cfg.Reporting.TrendFile)
	cfg.Reporting.ReplayArtifactFile = resolveConfigRelativePath(resolvedConfigPath, cfg.Reporting.ReplayArtifactFile)
}

func runContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), timeout)
}

func buildAndWriteReplayArtifact(cfg cleanr.Config, report cleanr.Report, stderr io.Writer) (cleanr.ReplayArtifact, bool, error) {
	replayArtifact := cleanr.ReplayArtifact{}
	hasReplayArtifact := false
	if needsReplayArtifact(cfg) {
		replayArtifact = cleanr.BuildReplayArtifact(report)
		hasReplayArtifact = true
	}
	if strings.TrimSpace(cfg.Reporting.ReplayArtifactFile) == "" {
		return replayArtifact, hasReplayArtifact, nil
	}
	if err := cleanr.WriteReplayArtifactFile(cfg.Reporting.ReplayArtifactFile, replayArtifact); err != nil {
		_, _ = fmt.Fprintf(stderr, "replay artifact error: %v\n", err)
		return cleanr.ReplayArtifact{}, false, err
	}
	return replayArtifact, hasReplayArtifact, nil
}

func buildAndWriteAttestation(cfg cleanr.Config, report cleanr.Report, replayArtifact cleanr.ReplayArtifact, hasReplayArtifact bool, resolvedConfigPath string, stderr io.Writer) (*cleanr.ReleaseGateAttestation, error) {
	if !cfg.Governance.Attestation.Enabled {
		return nil, nil
	}
	rawKey := os.Getenv(cfg.Governance.Attestation.KeyEnv)
	if !hasReplayArtifact {
		replayArtifact = cleanr.BuildReplayArtifact(report)
	}
	builtAttestation, err := cleanr.BuildReleaseGateAttestation(report, replayArtifact, rawKey, cfg.Governance.Attestation.KeyID)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "attestation error: %v\n", err)
		return nil, err
	}
	outputPath := resolveConfigRelativePath(resolvedConfigPath, cfg.Governance.Attestation.Output)
	if err := cleanr.WriteReleaseGateAttestationFile(outputPath, builtAttestation); err != nil {
		_, _ = fmt.Fprintf(stderr, "attestation error: %v\n", err)
		return nil, err
	}
	return &builtAttestation, nil
}

func publishIntegrations(ctx context.Context, cfg cleanr.Config, report *cleanr.Report, replayArtifact cleanr.ReplayArtifact, hasReplayArtifact bool, attestation *cleanr.ReleaseGateAttestation, resolvedConfigPath string, stderr io.Writer) {
	if !hasConfiguredIntegrations(cfg.Integrations) {
		return
	}
	integrationReport := cleanr.EnsureIntegrationReport(report)
	integrationReport.TrendSources = cleanr.CompareTrendSources(ctx, cfg.Integrations, *report, resolvedConfigPath)
	if hasReplayArtifact {
		integrationReport.ResultSinks = cleanr.PublishResultSinks(ctx, cfg.Integrations, *report, &replayArtifact, attestation)
	} else {
		integrationReport.ResultSinks = cleanr.PublishResultSinks(ctx, cfg.Integrations, *report, nil, attestation)
	}
	integrationReport.Summaries = cleanr.WriteSummaries(cfg.Integrations, *report, resolvedConfigPath)
	printIntegrationWarnings(stderr, integrationReport)
}

func writeRunReport(stdout, stderr io.Writer, reporting cleanr.ReportingConfig, report cleanr.Report) error {
	dest := stdout
	if reporting.Output != "" {
		f, err := os.Create(reporting.Output)
		if err != nil {
			return fmt.Errorf("open report output: %w", err)
		}
		defer f.Close()
		dest = f
	}
	if err := cleanr.WriteReport(dest, report, reporting.Format); err != nil {
		return err
	}
	if reporting.Output != "" && reporting.Format != "text" {
		_, _ = fmt.Fprintf(stdout, "wrote %s report to %s\n", reporting.Format, reporting.Output)
	}
	return nil
}

func maybeWriteBuildkiteRunOutputs(opts buildkiteOptions, reporting cleanr.ReportingConfig, report cleanr.Report, resolvedConfigPath string) error {
	if !opts.Meta && !opts.Annotation {
		return nil
	}
	buildCtx := context.Background()
	reportPath := resolveConfigRelativePath(resolvedConfigPath, reporting.Output)
	if opts.Meta {
		if err := writeBuildkiteMetadata(buildCtx, buildBuildkiteRunMetadata(report, reportPath, reporting.Format)); err != nil {
			return err
		}
	}
	if opts.Annotation {
		if err := writeBuildkiteAnnotation(buildCtx, "cleanr-run", "error", buildBuildkiteRunAnnotation(report)); err != nil {
			return err
		}
	}
	return nil
}

func snapshotCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to cleanr config")
	profile := fs.String("profile", "", "Optional staged config profile: pr, main, or release")
	output := fs.String("output", "", "Path to write snapshot baseline")
	timeout := fs.Duration("timeout", 0, "Overall execution timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedConfigPath, err := resolveConfigPath(*configPath, *profile)
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
	if len(cfg.Scenarios) == 0 {
		_, _ = fmt.Fprintln(stderr, "snapshot error: config contains no scenarios")
		return 2
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

func generateCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to cleanr config")
	profile := fs.String("profile", "", "Optional staged config profile: pr, main, or release")
	output := fs.String("output", "", "Path to write the generated scenario dataset")
	count := fs.Int("count", 0, "Optional override for scenario_generation.count")
	timeout := fs.Duration("timeout", 0, "Overall execution timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedConfigPath, err := resolveConfigPath(*configPath, *profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate error: %v\n", err)
		return 2
	}
	cfg, err := cleanr.LoadConfigFile(resolvedConfigPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate error: %v\n", err)
		return 2
	}
	if !cfg.ScenarioGeneration.Enabled {
		_, _ = fmt.Fprintln(stderr, "generate error: scenario_generation.enabled is false")
		return 2
	}
	if *count > 0 {
		cfg.ScenarioGeneration.Count = *count
	}
	outputPath := strings.TrimSpace(*output)
	if outputPath == "" {
		outputPath = cfg.ScenarioGeneration.OutputFile
	}
	outputPath = resolveConfigRelativePath(resolvedConfigPath, outputPath)
	if strings.TrimSpace(outputPath) == "" {
		_, _ = fmt.Fprintln(stderr, "generate error: no output path configured; set scenario_generation.output_file or pass -output")
		return 2
	}

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	client := &http.Client{Timeout: cfg.ScenarioGeneration.Provider.Timeout()}
	dataset, err := cleanr.GenerateScenarioDataset(ctx, cfg, client)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate error: %v\n", err)
		return 2
	}
	if err := cleanr.WriteScenarioDatasetFile(outputPath, dataset); err != nil {
		_, _ = fmt.Fprintf(stderr, "generate error: %v\n", err)
		return 2
	}
	for _, warning := range dataset.Warnings {
		_, _ = fmt.Fprintf(stderr, "generate warning: %s\n", warning)
	}
	_, _ = fmt.Fprintf(stdout, "wrote %d generated scenarios to %s\n", len(dataset.Scenarios), outputPath)
	return 0
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
