package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/devr-tools/cleanr/cleanr"
)

func writeDatasetReviewGitHubOutputs(reviewed cleanr.ReviewedScenarioDataset, gate datasetReviewGateResult, reviewPath, mergePath string) (err error) {
	outputPath := strings.TrimSpace(os.Getenv("GITHUB_OUTPUT"))
	summaryPath := strings.TrimSpace(os.Getenv("GITHUB_STEP_SUMMARY"))
	if outputPath == "" && summaryPath == "" {
		return nil
	}

	if outputPath != "" {
		if err := writeDatasetReviewGitHubOutputFile(outputPath, reviewed, gate, reviewPath, mergePath); err != nil {
			return err
		}
	}

	if summaryPath != "" {
		if err := writeDatasetReviewGitHubSummaryFile(summaryPath, reviewed, gate, reviewPath, mergePath); err != nil {
			return err
		}
	}
	return nil
}

func writeDatasetReviewGitHubOutputFile(path string, reviewed cleanr.ReviewedScenarioDataset, gate datasetReviewGateResult, reviewPath, mergePath string) error {
	f, openErr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if openErr != nil {
		return fmt.Errorf("open GITHUB_OUTPUT: %w", openErr)
	}
	values := map[string]string{
		"cleanr_review_gate_passed":   boolString(gate.Passed),
		"cleanr_review_total":         fmt.Sprintf("%d", reviewed.Summary.TotalCandidates),
		"cleanr_review_approved":      fmt.Sprintf("%d", reviewed.ApprovedScenarios),
		"cleanr_review_rejected":      fmt.Sprintf("%d", reviewed.RejectedScenarios),
		"cleanr_review_pending":       fmt.Sprintf("%d", reviewed.PendingScenarios),
		"cleanr_review_new":           fmt.Sprintf("%d", reviewed.Summary.NewCandidates),
		"cleanr_review_modified":      fmt.Sprintf("%d", reviewed.Summary.Modified),
		"cleanr_review_duplicates":    fmt.Sprintf("%d", reviewed.Summary.Duplicates),
		"cleanr_review_unchanged":     fmt.Sprintf("%d", reviewed.Summary.Unchanged),
		"cleanr_review_artifact":      reviewPath,
		"cleanr_review_policy_path":   reviewed.PolicyPath,
		"cleanr_review_merge_output":  mergePath,
		"cleanr_review_top_candidate": topReviewedScenarioName(reviewed),
		"cleanr_review_top_score":     fmt.Sprintf("%d", topReviewedScenarioScore(reviewed)),
	}
	for key, value := range values {
		if _, writeErr := fmt.Fprintf(f, "%s=%s\n", key, escapeGitHubOutput(value)); writeErr != nil {
			_ = f.Close()
			return fmt.Errorf("write GITHUB_OUTPUT: %w", writeErr)
		}
	}
	if closeErr := f.Close(); closeErr != nil {
		return fmt.Errorf("close GITHUB_OUTPUT: %w", closeErr)
	}
	return nil
}

func writeDatasetReviewGitHubSummaryFile(path string, reviewed cleanr.ReviewedScenarioDataset, gate datasetReviewGateResult, reviewPath, mergePath string) error {
	f, openErr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if openErr != nil {
		return fmt.Errorf("open GITHUB_STEP_SUMMARY: %w", openErr)
	}
	for _, line := range buildDatasetReviewGitHubSummaryLines(reviewed, gate, reviewPath, mergePath) {
		if _, writeErr := fmt.Fprintln(f, line); writeErr != nil {
			_ = f.Close()
			return fmt.Errorf("write GITHUB_STEP_SUMMARY: %w", writeErr)
		}
	}
	if closeErr := f.Close(); closeErr != nil {
		return fmt.Errorf("close GITHUB_STEP_SUMMARY: %w", closeErr)
	}
	return nil
}

func buildDatasetReviewGitHubSummaryLines(reviewed cleanr.ReviewedScenarioDataset, gate datasetReviewGateResult, reviewPath, mergePath string) []string {
	lines := []string{
		"## cleanr Dataset Review",
		"",
		fmt.Sprintf("- Gate passed: `%s`", strings.ToLower(boolString(gate.Passed))),
		fmt.Sprintf("- Approved: `%d`", reviewed.ApprovedScenarios),
		fmt.Sprintf("- Rejected: `%d`", reviewed.RejectedScenarios),
		fmt.Sprintf("- Pending: `%d`", reviewed.PendingScenarios),
		fmt.Sprintf("- Duplicates: `%d`", reviewed.Summary.Duplicates),
		fmt.Sprintf("- Review artifact: `%s`", reviewPath),
	}
	if strings.TrimSpace(reviewed.PolicyPath) != "" {
		lines = append(lines, fmt.Sprintf("- Review policy: `%s`", reviewed.PolicyPath))
	}
	if strings.TrimSpace(mergePath) != "" {
		lines = append(lines, fmt.Sprintf("- Merge output: `%s`", mergePath))
	}
	if len(gate.Messages) == 0 {
		return lines
	}
	lines = append(lines, "", "Gate findings:")
	for _, message := range gate.Messages {
		lines = append(lines, "- "+message)
	}
	return lines
}

func topReviewedScenarioName(reviewed cleanr.ReviewedScenarioDataset) string {
	if len(reviewed.Scenarios) == 0 {
		return ""
	}
	return reviewed.Scenarios[0].Entry.Scenario.Name
}

func topReviewedScenarioScore(reviewed cleanr.ReviewedScenarioDataset) int {
	if len(reviewed.Scenarios) == 0 {
		return 0
	}
	return reviewed.Scenarios[0].Analysis.UsefulnessScore
}

func escapeGitHubOutput(value string) string {
	replacer := strings.NewReplacer("%", "%25", "\n", "%0A", "\r", "%0D")
	return replacer.Replace(value)
}

func maybeWriteBuildkiteReviewOutputs(opts buildkiteOptions, ctx datasetReviewCommandContext, gate datasetReviewGateResult) error {
	if !opts.Meta && !opts.Annotation && !opts.UploadArtifacts {
		return nil
	}
	buildCtx := context.Background()
	if opts.Meta {
		if err := writeBuildkiteMetadata(buildCtx, buildBuildkiteReviewMetadata(ctx.Reviewed, gate, ctx.OutputPath, ctx.MergePath)); err != nil {
			return err
		}
	}
	if opts.Annotation {
		if err := writeBuildkiteAnnotation(buildCtx, "cleanr-dataset-review", "error", buildBuildkiteReviewAnnotation(ctx.Reviewed, gate)); err != nil {
			return err
		}
	}
	if opts.UploadArtifacts {
		paths := []string{ctx.OutputPath}
		if strings.TrimSpace(ctx.MergePath) != "" {
			paths = append(paths, ctx.MergePath)
		}
		if err := uploadBuildkiteArtifacts(buildCtx, paths); err != nil {
			return err
		}
	}
	return nil
}
