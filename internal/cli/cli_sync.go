package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

type syncBraintrustOptions struct {
	configPath      string
	profile         string
	project         string
	experiment      string
	apiKeyEnv       string
	baseURL         string
	timeoutMS       int
	historyLimit    int
	outputInsights  string
	outputDataset   string
	outputConfig    string
	applyScenarios  bool
	applyPatches    bool
	approveInsights bool
	createPR        bool
	prBranch        string
	prBase          string
	prTitle         string
	prBody          string
	commitMessage   string
}

func syncCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "sync error: expected a sync target such as braintrust")
		return 2
	}
	switch args[0] {
	case "braintrust":
		return syncBraintrustCmd(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "sync error: unsupported subcommand %s\n", args[0])
		return 2
	}
}

func syncBraintrustCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sync braintrust", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := bindSyncBraintrustFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if opts.historyLimit < 0 {
		_, _ = fmt.Fprintln(stderr, "sync braintrust error: history-limit must be >= 0")
		return 2
	}
	return runSyncBraintrust(*opts, stdout, stderr)
}

func bindSyncBraintrustFlags(fs *flag.FlagSet) *syncBraintrustOptions {
	opts := &syncBraintrustOptions{}
	fs.StringVar(&opts.configPath, "config", "", "Path to the base cleanr config to sync into")
	fs.StringVar(&opts.profile, "profile", "", "Optional staged config profile: pr, main, or release")
	fs.StringVar(&opts.project, "project", "", "Braintrust project name")
	fs.StringVar(&opts.experiment, "experiment", "", "Optional Braintrust experiment family")
	fs.StringVar(&opts.apiKeyEnv, "api-key-env", "", "Environment variable name used for the Braintrust API key")
	fs.StringVar(&opts.baseURL, "base-url", "", "Optional Braintrust base URL override")
	fs.IntVar(&opts.timeoutMS, "timeout-ms", 0, "Optional Braintrust API timeout in milliseconds")
	fs.IntVar(&opts.historyLimit, "history-limit", 10, "Number of recent Braintrust experiments to scan")
	fs.StringVar(&opts.outputInsights, "output-insights", "reports/braintrust.insights.yaml", "Path to write the normalized Braintrust insight dataset")
	fs.StringVar(&opts.outputDataset, "output-dataset", "", "Optional path to write the scenario dataset extracted from Braintrust")
	fs.StringVar(&opts.outputConfig, "output-config", "cleanr.synced.yaml", "Path to write the merged cleanr config")
	fs.BoolVar(&opts.applyScenarios, "apply-scenarios", true, "Merge replay-derived or explicit scenario updates into the output config")
	fs.BoolVar(&opts.applyPatches, "apply-patches", true, "Apply explicit config patch operations from Braintrust insights")
	fs.BoolVar(&opts.approveInsights, "approve-insights", false, "Allow applying Braintrust insights that require explicit review")
	fs.BoolVar(&opts.createPR, "create-pr", false, "Create a Git branch, commit, and GitHub pull request for the generated files")
	fs.StringVar(&opts.prBranch, "pr-branch", "", "Optional Git branch name for the generated PR")
	fs.StringVar(&opts.prBase, "pr-base", "", "Optional GitHub PR base branch")
	fs.StringVar(&opts.prTitle, "pr-title", "", "Optional GitHub PR title")
	fs.StringVar(&opts.prBody, "pr-body", "", "Optional GitHub PR body")
	fs.StringVar(&opts.commitMessage, "commit-message", "", "Optional Git commit message")
	return opts
}

func runSyncBraintrust(opts syncBraintrustOptions, stdout, stderr io.Writer) int {
	resolvedConfigPath, err := resolveConfigPath(opts.configPath, opts.profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sync braintrust error: %v\n", err)
		return 2
	}
	baseCfg, err := cleanr.LoadConfigFile(resolvedConfigPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sync braintrust error: %v\n", err)
		return 2
	}

	source, err := resolveBraintrustSyncSource(baseCfg, opts.project, opts.experiment, opts.apiKeyEnv, opts.baseURL, opts.timeoutMS, opts.historyLimit)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sync braintrust error: %v\n", err)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), syncTimeout(opts.timeoutMS))
	defer cancel()

	dataset, err := cleanr.FetchBraintrustInsightDataset(ctx, source, baseCfg)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sync braintrust error: %v\n", err)
		return 2
	}

	insightsPath := resolveConfigRelativePath(resolvedConfigPath, opts.outputInsights)
	if err := cleanr.WriteBraintrustInsightDatasetFile(insightsPath, dataset); err != nil {
		_, _ = fmt.Fprintf(stderr, "sync braintrust error: %v\n", err)
		return 2
	}

	writtenFiles := []string{insightsPath}
	if strings.TrimSpace(opts.outputDataset) != "" && dataset.ScenarioDataset != nil {
		datasetPath := resolveConfigRelativePath(resolvedConfigPath, opts.outputDataset)
		if err := cleanr.WriteScenarioDatasetFile(datasetPath, *dataset.ScenarioDataset); err != nil {
			_, _ = fmt.Fprintf(stderr, "sync braintrust error: %v\n", err)
			return 2
		}
		writtenFiles = append(writtenFiles, datasetPath)
	}

	mergedCfg, err := cleanr.ApplyBraintrustInsightDataset(baseCfg, dataset, opts.applyScenarios, opts.applyPatches, opts.approveInsights)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sync braintrust error: %v\n", err)
		return 2
	}
	configOutPath := resolveConfigRelativePath(resolvedConfigPath, opts.outputConfig)
	if err := cleanr.WriteConfigFile(configOutPath, mergedCfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "sync braintrust error: %v\n", err)
		return 2
	}
	writtenFiles = append(writtenFiles, configOutPath)

	if opts.createPR {
		prOpts := gitHubPROptions{
			Files:         writtenFiles,
			Branch:        strings.TrimSpace(opts.prBranch),
			Base:          strings.TrimSpace(opts.prBase),
			Title:         firstNonEmpty(opts.prTitle, defaultSyncPRTitle(dataset, source)),
			Body:          firstNonEmpty(opts.prBody, defaultSyncPRBody(dataset, writtenFiles)),
			CommitMessage: firstNonEmpty(opts.commitMessage, defaultSyncCommitMessage(dataset, source)),
		}
		if err := createGitHubPR(context.Background(), prOpts); err != nil {
			_, _ = fmt.Fprintf(stderr, "sync braintrust error: %v\n", err)
			return 2
		}
	}

	_, _ = fmt.Fprintf(stdout, "wrote braintrust insights to %s\n", insightsPath)
	if strings.TrimSpace(opts.outputDataset) != "" && dataset.ScenarioDataset != nil {
		_, _ = fmt.Fprintf(stdout, "wrote scenario dataset with %d scenarios to %s\n", len(dataset.ScenarioDataset.Scenarios), resolveConfigRelativePath(resolvedConfigPath, opts.outputDataset))
	}
	_, _ = fmt.Fprintf(stdout, "wrote merged config to %s\n", configOutPath)
	if opts.createPR {
		_, _ = fmt.Fprintln(stdout, "created Git branch, commit, and GitHub pull request")
	}
	return 0
}

func resolveBraintrustSyncSource(cfg cleanr.Config, project, experiment, apiKeyEnv, baseURL string, timeoutMS, historyLimit int) (cleanr.TrendSourceConfig, error) {
	source := cleanr.TrendSourceConfig{
		Type:         "braintrust",
		Project:      strings.TrimSpace(project),
		Experiment:   strings.TrimSpace(experiment),
		APIKeyEnv:    strings.TrimSpace(apiKeyEnv),
		BaseURL:      strings.TrimSpace(baseURL),
		TimeoutMS:    timeoutMS,
		HistoryLimit: historyLimit,
	}
	if source.Project != "" {
		if source.APIKeyEnv == "" {
			source.APIKeyEnv = "BRAINTRUST_API_KEY"
		}
		return source, nil
	}
	for _, item := range cfg.Integrations.TrendSources {
		if strings.TrimSpace(item.Type) != "braintrust" {
			continue
		}
		source = item
		if strings.TrimSpace(project) != "" {
			source.Project = strings.TrimSpace(project)
		}
		if strings.TrimSpace(experiment) != "" {
			source.Experiment = strings.TrimSpace(experiment)
		}
		if strings.TrimSpace(apiKeyEnv) != "" {
			source.APIKeyEnv = strings.TrimSpace(apiKeyEnv)
		}
		if strings.TrimSpace(baseURL) != "" {
			source.BaseURL = strings.TrimSpace(baseURL)
		}
		if timeoutMS > 0 {
			source.TimeoutMS = timeoutMS
		}
		if historyLimit > 0 {
			source.HistoryLimit = historyLimit
		}
		return source, nil
	}
	return cleanr.TrendSourceConfig{}, fmt.Errorf("no braintrust trend source configured; pass -project or add integrations.trend_sources[].type: braintrust")
}

func syncTimeout(timeoutMS int) time.Duration {
	if timeoutMS > 0 {
		return time.Duration(timeoutMS) * time.Millisecond
	}
	return 20 * time.Second
}

func defaultSyncPRTitle(dataset cleanr.BraintrustInsightDataset, source cleanr.TrendSourceConfig) string {
	label := firstNonEmpty(dataset.BuildID, dataset.Experiment, source.Experiment, "braintrust")
	return "cleanr sync: apply Braintrust insights for " + label
}

func defaultSyncCommitMessage(dataset cleanr.BraintrustInsightDataset, source cleanr.TrendSourceConfig) string {
	label := firstNonEmpty(dataset.BuildID, dataset.Experiment, source.Experiment, "braintrust")
	return "cleanr sync: apply Braintrust insights for " + label
}

func defaultSyncPRBody(dataset cleanr.BraintrustInsightDataset, files []string) string {
	var b strings.Builder
	b.WriteString("## Summary\n\n")
	b.WriteString("- sync source: Braintrust\n")
	if dataset.Project != "" {
		b.WriteString("- project: `" + dataset.Project + "`\n")
	}
	if dataset.Experiment != "" {
		b.WriteString("- experiment family: `" + dataset.Experiment + "`\n")
	}
	if dataset.BuildID != "" {
		b.WriteString("- build id: `" + dataset.BuildID + "`\n")
	}
	if dataset.ExperimentURL != "" {
		b.WriteString("- experiment url: " + dataset.ExperimentURL + "\n")
	}
	if dataset.ScenarioDataset != nil {
		b.WriteString(fmt.Sprintf("- replay-derived scenarios: `%d`\n", len(dataset.ScenarioDataset.Scenarios)))
	}
	if dataset.ConfigPatch != nil {
		b.WriteString(fmt.Sprintf("- config patch operations: `%d`\n", len(dataset.ConfigPatch.Operations)))
	}
	b.WriteString("\n## Files\n\n")
	for _, file := range files {
		b.WriteString("- `" + filepath.ToSlash(file) + "`\n")
	}
	return b.String()
}
