package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
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
	gitlab             gitlabOptions
}

func runCmd(args []string, stdout, stderr io.Writer) int {
	opts, err := parseRunOptions(args, stderr)
	if err != nil {
		return 2
	}
	return executeRunCommand(opts, stdout, stderr)
}

func executeRunCommand(opts runOptions, stdout, stderr io.Writer) int {
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
	slog.Debug("run config loaded", "path", resolvedConfigPath, "scenarios", len(cfg.Scenarios))
	applyRunOptions(&cfg, opts)
	if len(cfg.Scenarios) == 0 {
		_, _ = fmt.Fprintln(stderr, "run error: config contains no scenarios; generate or import scenarios before running tests")
		return 2
	}
	resolveRunPaths(&cfg, resolvedConfigPath)

	ctx, cancel := runContext(opts.timeout)
	defer cancel()
	slog.Debug("run starting", "format", cfg.Reporting.Format, "output", cfg.Reporting.Output)
	report := cleanr.NewHTTPRunner(cfg).Run(ctx)
	if ctx.Err() != nil {
		_, _ = fmt.Fprintf(stderr, "run interrupted: %v; writing partial report\n", ctx.Err())
		slog.Warn("run interrupted", "error", ctx.Err())
	}

	// persistWarnings accumulates non-fatal bookkeeping failures so the run can
	// still exit non-zero at the end without ever discarding the paid report.
	var persistWarnings []string

	// Trend history enriches the report and its gates can flip pass/fail, so it
	// must run before the report is written. A persistence failure here is
	// downgraded to a warning instead of discarding the whole run.
	if err := cleanr.AttachTrendHistory(&report, cfg.Reporting.TrendFile, cfg.Reporting.BuildID, cfg.Reporting.TrendLimit); err != nil {
		_, _ = fmt.Fprintf(stderr, "trend history warning: %v\n", err)
		slog.Warn("trend history persistence failed", "error", err)
		persistWarnings = append(persistWarnings, "trend history")
	}
	cleanr.EvaluateTrendGates(&report, cfg.Reporting.TrendGates)

	// Build the replay artifact and attestation in memory and publish
	// integrations before writing the report: these enrich the report content
	// (integration results, attached artifacts) that the written report must
	// contain. Only the on-disk persistence of these artifacts is deferred until
	// after the report is written.
	replayArtifact, hasReplayArtifact := buildReplayArtifact(cfg, report)
	attestation, err := buildAttestation(cfg, report, replayArtifact, hasReplayArtifact, stderr)
	if err != nil {
		slog.Warn("attestation build failed", "error", err)
		persistWarnings = append(persistWarnings, "attestation")
	}
	publishIntegrations(ctx, cfg, &report, replayArtifact, hasReplayArtifact, attestation, resolvedConfigPath, stderr)

	// Write the report first among the disk-persistence side effects: the
	// artifact and attestation file writes below are best-effort and must never
	// prevent the paid report from reaching the user.
	if err := writeRunReport(stdout, stderr, cfg.Reporting, report); err != nil {
		_, _ = fmt.Fprintf(stderr, "write report: %v\n", err)
		slog.Error("write report failed", "error", err)
		return 2
	}
	slog.Debug("run report written", "format", cfg.Reporting.Format, "output", cfg.Reporting.Output, "passed", report.Passed)

	if err := persistReplayArtifactFile(cfg, replayArtifact, stderr); err != nil {
		slog.Warn("replay artifact persistence failed", "error", err)
		persistWarnings = append(persistWarnings, "replay artifact")
	}
	if err := persistAttestationFile(cfg, attestation, resolvedConfigPath, stderr); err != nil {
		slog.Warn("attestation persistence failed", "error", err)
		persistWarnings = append(persistWarnings, "attestation write")
	}

	persistWarnings = append(persistWarnings, emitRunSideOutputs(opts, cfg, report, resolvedConfigPath, stdout, stderr)...)

	// Primary pass/fail from the run result takes precedence; only when the run
	// itself passed do bookkeeping failures surface as the distinct exit code 2.
	if !report.Passed {
		return 1
	}
	if len(persistWarnings) > 0 {
		slog.Warn("run passed but persistence steps failed", "steps", persistWarnings)
		return 2
	}
	return 0
}

// emitRunSideOutputs writes the optional CI/integration side outputs (GitHub,
// Buildkite, GitLab) after the report has been written. All failures are
// non-fatal warnings; it returns any that should surface as the run's distinct
// bookkeeping-failure exit code.
func emitRunSideOutputs(opts runOptions, cfg cleanr.Config, report cleanr.Report, resolvedConfigPath string, stdout, stderr io.Writer) []string {
	var warnings []string
	if opts.githubOutputs {
		if err := writeRunGitHubOutputs(report); err != nil {
			_, _ = fmt.Fprintf(stderr, "github output warning: %v\n", err)
		}
	}
	if opts.githubPRComment {
		number, err := postRunGitHubPRComment(report, opts.githubPRNumber)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "github pr comment warning: %v\n", err)
			slog.Warn("github pr comment failed", "error", err)
			warnings = append(warnings, "github pr comment")
		} else {
			_, _ = fmt.Fprintf(stdout, "posted GitHub PR comment to #%d\n", number)
		}
	}
	if err := maybeWriteBuildkiteRunOutputs(opts.buildkite, cfg.Reporting, report, resolvedConfigPath); err != nil {
		_, _ = fmt.Fprintf(stderr, "buildkite warning: %v\n", err)
	}
	if err := maybeWriteGitLabRunOutputs(opts.gitlab, cfg, report, resolvedConfigPath); err != nil {
		_, _ = fmt.Fprintf(stderr, "gitlab warning: %v\n", err)
	}
	return warnings
}

func parseRunOptions(args []string, stderr io.Writer) (runOptions, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := runOptions{}
	fs.StringVar(&opts.configPath, "config", "", "Path to cleanr config")
	fs.StringVar(&opts.profile, "profile", "", "Optional staged config profile: pr, main, or release")
	fs.StringVar(&opts.format, "format", "", "Report format: text, json, junit, sarif, agent, or html")
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
	fs.StringVar(&opts.gitlab.DotenvPath, "gitlab-dotenv", "", "Write run metrics to a GitLab dotenv report file")
	fs.StringVar(&opts.gitlab.AnnotationsPath, "gitlab-annotations", "", "Write a GitLab annotations report JSON file")
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

// runContext returns a context that is cancelled on SIGINT/SIGTERM so an
// interrupt stops the run gracefully (allowing a partial report to be written),
// with the optional execution timeout layered on top.
func runContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if timeout <= 0 {
		return ctx, stop
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	return timeoutCtx, func() {
		cancel()
		stop()
	}
}

// buildReplayArtifact constructs the in-memory replay artifact when the config
// requires one (for the report, attestation, or integration sinks). It performs
// no disk I/O; see persistReplayArtifactFile for the best-effort write.
func buildReplayArtifact(cfg cleanr.Config, report cleanr.Report) (cleanr.ReplayArtifact, bool) {
	if needsReplayArtifact(cfg) {
		return cleanr.BuildReplayArtifact(report), true
	}
	return cleanr.ReplayArtifact{}, false
}

// persistReplayArtifactFile writes the replay artifact to disk when a path is
// configured. It is best-effort: callers treat a returned error as a warning.
func persistReplayArtifactFile(cfg cleanr.Config, replayArtifact cleanr.ReplayArtifact, stderr io.Writer) error {
	if strings.TrimSpace(cfg.Reporting.ReplayArtifactFile) == "" {
		return nil
	}
	if err := cleanr.WriteReplayArtifactFile(cfg.Reporting.ReplayArtifactFile, replayArtifact); err != nil {
		_, _ = fmt.Fprintf(stderr, "replay artifact error: %v\n", err)
		return err
	}
	return nil
}

// buildAttestation constructs the signed release-gate attestation in memory
// when governance attestation is enabled. It returns (nil, nil) when disabled
// and performs no disk I/O; see persistAttestationFile for the write.
func buildAttestation(cfg cleanr.Config, report cleanr.Report, replayArtifact cleanr.ReplayArtifact, hasReplayArtifact bool, stderr io.Writer) (*cleanr.ReleaseGateAttestation, error) {
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
	return &builtAttestation, nil
}

// persistAttestationFile writes the built attestation to disk. It is
// best-effort: callers treat a returned error as a warning. A nil attestation
// (attestation disabled) is a no-op.
func persistAttestationFile(cfg cleanr.Config, attestation *cleanr.ReleaseGateAttestation, resolvedConfigPath string, stderr io.Writer) error {
	if attestation == nil {
		return nil
	}
	outputPath := resolveConfigRelativePath(resolvedConfigPath, cfg.Governance.Attestation.Output)
	if err := cleanr.WriteReleaseGateAttestationFile(outputPath, *attestation); err != nil {
		_, _ = fmt.Fprintf(stderr, "attestation error: %v\n", err)
		return err
	}
	return nil
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
		if err := os.MkdirAll(filepath.Dir(reporting.Output), 0o755); err != nil {
			return fmt.Errorf("open report output: %w", err)
		}
		f, err := os.Create(reporting.Output)
		if err != nil {
			return fmt.Errorf("open report output: %w", err)
		}
		defer func() { _ = f.Close() }()
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
	if len(args) > 0 && !strings.HasPrefix(strings.TrimSpace(args[0]), "-") {
		return generateAuthoringCmd(args, stdout, stderr)
	}

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
