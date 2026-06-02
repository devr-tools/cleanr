package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/devr-tools/cleanr/cleanr"
)

func writeDatasetReviewGitHubOutputs(reviewed cleanr.ReviewedScenarioDataset, gate datasetReviewGateResult, reviewPath, mergePath string) error {
	outputPath := strings.TrimSpace(os.Getenv("GITHUB_OUTPUT"))
	summaryPath := strings.TrimSpace(os.Getenv("GITHUB_STEP_SUMMARY"))
	if outputPath == "" && summaryPath == "" {
		return nil
	}

	if outputPath != "" {
		f, err := os.OpenFile(outputPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			return fmt.Errorf("open GITHUB_OUTPUT: %w", err)
		}
		defer f.Close()
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
			if _, err := fmt.Fprintf(f, "%s=%s\n", key, escapeGitHubOutput(value)); err != nil {
				return fmt.Errorf("write GITHUB_OUTPUT: %w", err)
			}
		}
	}

	if summaryPath != "" {
		f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			return fmt.Errorf("open GITHUB_STEP_SUMMARY: %w", err)
		}
		defer f.Close()
		_, _ = fmt.Fprintln(f, "## cleanr Dataset Review")
		_, _ = fmt.Fprintln(f)
		_, _ = fmt.Fprintf(f, "- Gate passed: `%s`\n", strings.ToLower(boolString(gate.Passed)))
		_, _ = fmt.Fprintf(f, "- Approved: `%d`\n", reviewed.ApprovedScenarios)
		_, _ = fmt.Fprintf(f, "- Rejected: `%d`\n", reviewed.RejectedScenarios)
		_, _ = fmt.Fprintf(f, "- Pending: `%d`\n", reviewed.PendingScenarios)
		_, _ = fmt.Fprintf(f, "- Duplicates: `%d`\n", reviewed.Summary.Duplicates)
		_, _ = fmt.Fprintf(f, "- Review artifact: `%s`\n", reviewPath)
		if strings.TrimSpace(reviewed.PolicyPath) != "" {
			_, _ = fmt.Fprintf(f, "- Review policy: `%s`\n", reviewed.PolicyPath)
		}
		if strings.TrimSpace(mergePath) != "" {
			_, _ = fmt.Fprintf(f, "- Merge output: `%s`\n", mergePath)
		}
		if len(gate.Messages) > 0 {
			_, _ = fmt.Fprintln(f)
			_, _ = fmt.Fprintln(f, "Gate findings:")
			for _, message := range gate.Messages {
				_, _ = fmt.Fprintf(f, "- %s\n", message)
			}
		}
	}
	return nil
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
