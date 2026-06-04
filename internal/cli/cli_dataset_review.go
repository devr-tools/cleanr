package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/devr-tools/cleanr/cleanr"
)

func datasetReviewCmd(args []string, stdout, stderr io.Writer) int {
	cmd, code := parseDatasetReviewCommand(args, stderr)
	if code != 0 {
		return code
	}

	ctx, err := loadDatasetReviewContext(cmd)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset review error: %v\n", err)
		return 2
	}
	if cmd.Interactive {
		reviewed, err := runInteractiveDatasetReview(os.Stdin, stdout, ctx.Reviewed)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "dataset review error: %v\n", err)
			return 2
		}
		ctx.Reviewed = reviewed
	}
	if code := persistDatasetReviewOutputs(stdout, stderr, ctx); code != 0 {
		return code
	}

	gate := evaluateDatasetReviewGate(ctx.Reviewed, cmd.Gate)
	if cmd.GitHubOutputs {
		if err := writeDatasetReviewGitHubOutputs(ctx.Reviewed, gate, ctx.OutputPath, ctx.MergePath); err != nil {
			_, _ = fmt.Fprintf(stderr, "dataset review warning: %v\n", err)
		}
	}
	if err := maybeWriteBuildkiteReviewOutputs(cmd.Buildkite, ctx, gate); err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset review warning: %v\n", err)
	}
	if !gate.Passed {
		for _, message := range gate.Messages {
			_, _ = fmt.Fprintf(stderr, "dataset review gate: %s\n", message)
		}
		return 1
	}
	return 0
}

func parseDatasetReviewCommand(args []string, stderr io.Writer) (datasetReviewCommandOptions, int) {
	fs := flag.NewFlagSet("dataset review", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cmd := datasetReviewCommandOptions{}
	input := fs.String("input", "", "Path to scenario dataset file")
	policyPath := fs.String("policy", "", "Optional dataset review policy file")
	baseConfig := fs.String("base-config", "", "Optional base cleanr config to diff and merge against")
	profile := fs.String("profile", "", "Optional staged config profile: pr, main, or release")
	output := fs.String("output", "cleanr.reviewed.yaml", "Path to write the reviewed dataset artifact")
	mergeOutput := fs.String("merge-output", "", "Optional path to write the merged cleanr config containing approved scenarios")
	mergeInPlace := fs.Bool("merge-in-place", false, "Write approved scenarios back into the resolved base config path")
	format := fs.String("format", "text", "Review output format: text or json")
	interactive := fs.Bool("interactive", false, "Review scenarios interactively from stdin before writing outputs")
	failOnPending := fs.Bool("fail-on-pending", false, "Exit with code 1 if any reviewed scenarios remain pending")
	failOnRejected := fs.Bool("fail-on-rejected", false, "Exit with code 1 if any reviewed scenarios are rejected")
	minApproved := fs.Int("min-approved", 0, "Minimum approved scenario count required for exit code 0")
	maxDuplicates := fs.Int("max-duplicates", -1, "Maximum duplicate candidates allowed before exit code 1")
	githubOutputs := fs.Bool("github-outputs", false, "Write review metrics to $GITHUB_OUTPUT and $GITHUB_STEP_SUMMARY when available")
	buildkiteMeta := fs.Bool("buildkite-meta", false, "Write review metrics to Buildkite metadata when buildkite-agent is available")
	buildkiteAnnotation := fs.Bool("buildkite-annotation", false, "Write a Buildkite annotation when the review gate fails and buildkite-agent is available")
	buildkiteUploadArtifacts := fs.Bool("buildkite-upload-artifacts", false, "Upload reviewed artifacts with buildkite-agent when available")
	var approve repeatedStringFlag
	var reject repeatedStringFlag
	var promoteStable repeatedStringFlag
	var promoteRegression repeatedStringFlag
	var addTag repeatedStringFlag
	var setTags repeatedStringFlag
	var setMetadata repeatedStringFlag
	fs.Var(&approve, "approve", "Approve a scenario by name. Repeat or pass comma-separated values")
	fs.Var(&reject, "reject", "Reject a scenario by name. Repeat or pass comma-separated values")
	fs.Var(&promoteStable, "promote-stable", "Promote a scenario to stable by adding the stable tag")
	fs.Var(&promoteRegression, "promote-regression", "Promote a scenario to regression by adding the regression tag")
	fs.Var(&addTag, "add-tag", "Add a tag using name:tag")
	fs.Var(&setTags, "set-tags", "Replace tags using name=tag1,tag2")
	fs.Var(&setMetadata, "set-metadata", "Set scenario metadata using name:key=value")
	if err := fs.Parse(args); err != nil {
		return datasetReviewCommandOptions{}, 2
	}

	cmd = datasetReviewCommandOptions{
		Input:         strings.TrimSpace(*input),
		Policy:        strings.TrimSpace(*policyPath),
		BaseConfig:    strings.TrimSpace(*baseConfig),
		Profile:       strings.TrimSpace(*profile),
		Output:        *output,
		MergeOutput:   *mergeOutput,
		Format:        *format,
		Interactive:   *interactive,
		MergeInPlace:  *mergeInPlace,
		GitHubOutputs: *githubOutputs,
		Buildkite: buildkiteOptions{
			Meta:            *buildkiteMeta,
			Annotation:      *buildkiteAnnotation,
			UploadArtifacts: *buildkiteUploadArtifacts,
		},
		Gate: datasetReviewGateOptions{
			FailOnPending:  *failOnPending,
			FailOnRejected: *failOnRejected,
			MinApproved:    *minApproved,
			MaxDuplicates:  *maxDuplicates,
			WriteGHOutputs: *githubOutputs,
		},
		Approve:       approve,
		Reject:        reject,
		PromoteStable: promoteStable,
		PromoteReg:    promoteRegression,
		AddTag:        addTag,
		SetTags:       setTags,
		SetMetadata:   setMetadata,
	}
	if err := validateDatasetReviewCommand(cmd); err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset review error: %v\n", err)
		return datasetReviewCommandOptions{}, 2
	}
	return cmd, 0
}

func validateDatasetReviewCommand(cmd datasetReviewCommandOptions) error {
	switch {
	case cmd.Input == "":
		return fmt.Errorf("-input is required")
	case cmd.MergeInPlace && strings.TrimSpace(cmd.MergeOutput) != "":
		return fmt.Errorf("use either -merge-output or -merge-in-place, not both")
	case cmd.Gate.MinApproved < 0:
		return fmt.Errorf("-min-approved must be >= 0")
	case cmd.Gate.MaxDuplicates < -1:
		return fmt.Errorf("-max-duplicates must be >= -1")
	default:
		return nil
	}
}

func loadDatasetReviewContext(cmd datasetReviewCommandOptions) (datasetReviewCommandContext, error) {
	dataset, err := cleanr.LoadScenarioDatasetFile(cmd.Input)
	if err != nil {
		return datasetReviewCommandContext{}, err
	}
	if len(dataset.Scenarios) == 0 {
		return datasetReviewCommandContext{}, fmt.Errorf("dataset contains no scenarios")
	}

	resolvedBasePath, err := resolveConfigPath(cmd.BaseConfig, cmd.Profile)
	if err != nil {
		return datasetReviewCommandContext{}, err
	}
	baseCfg, err := cleanr.LoadConfigFile(resolvedBasePath)
	if err != nil {
		return datasetReviewCommandContext{}, err
	}

	opts, err := parseDatasetReviewOptions(datasetReviewOptionInputs{
		Approve:           cmd.Approve,
		Reject:            cmd.Reject,
		PromoteStable:     cmd.PromoteStable,
		PromoteRegression: cmd.PromoteReg,
		AddTag:            cmd.AddTag,
		SetTags:           cmd.SetTags,
		SetMetadata:       cmd.SetMetadata,
	})
	if err != nil {
		return datasetReviewCommandContext{}, err
	}
	policyPath := ""
	if resolvedPolicyPath, ok, err := resolveDatasetReviewPolicyPath(cmd.Policy, resolvedBasePath, cmd.Profile); err != nil {
		return datasetReviewCommandContext{}, err
	} else if ok {
		policy, err := cleanr.LoadDatasetReviewPolicyFile(resolvedPolicyPath)
		if err != nil {
			return datasetReviewCommandContext{}, err
		}
		policyPath = resolvedPolicyPath
		opts.Policy = &policy
	}
	reviewed, err := cleanr.ReviewDatasetAgainstConfig(baseCfg, dataset, opts)
	if err != nil {
		return datasetReviewCommandContext{}, err
	}
	reviewed.PolicyPath = policyPath
	if opts.Policy != nil {
		reviewed.PolicyVersion = opts.Policy.Version
	}

	return datasetReviewCommandContext{
		BaseConfig: baseCfg,
		Reviewed:   reviewed,
		PolicyPath: policyPath,
		OutputPath: resolveConfigRelativePath(resolvedBasePath, cmd.Output),
		MergePath:  resolveDatasetReviewMergePath(cmd, resolvedBasePath),
		JSONOutput: strings.EqualFold(strings.TrimSpace(cmd.Format), "json"),
	}, nil
}

func persistDatasetReviewOutputs(stdout, stderr io.Writer, ctx datasetReviewCommandContext) int {
	if err := cleanr.WriteReviewedScenarioDatasetFile(ctx.OutputPath, ctx.Reviewed); err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset review error: %v\n", err)
		return 2
	}
	if code := renderDatasetReview(stdout, ctx); code != 0 {
		return code
	}
	if ctx.MergePath == "" {
		return 0
	}

	merged := cleanr.MergeReviewedDatasetIntoConfig(ctx.BaseConfig, ctx.Reviewed)
	if err := cleanr.WriteConfigFile(ctx.MergePath, merged); err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset review error: %v\n", err)
		return 2
	}
	if !ctx.JSONOutput {
		_, _ = fmt.Fprintf(stdout, "wrote merged config with %d approved scenarios to %s\n", ctx.Reviewed.ApprovedScenarios, ctx.MergePath)
	}
	return 0
}

func renderDatasetReview(stdout io.Writer, ctx datasetReviewCommandContext) int {
	if ctx.JSONOutput {
		return writeJSON(stdout, ctx.Reviewed)
	}
	writeReviewedDatasetText(stdout, ctx.Reviewed)
	if strings.TrimSpace(ctx.PolicyPath) != "" {
		_, _ = fmt.Fprintf(stdout, "applied review policy: %s\n", ctx.PolicyPath)
	}
	_, _ = fmt.Fprintf(stdout, "wrote reviewed dataset to %s\n", ctx.OutputPath)
	return 0
}

func resolveDatasetReviewMergePath(cmd datasetReviewCommandOptions, resolvedBasePath string) string {
	switch {
	case cmd.MergeInPlace:
		return resolvedBasePath
	case strings.TrimSpace(cmd.MergeOutput) != "":
		return resolveConfigRelativePath(resolvedBasePath, cmd.MergeOutput)
	default:
		return ""
	}
}

type datasetReviewOptionInputs struct {
	Approve           []string
	Reject            []string
	PromoteStable     []string
	PromoteRegression []string
	AddTag            []string
	SetTags           []string
	SetMetadata       []string
}

func parseDatasetReviewOptions(input datasetReviewOptionInputs) (cleanr.DatasetReviewOptions, error) {
	opts := cleanr.DatasetReviewOptions{
		Approve:           expandNameList(input.Approve),
		Reject:            expandNameList(input.Reject),
		PromoteStable:     expandNameList(input.PromoteStable),
		PromoteRegression: expandNameList(input.PromoteRegression),
		AddTags:           map[string][]string{},
		SetTags:           map[string][]string{},
		SetMetadata:       map[string]map[string]string{},
	}
	for _, raw := range input.AddTag {
		name, tag, ok := strings.Cut(strings.TrimSpace(raw), ":")
		if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(tag) == "" {
			return cleanr.DatasetReviewOptions{}, fmt.Errorf("invalid -add-tag value %q; expected name:tag", raw)
		}
		opts.AddTags[strings.TrimSpace(name)] = append(opts.AddTags[strings.TrimSpace(name)], strings.TrimSpace(tag))
	}
	for _, raw := range input.SetTags {
		name, list, ok := strings.Cut(strings.TrimSpace(raw), "=")
		if !ok || strings.TrimSpace(name) == "" {
			return cleanr.DatasetReviewOptions{}, fmt.Errorf("invalid -set-tags value %q; expected name=tag1,tag2", raw)
		}
		opts.SetTags[strings.TrimSpace(name)] = expandNameList([]string{list})
	}
	for _, raw := range input.SetMetadata {
		name, assignment, ok := strings.Cut(strings.TrimSpace(raw), ":")
		if !ok || strings.TrimSpace(name) == "" {
			return cleanr.DatasetReviewOptions{}, fmt.Errorf("invalid -set-metadata value %q; expected name:key=value", raw)
		}
		key, value, ok := strings.Cut(strings.TrimSpace(assignment), "=")
		if !ok || strings.TrimSpace(key) == "" {
			return cleanr.DatasetReviewOptions{}, fmt.Errorf("invalid -set-metadata value %q; expected name:key=value", raw)
		}
		name = strings.TrimSpace(name)
		if opts.SetMetadata[name] == nil {
			opts.SetMetadata[name] = map[string]string{}
		}
		opts.SetMetadata[name][strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return opts, nil
}

func expandNameList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			out = append(out, part)
		}
	}
	return out
}

func writeReviewedDatasetText(w io.Writer, reviewed cleanr.ReviewedScenarioDataset) {
	_, _ = fmt.Fprintf(w, "reviewed %d candidates: %d approved, %d rejected, %d pending\n", reviewed.Summary.TotalCandidates, reviewed.ApprovedScenarios, reviewed.RejectedScenarios, reviewed.PendingScenarios)
	for _, item := range reviewed.Scenarios {
		line := map[string]any{
			"name":               item.Entry.Scenario.Name,
			"decision":           item.Decision.Status,
			"diff_status":        item.Diff.Status,
			"usefulness_score":   item.Analysis.UsefulnessScore,
			"highest_severity":   item.Analysis.HighestSeverity,
			"stable_suitability": item.Analysis.StableSuitability,
			"changes":            item.Diff.Summary,
		}
		if item.Diff.DuplicateOf != "" {
			line["duplicate_of"] = item.Diff.DuplicateOf
		}
		if len(item.Decision.PolicyRules) > 0 {
			line["policy_rules"] = item.Decision.PolicyRules
		}
		data, _ := json.Marshal(line)
		_, _ = fmt.Fprintln(w, string(data))
	}
}

func evaluateDatasetReviewGate(reviewed cleanr.ReviewedScenarioDataset, opts datasetReviewGateOptions) datasetReviewGateResult {
	result := datasetReviewGateResult{Passed: true}
	if opts.FailOnPending && reviewed.PendingScenarios > 0 {
		result.Passed = false
		result.Messages = append(result.Messages, fmt.Sprintf("found %d pending scenarios", reviewed.PendingScenarios))
	}
	if opts.FailOnRejected && reviewed.RejectedScenarios > 0 {
		result.Passed = false
		result.Messages = append(result.Messages, fmt.Sprintf("found %d rejected scenarios", reviewed.RejectedScenarios))
	}
	if reviewed.ApprovedScenarios < opts.MinApproved {
		result.Passed = false
		result.Messages = append(result.Messages, fmt.Sprintf("approved scenario count %d is below required minimum %d", reviewed.ApprovedScenarios, opts.MinApproved))
	}
	if opts.MaxDuplicates >= 0 && reviewed.Summary.Duplicates > opts.MaxDuplicates {
		result.Passed = false
		result.Messages = append(result.Messages, fmt.Sprintf("duplicate candidate count %d exceeds maximum %d", reviewed.Summary.Duplicates, opts.MaxDuplicates))
	}
	return result
}
