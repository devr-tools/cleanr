package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/devr-tools/cleanr/cleanr"
)

func writeRunGitHubOutputs(report cleanr.Report) error {
	outputPath := strings.TrimSpace(os.Getenv("GITHUB_OUTPUT"))
	summaryPath := strings.TrimSpace(os.Getenv("GITHUB_STEP_SUMMARY"))
	if outputPath == "" && summaryPath == "" {
		return nil
	}

	if outputPath != "" {
		if err := writeRunGitHubOutputFile(outputPath, report); err != nil {
			return err
		}
	}
	if summaryPath != "" {
		if err := writeRunGitHubSummaryFile(summaryPath, report); err != nil {
			return err
		}
	}
	return nil
}

func writeRunGitHubOutputFile(path string, report cleanr.Report) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open GITHUB_OUTPUT: %w", err)
	}
	defer f.Close()

	commentBody := buildRunGitHubSummaryBody(report)
	values := map[string]string{
		"cleanr_run_gate_passed":      boolString(report.Passed),
		"cleanr_run_failed_suites":    fmt.Sprintf("%d", report.FailedSuites),
		"cleanr_run_failed_cases":     fmt.Sprintf("%d", report.FailedCases),
		"cleanr_run_new_failures":     fmt.Sprintf("%d", len(runCaseRegressions(report))),
		"cleanr_run_worsened_drift":   fmt.Sprintf("%d", len(runWorsenedDrift(report))),
		"cleanr_run_review_scenarios": strings.Join(runRecommendedScenarioNames(report), ","),
		"cleanr_run_gate_summary":     runGateSummary(report),
		"cleanr_run_pr_comment":       commentBody,
	}
	for key, value := range values {
		if _, err := fmt.Fprintf(f, "%s=%s\n", key, escapeGitHubOutput(value)); err != nil {
			return fmt.Errorf("write GITHUB_OUTPUT: %w", err)
		}
	}
	return nil
}

func writeRunGitHubSummaryFile(path string, report cleanr.Report) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open GITHUB_STEP_SUMMARY: %w", err)
	}
	defer f.Close()

	for _, line := range buildRunGitHubSummaryLines(report) {
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("write GITHUB_STEP_SUMMARY: %w", err)
		}
	}
	return nil
}

func buildRunGitHubSummaryBody(report cleanr.Report) string {
	return strings.Join(buildRunGitHubSummaryLines(report), "\n")
}

func buildRunGitHubSummaryLines(report cleanr.Report) []string {
	lines := []string{
		"## cleanr PR Review",
		"",
		fmt.Sprintf("- Gate: `%s`", runGateStatus(report)),
		fmt.Sprintf("- Target: `%s`", report.Name),
		fmt.Sprintf("- Failed suites: `%d`", report.FailedSuites),
		fmt.Sprintf("- Failed cases: `%d`", report.FailedCases),
	}
	if report.Metadata != nil && strings.TrimSpace(report.Metadata.BuildID) != "" {
		lines = append(lines, fmt.Sprintf("- Build: `%s`", report.Metadata.BuildID))
	}

	lines = append(lines, "", "### Gate Explanation", "")
	for _, line := range buildRunGateExplanationLines(report) {
		lines = append(lines, "- "+line)
	}

	lines = append(lines, "")
	lines = append(lines, buildRunSection("New Failures", formatRunCaseRegressions(runCaseRegressions(report)))...)
	lines = append(lines, "")
	lines = append(lines, buildRunSection("Worsened Drift", formatRunWorsenedDrift(runWorsenedDrift(report)))...)
	lines = append(lines, "")
	lines = append(lines, buildRunSection("Recommended Scenarios To Review", formatRunScenarioNames(runRecommendedScenarioNames(report)))...)

	if len(report.Recommendations) > 0 {
		lines = append(lines, "", "### Recommended Follow-up", "")
		for _, rec := range report.Recommendations {
			lines = append(lines, "- "+rec)
		}
	}

	return lines
}

func buildRunSection(title string, items []string) []string {
	lines := []string{"### " + title, ""}
	if len(items) == 0 {
		return append(lines, "- none")
	}
	for _, item := range items {
		lines = append(lines, "- "+item)
	}
	return lines
}

func runGateStatus(report cleanr.Report) string {
	if report.Passed {
		return "PASS"
	}
	return "FAIL"
}

func runGateSummary(report cleanr.Report) string {
	parts := []string{
		fmt.Sprintf("local gate %s", strings.ToLower(runGateStatus(report))),
		fmt.Sprintf("%d failed suites", report.FailedSuites),
		fmt.Sprintf("%d failed cases", report.FailedCases),
	}
	if report.TrendGate != nil {
		parts = append(parts, "trend gate "+strings.ToLower(runTrendGateStatus(*report.TrendGate)))
	}
	return strings.Join(parts, ", ")
}

func buildRunGateExplanationLines(report cleanr.Report) []string {
	lines := []string{
		fmt.Sprintf("Local gate `%s` with `%d` failed suites and `%d` failed cases.", runGateStatus(report), report.FailedSuites, report.FailedCases),
	}
	if report.Trend == nil {
		lines = append(lines, "No retained trend comparison was attached to this run.")
	} else if report.Trend.Baseline {
		lines = append(lines, "Trend comparison is baseline-only because no previous retained run was available.")
	} else {
		lines = append(lines, fmt.Sprintf(
			"Build-over-build delta: `%+d` failed suites, `%+d` failed cases, `%s` duration.",
			report.Trend.Summary.FailedSuitesDelta,
			report.Trend.Summary.FailedCasesDelta,
			report.Trend.Summary.DurationDelta,
		))
	}
	if report.TrendGate == nil {
		return lines
	}
	switch {
	case !report.TrendGate.Evaluated:
		lines = append(lines, "Trend gates were configured but not evaluated for this run window.")
	case report.TrendGate.Passed:
		lines = append(lines, "Trend gates passed.")
	default:
		lines = append(lines, "Trend gates failed.")
		for _, finding := range report.TrendGate.Findings {
			if strings.TrimSpace(finding.Message) != "" {
				lines = append(lines, finding.Message)
			}
		}
	}
	return lines
}

func runTrendGateStatus(gate cleanr.TrendGateReport) string {
	if !gate.Evaluated {
		return "SKIPPED"
	}
	if gate.Passed {
		return "PASS"
	}
	return "FAIL"
}

func runCaseRegressions(report cleanr.Report) []cleanr.CaseTrend {
	if report.Trend == nil {
		return nil
	}
	out := make([]cleanr.CaseTrend, 0, len(report.Trend.CaseRegressions))
	for _, item := range report.Trend.CaseRegressions {
		if item.Status == "new" || item.Status == "regressed" {
			out = append(out, item)
		}
	}
	return out
}

func formatRunCaseRegressions(items []cleanr.CaseTrend) []string {
	if len(items) == 0 {
		return nil
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		line := fmt.Sprintf("`%s/%s` is `%s`", item.Suite, item.Name, item.Status)
		if len(item.NewFindingSignatures) > 0 {
			line += " with new finding `" + item.NewFindingSignatures[0] + "`"
		} else if len(item.FindingSignatures) > 0 {
			line += " with finding `" + item.FindingSignatures[0] + "`"
		} else if item.FirstUnsupportedClaim != "" {
			line += " with unsupported claim `" + item.FirstUnsupportedClaim + "`"
		}
		lines = append(lines, line)
	}
	return lines
}

type runDriftRegression struct {
	Suite   string
	Changes []string
}

func runWorsenedDrift(report cleanr.Report) []runDriftRegression {
	if report.Trend == nil {
		return nil
	}
	out := make([]runDriftRegression, 0)
	for _, suite := range report.Trend.Suites {
		if suite.Drift == nil {
			continue
		}
		changes := make([]string, 0, 6)
		if suite.Drift.NormalizedDriftDelta > 0 {
			changes = append(changes, fmt.Sprintf("normalized %+0.3f", suite.Drift.NormalizedDriftDelta))
		}
		if suite.Drift.SemanticDriftDelta > 0 {
			changes = append(changes, fmt.Sprintf("semantic %+0.3f", suite.Drift.SemanticDriftDelta))
		}
		if suite.Drift.BaselineDriftDelta > 0 {
			changes = append(changes, fmt.Sprintf("baseline %+0.3f", suite.Drift.BaselineDriftDelta))
		}
		if suite.Drift.BaselineSemanticDriftDelta > 0 {
			changes = append(changes, fmt.Sprintf("baseline_semantic %+0.3f", suite.Drift.BaselineSemanticDriftDelta))
		}
		if suite.Drift.ConsistencyScoreDelta < 0 {
			changes = append(changes, fmt.Sprintf("consistency %+0.3f", suite.Drift.ConsistencyScoreDelta))
		}
		if suite.Drift.SemanticConsistencyScoreDelta < 0 {
			changes = append(changes, fmt.Sprintf("semantic_consistency %+0.3f", suite.Drift.SemanticConsistencyScoreDelta))
		}
		if len(changes) == 0 {
			continue
		}
		out = append(out, runDriftRegression{Suite: suite.Name, Changes: changes})
	}
	return out
}

func formatRunWorsenedDrift(items []runDriftRegression) []string {
	if len(items) == 0 {
		return nil
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("`%s`: %s", item.Suite, strings.Join(item.Changes, ", ")))
	}
	return lines
}

func runRecommendedScenarioNames(report cleanr.Report) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, item := range runCaseRegressions(report) {
		if _, ok := seen[item.Name]; ok || strings.TrimSpace(item.Name) == "" {
			continue
		}
		seen[item.Name] = struct{}{}
		out = append(out, item.Name)
	}
	if report.Trend != nil && report.Trend.BuildDiff != nil {
		for _, change := range report.Trend.BuildDiff.ScenarioChanges {
			if change.Status != "new" && change.Status != "changed" {
				continue
			}
			if _, ok := seen[change.Name]; ok || strings.TrimSpace(change.Name) == "" {
				continue
			}
			seen[change.Name] = struct{}{}
			out = append(out, change.Name)
		}
	}
	sort.Strings(out)
	return out
}

func formatRunScenarioNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, "`"+name+"`")
	}
	return lines
}

func postRunGitHubPRComment(report cleanr.Report, requestedPRNumber int) (int, error) {
	if _, err := syncLookPath("gh"); err != nil {
		return 0, fmt.Errorf("gh is not available")
	}
	number, err := resolveGitHubPRNumber(requestedPRNumber)
	if err != nil {
		return 0, err
	}

	bodyFile, err := os.CreateTemp("", "cleanr-pr-comment-*.md")
	if err != nil {
		return 0, fmt.Errorf("create temp comment file: %w", err)
	}
	bodyPath := bodyFile.Name()
	defer os.Remove(bodyPath)
	if _, err := bodyFile.WriteString(buildRunGitHubSummaryBody(report)); err != nil {
		_ = bodyFile.Close()
		return 0, fmt.Errorf("write temp comment file: %w", err)
	}
	if err := bodyFile.Close(); err != nil {
		return 0, fmt.Errorf("close temp comment file: %w", err)
	}

	if err := runSyncCommand(context.Background(), "gh", "pr", "comment", strconv.Itoa(number), "--body-file", bodyPath); err != nil {
		return 0, err
	}
	return number, nil
}

func resolveGitHubPRNumber(requested int) (int, error) {
	if requested > 0 {
		return requested, nil
	}
	eventPath := strings.TrimSpace(os.Getenv("GITHUB_EVENT_PATH"))
	if eventPath == "" {
		return 0, fmt.Errorf("no pull request number provided; pass -github-pr-number or run inside GitHub Actions with GITHUB_EVENT_PATH")
	}
	data, err := os.ReadFile(eventPath)
	if err != nil {
		return 0, fmt.Errorf("read GITHUB_EVENT_PATH: %w", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return 0, fmt.Errorf("parse GITHUB_EVENT_PATH: %w", err)
	}
	if number := extractGitHubPRNumber(payload); number > 0 {
		return number, nil
	}
	return 0, fmt.Errorf("could not resolve pull request number from GITHUB_EVENT_PATH; pass -github-pr-number explicitly")
}

func extractGitHubPRNumber(payload map[string]any) int {
	if number := intFromAny(payload["number"]); number > 0 {
		return number
	}
	if pr, ok := payload["pull_request"].(map[string]any); ok {
		if number := intFromAny(pr["number"]); number > 0 {
			return number
		}
	}
	return 0
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(typed))
		return n
	default:
		return 0
	}
}
