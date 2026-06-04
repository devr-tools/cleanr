package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/devr-tools/cleanr/cleanr"
)

var buildkiteExecCommandContext = exec.CommandContext
var buildkiteLookPath = exec.LookPath

func buildkiteAgentAvailable() bool {
	_, err := buildkiteLookPath("buildkite-agent")
	return err == nil
}

func runBuildkiteAgent(ctx context.Context, args ...string) error {
	if !buildkiteAgentAvailable() {
		return fmt.Errorf("buildkite-agent is not available")
	}
	cmd := buildkiteExecCommandContext(ctx, "buildkite-agent", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("buildkite-agent %s: %s", strings.Join(args, " "), message)
	}
	return nil
}

func writeBuildkiteMetadata(ctx context.Context, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	for key, value := range values {
		if err := runBuildkiteAgent(ctx, "meta-data", "set", key, value); err != nil {
			return err
		}
	}
	return nil
}

func writeBuildkiteAnnotation(ctx context.Context, contextName, style, body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	args := []string{"annotate", body, "--style", style}
	if strings.TrimSpace(contextName) != "" {
		args = append(args, "--context", strings.TrimSpace(contextName))
	}
	return runBuildkiteAgent(ctx, args...)
}

func uploadBuildkiteArtifacts(ctx context.Context, paths []string) error {
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("buildkite artifact %s: %w", path, err)
		}
		if err := runBuildkiteAgent(ctx, "artifact", "upload", path); err != nil {
			return err
		}
	}
	return nil
}

func buildBuildkiteRunMetadata(report cleanr.Report, reportPath string, reportFormat string) map[string]string {
	values := map[string]string{
		"cleanr.run.passed":        boolString(report.Passed),
		"cleanr.run.total_suites":  strconv.Itoa(report.TotalSuites),
		"cleanr.run.failed_suites": strconv.Itoa(report.FailedSuites),
		"cleanr.run.total_cases":   strconv.Itoa(report.TotalCases),
		"cleanr.run.failed_cases":  strconv.Itoa(report.FailedCases),
		"cleanr.run.report_format": strings.TrimSpace(reportFormat),
	}
	if strings.TrimSpace(report.Name) != "" {
		values["cleanr.run.target"] = strings.TrimSpace(report.Name)
	}
	if strings.TrimSpace(reportPath) != "" {
		values["cleanr.run.report_output"] = strings.TrimSpace(reportPath)
	}
	if report.Metadata != nil {
		if strings.TrimSpace(report.Metadata.BuildID) != "" {
			values["cleanr.run.build_id"] = strings.TrimSpace(report.Metadata.BuildID)
		}
		if strings.TrimSpace(report.Metadata.ProviderModel) != "" {
			values["cleanr.run.provider_model"] = strings.TrimSpace(report.Metadata.ProviderModel)
		}
		if strings.TrimSpace(report.Metadata.TargetType) != "" {
			values["cleanr.run.target_type"] = strings.TrimSpace(report.Metadata.TargetType)
		}
	}
	if report.TrendGate != nil {
		values["cleanr.run.trend_gate_enabled"] = boolString(report.TrendGate.Enabled)
		values["cleanr.run.trend_gate_passed"] = boolString(report.TrendGate.Passed)
	}
	return values
}

func buildBuildkiteRunAnnotation(report cleanr.Report) string {
	if report.Passed {
		return ""
	}
	var b strings.Builder
	b.WriteString("### cleanr run failed\n\n")
	b.WriteString(fmt.Sprintf("- Target: `%s`\n", report.Name))
	b.WriteString(fmt.Sprintf("- Failed suites: `%d`\n", report.FailedSuites))
	b.WriteString(fmt.Sprintf("- Failed cases: `%d`\n", report.FailedCases))
	if len(report.Recommendations) > 0 {
		b.WriteString("\nRecommendations:\n")
		for _, rec := range report.Recommendations {
			b.WriteString("- " + rec + "\n")
		}
	}
	return b.String()
}

func buildBuildkiteReviewMetadata(reviewed cleanr.ReviewedScenarioDataset, gate datasetReviewGateResult, reviewPath, mergePath string) map[string]string {
	values := map[string]string{
		"cleanr.review.gate_passed":   boolString(gate.Passed),
		"cleanr.review.total":         strconv.Itoa(reviewed.Summary.TotalCandidates),
		"cleanr.review.approved":      strconv.Itoa(reviewed.ApprovedScenarios),
		"cleanr.review.rejected":      strconv.Itoa(reviewed.RejectedScenarios),
		"cleanr.review.pending":       strconv.Itoa(reviewed.PendingScenarios),
		"cleanr.review.new":           strconv.Itoa(reviewed.Summary.NewCandidates),
		"cleanr.review.modified":      strconv.Itoa(reviewed.Summary.Modified),
		"cleanr.review.duplicates":    strconv.Itoa(reviewed.Summary.Duplicates),
		"cleanr.review.unchanged":     strconv.Itoa(reviewed.Summary.Unchanged),
		"cleanr.review.artifact":      strings.TrimSpace(reviewPath),
		"cleanr.review.policy_path":   strings.TrimSpace(reviewed.PolicyPath),
		"cleanr.review.merge_output":  strings.TrimSpace(mergePath),
		"cleanr.review.top_candidate": topReviewedScenarioName(reviewed),
		"cleanr.review.top_score":     strconv.Itoa(topReviewedScenarioScore(reviewed)),
	}
	return values
}

func buildBuildkiteReviewAnnotation(reviewed cleanr.ReviewedScenarioDataset, gate datasetReviewGateResult) string {
	if gate.Passed {
		return ""
	}
	var b strings.Builder
	b.WriteString("### cleanr dataset review gate failed\n\n")
	b.WriteString(fmt.Sprintf("- Approved: `%d`\n", reviewed.ApprovedScenarios))
	b.WriteString(fmt.Sprintf("- Rejected: `%d`\n", reviewed.RejectedScenarios))
	b.WriteString(fmt.Sprintf("- Pending: `%d`\n", reviewed.PendingScenarios))
	b.WriteString(fmt.Sprintf("- Duplicates: `%d`\n", reviewed.Summary.Duplicates))
	if strings.TrimSpace(reviewed.PolicyPath) != "" {
		b.WriteString(fmt.Sprintf("- Review policy: `%s`\n", reviewed.PolicyPath))
	}
	if len(gate.Messages) > 0 {
		b.WriteString("\nGate findings:\n")
		for _, message := range gate.Messages {
			b.WriteString("- " + message + "\n")
		}
	}
	if len(reviewed.Scenarios) > 0 {
		b.WriteString("\nTop candidates:\n")
		limit := len(reviewed.Scenarios)
		if limit > 3 {
			limit = 3
		}
		for i := 0; i < limit; i++ {
			item := reviewed.Scenarios[i]
			b.WriteString(fmt.Sprintf("- `%s` score=`%d` decision=`%s` diff=`%s`\n", item.Entry.Scenario.Name, item.Analysis.UsefulnessScore, item.Decision.Status, item.Diff.Status))
		}
	}
	return b.String()
}
