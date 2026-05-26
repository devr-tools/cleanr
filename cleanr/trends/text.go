package trends

import (
	"fmt"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func RenderAnalysisText(analysis Analysis) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Trend Summary\n")
	fmt.Fprintf(&b, "=============\n")
	fmt.Fprintf(&b, "Target        %s\n", analysis.Target)
	fmt.Fprintf(&b, "RetainedRuns  %d\n", analysis.TotalRetainedRuns)
	fmt.Fprintf(&b, "WindowSize    %d\n", analysis.WindowSize)
	if !analysis.OldestAt.IsZero() {
		fmt.Fprintf(&b, "WindowStart   %s\n", analysis.OldestAt.Format("2006-01-02T15:04:05Z07:00"))
	}
	if !analysis.Latest.GeneratedAt.IsZero() {
		fmt.Fprintf(&b, "LatestRun     %s", analysis.Latest.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"))
		if analysis.Latest.BuildID != "" {
			fmt.Fprintf(&b, " (%s)", analysis.Latest.BuildID)
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintf(&b, "PassRate      %.2f\n", analysis.PassRate)
	fmt.Fprintf(&b, "FailedRuns    %d\n", analysis.FailedRuns)
	fmt.Fprintf(&b, "AvgDuration   %s\n", analysis.AverageDuration.Round(0).String())
	fmt.Fprintf(&b, "LatestStatus  %s | failed_suites=%d | failed_cases=%d\n", passLabel(analysis.Latest.Passed), analysis.Latest.FailedSuites, analysis.Latest.FailedCases)

	if analysis.Previous != nil {
		fmt.Fprintf(&b, "\nLatest Delta\n")
		fmt.Fprintf(&b, "------------\n")
		fmt.Fprintf(&b, "Previous     %s", analysis.Previous.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"))
		if analysis.Previous.BuildID != "" {
			fmt.Fprintf(&b, " (%s)", analysis.Previous.BuildID)
		}
		fmt.Fprintln(&b)
		if analysis.Delta != nil {
			fmt.Fprintf(&b, "FailedSuites %+d\n", analysis.Delta.FailedSuitesDelta)
			fmt.Fprintf(&b, "FailedCases  %+d\n", analysis.Delta.FailedCasesDelta)
			fmt.Fprintf(&b, "Duration     %s\n", analysis.Delta.DurationDelta.Round(0).String())
			fmt.Fprintf(&b, "Regressions  %d\n", analysis.Delta.RegressedSuites)
			fmt.Fprintf(&b, "Improvements %d\n", analysis.Delta.ImprovedSuites)
		}
	}
	if analysis.BuildDiff != nil {
		fmt.Fprintf(&b, "\nBuild Changes\n")
		fmt.Fprintf(&b, "-------------\n")
		if line := buildDiffHeaderLine(*analysis.BuildDiff); line != "" {
			fmt.Fprintf(&b, "%s\n", line)
		}
		for _, change := range analysis.BuildDiff.ScenarioChanges {
			fmt.Fprintf(&b, "- %s\n", scenarioDiffLine(change))
		}
	}

	if analysis.Drift != nil {
		fmt.Fprintf(&b, "\nDrift Window\n")
		fmt.Fprintf(&b, "------------\n")
		fmt.Fprintf(&b, "AvgNormalizedDrift %.3f\n", analysis.Drift.AverageNormalizedDrift)
		fmt.Fprintf(&b, "AvgSemanticDrift   %.3f\n", analysis.Drift.AverageSemanticDrift)
		fmt.Fprintf(&b, "MaxNormalizedDrift %.3f\n", analysis.Drift.MaxNormalizedDrift)
		fmt.Fprintf(&b, "MaxSemanticDrift   %.3f\n", analysis.Drift.MaxSemanticDrift)
		fmt.Fprintf(&b, "LatestSemantic     %.3f\n", analysis.Drift.LatestSemanticDrift)
	}

	if len(analysis.Regressions) > 0 {
		fmt.Fprintf(&b, "\nRegressions\n")
		fmt.Fprintf(&b, "-----------\n")
		for _, suite := range analysis.Regressions {
			fmt.Fprintf(&b, "- %s\n", suiteLine(suite))
		}
	}
	if len(analysis.CaseRegressions) > 0 {
		fmt.Fprintf(&b, "\nCase Regressions\n")
		fmt.Fprintf(&b, "----------------\n")
		for _, c := range analysis.CaseRegressions {
			fmt.Fprintf(&b, "- %s\n", caseLine(c))
		}
	}
	if len(analysis.Improvements) > 0 {
		fmt.Fprintf(&b, "\nImprovements\n")
		fmt.Fprintf(&b, "------------\n")
		for _, suite := range analysis.Improvements {
			fmt.Fprintf(&b, "- %s\n", suiteLine(suite))
		}
	}
	if len(analysis.CaseImprovements) > 0 {
		fmt.Fprintf(&b, "\nCase Improvements\n")
		fmt.Fprintf(&b, "-----------------\n")
		for _, c := range analysis.CaseImprovements {
			fmt.Fprintf(&b, "- %s\n", caseLine(c))
		}
	}
	if len(analysis.FailureBuckets) > 0 {
		fmt.Fprintf(&b, "\nFailure Buckets\n")
		fmt.Fprintf(&b, "---------------\n")
		for _, bucket := range analysis.FailureBuckets {
			fmt.Fprintf(&b, "- %s\n", failureBucketLine(bucket))
		}
	}

	if len(analysis.RecentRuns) > 0 {
		fmt.Fprintf(&b, "\nRecent Runs\n")
		fmt.Fprintf(&b, "----------\n")
		for _, run := range analysis.RecentRuns {
			fmt.Fprintf(&b, "- %s", run.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"))
			if run.BuildID != "" {
				fmt.Fprintf(&b, " %s", run.BuildID)
			}
			fmt.Fprintf(&b, " %s failed_suites=%d failed_cases=%d duration=%s\n", passLabel(run.Passed), run.FailedSuites, run.FailedCases, run.Duration.Round(0).String())
		}
	}

	return b.String()
}

func passLabel(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
}

func suiteLine(suite core.SuiteTrend) string {
	parts := []string{suite.Name, suite.Status}
	if suite.FailedCasesDelta != 0 {
		parts = append(parts, fmt.Sprintf("failed_cases_delta=%+d", suite.FailedCasesDelta))
	}
	if suite.ScoreDelta != 0 {
		parts = append(parts, fmt.Sprintf("score_delta=%+.3f", suite.ScoreDelta))
	}
	if suite.Drift != nil {
		if suite.Drift.SemanticDriftDelta != 0 {
			parts = append(parts, fmt.Sprintf("semantic_drift_delta=%+.3f", suite.Drift.SemanticDriftDelta))
		}
		if suite.Drift.NormalizedDriftDelta != 0 {
			parts = append(parts, fmt.Sprintf("normalized_drift_delta=%+.3f", suite.Drift.NormalizedDriftDelta))
		}
		if suite.Drift.BaselineSemanticDriftDelta != 0 {
			parts = append(parts, fmt.Sprintf("baseline_semantic_drift_delta=%+.3f", suite.Drift.BaselineSemanticDriftDelta))
		}
	}
	return strings.Join(parts, " | ")
}

func caseLine(trend core.CaseTrend) string {
	parts := []string{trend.Suite + "/" + trend.Name, trend.Status}
	if len(trend.NewFindingSignatures) > 0 {
		parts = append(parts, "new_findings="+compactItems(trend.NewFindingSignatures, 3))
	} else if len(trend.FindingSignatures) > 0 {
		parts = append(parts, "findings="+compactItems(trend.FindingSignatures, 3))
	}
	if trend.FirstUnsupportedClaim != "" {
		parts = append(parts, "first_unsupported_claim="+trend.FirstUnsupportedClaim)
	}
	if len(trend.ToolCalls) > 0 {
		parts = append(parts, "tools="+compactItems(trend.ToolCalls, 3))
	}
	if len(trend.StateChanges) > 0 {
		parts = append(parts, "state_changes="+compactItems(trend.StateChanges, 2))
	}
	if len(trend.FileChanges) > 0 {
		parts = append(parts, "file_changes="+compactItems(trend.FileChanges, 2))
	}
	if len(trend.MemoryMarkers) > 0 {
		parts = append(parts, "memory="+compactItems(trend.MemoryMarkers, 2))
	}
	return strings.Join(parts, " | ")
}

func failureBucketLine(bucket core.FailureBucket) string {
	parts := []string{bucket.Signature, fmt.Sprintf("cases=%d", bucket.Count)}
	if len(bucket.Cases) > 0 {
		parts = append(parts, "impacted="+compactItems(bucket.Cases, 3))
	}
	return strings.Join(parts, " | ")
}

func buildDiffHeaderLine(diff core.BuildDiff) string {
	parts := make([]string, 0, 2)
	if diff.TargetTypeBefore != "" || diff.TargetTypeAfter != "" {
		parts = append(parts, fmt.Sprintf("target_type=%s -> %s", emptyLabel(diff.TargetTypeBefore), emptyLabel(diff.TargetTypeAfter)))
	}
	if diff.ModelBefore != "" || diff.ModelAfter != "" {
		parts = append(parts, fmt.Sprintf("model=%s -> %s", emptyLabel(diff.ModelBefore), emptyLabel(diff.ModelAfter)))
	}
	return strings.Join(parts, " | ")
}

func scenarioDiffLine(change core.ScenarioDiff) string {
	parts := []string{change.Name, change.Status}
	if change.SystemChanged {
		parts = append(parts, "system")
	}
	if change.InputChanged {
		parts = append(parts, "input")
	}
	if change.ContextChanged {
		parts = append(parts, "context")
	}
	if change.MemoryReplayChanged {
		parts = append(parts, "memory_replay")
	}
	if change.TagsChanged {
		parts = append(parts, "tags")
	}
	return strings.Join(parts, " | ")
}

func emptyLabel(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<unset>"
	}
	return value
}

func compactItems(items []string, limit int) string {
	if len(items) == 0 {
		return ""
	}
	if limit <= 0 || len(items) <= limit {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s (+%d more)", strings.Join(items[:limit], ", "), len(items)-limit)
}
