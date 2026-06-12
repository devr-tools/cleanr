package trends

import (
	"fmt"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func RenderAnalysisText(analysis Analysis) string {
	var b strings.Builder
	writeAnalysisHeader(&b, analysis)
	writeAnalysisDelta(&b, analysis)
	writeAnalysisBuildDiff(&b, analysis)
	writeAnalysisDrift(&b, analysis)
	writeAnalysisLoad(&b, analysis)
	writeSuiteTrendSection(&b, "Regressions", analysis.Regressions)
	writeCaseTrendSection(&b, "Case Regressions", analysis.CaseRegressions)
	writeSuiteTrendSection(&b, "Improvements", analysis.Improvements)
	writeCaseTrendSection(&b, "Case Improvements", analysis.CaseImprovements)
	writeFailureBucketSection(&b, analysis.FailureBuckets)
	writeRecentRunsSection(&b, analysis.RecentRuns)
	return b.String()
}

func writeAnalysisHeader(b *strings.Builder, analysis Analysis) {
	fmt.Fprintf(b, "Trend Summary\n")
	fmt.Fprintf(b, "=============\n")
	fmt.Fprintf(b, "Target        %s\n", analysis.Target)
	fmt.Fprintf(b, "RetainedRuns  %d\n", analysis.TotalRetainedRuns)
	fmt.Fprintf(b, "WindowSize    %d\n", analysis.WindowSize)
	if !analysis.OldestAt.IsZero() {
		fmt.Fprintf(b, "WindowStart   %s\n", analysis.OldestAt.Format("2006-01-02T15:04:05Z07:00"))
	}
	if !analysis.Latest.GeneratedAt.IsZero() {
		fmt.Fprintf(b, "LatestRun     %s", analysis.Latest.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"))
		if analysis.Latest.BuildID != "" {
			fmt.Fprintf(b, " (%s)", analysis.Latest.BuildID)
		}
		fmt.Fprintln(b)
	}
	fmt.Fprintf(b, "PassRate      %.2f\n", analysis.PassRate)
	fmt.Fprintf(b, "FailedRuns    %d\n", analysis.FailedRuns)
	fmt.Fprintf(b, "AvgDuration   %s\n", analysis.AverageDuration.Round(0).String())
	fmt.Fprintf(b, "LatestStatus  %s | failed_suites=%d | failed_cases=%d\n", passLabel(analysis.Latest.Passed), analysis.Latest.FailedSuites, analysis.Latest.FailedCases)
}

func writeAnalysisDelta(b *strings.Builder, analysis Analysis) {
	if analysis.Previous == nil {
		return
	}
	fmt.Fprintf(b, "\nLatest Delta\n")
	fmt.Fprintf(b, "------------\n")
	fmt.Fprintf(b, "Previous     %s", analysis.Previous.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"))
	if analysis.Previous.BuildID != "" {
		fmt.Fprintf(b, " (%s)", analysis.Previous.BuildID)
	}
	fmt.Fprintln(b)
	if analysis.Delta == nil {
		return
	}
	fmt.Fprintf(b, "FailedSuites %+d\n", analysis.Delta.FailedSuitesDelta)
	fmt.Fprintf(b, "FailedCases  %+d\n", analysis.Delta.FailedCasesDelta)
	fmt.Fprintf(b, "Duration     %s\n", analysis.Delta.DurationDelta.Round(0).String())
	fmt.Fprintf(b, "Regressions  %d\n", analysis.Delta.RegressedSuites)
	fmt.Fprintf(b, "Improvements %d\n", analysis.Delta.ImprovedSuites)
}

func writeAnalysisBuildDiff(b *strings.Builder, analysis Analysis) {
	if analysis.BuildDiff == nil {
		return
	}
	fmt.Fprintf(b, "\nBuild Changes\n")
	fmt.Fprintf(b, "-------------\n")
	if line := buildDiffHeaderLine(*analysis.BuildDiff); line != "" {
		fmt.Fprintf(b, "%s\n", line)
	}
	for _, change := range analysis.BuildDiff.ScenarioChanges {
		fmt.Fprintf(b, "- %s\n", scenarioDiffLine(change))
	}
}

func writeAnalysisDrift(b *strings.Builder, analysis Analysis) {
	if analysis.Drift == nil {
		return
	}
	fmt.Fprintf(b, "\nDrift Window\n")
	fmt.Fprintf(b, "------------\n")
	fmt.Fprintf(b, "AvgNormalizedDrift %.3f\n", analysis.Drift.AverageNormalizedDrift)
	fmt.Fprintf(b, "AvgSemanticDrift   %.3f\n", analysis.Drift.AverageSemanticDrift)
	fmt.Fprintf(b, "MaxNormalizedDrift %.3f\n", analysis.Drift.MaxNormalizedDrift)
	fmt.Fprintf(b, "MaxSemanticDrift   %.3f\n", analysis.Drift.MaxSemanticDrift)
	fmt.Fprintf(b, "LatestSemantic     %.3f\n", analysis.Drift.LatestSemanticDrift)
}

func writeAnalysisLoad(b *strings.Builder, analysis Analysis) {
	if analysis.Load == nil {
		return
	}
	fmt.Fprintf(b, "\nLoad Window\n")
	fmt.Fprintf(b, "-----------\n")
	fmt.Fprintf(b, "Runs              %d\n", analysis.Load.Runs)
	fmt.Fprintf(b, "AvgErrorRatePct   %.3f\n", analysis.Load.AverageErrorRatePct)
	fmt.Fprintf(b, "AvgP50LatencyMS   %.3f\n", analysis.Load.AverageP50LatencyMS)
	fmt.Fprintf(b, "AvgP95LatencyMS   %.3f\n", analysis.Load.AverageP95LatencyMS)
	fmt.Fprintf(b, "AvgP99LatencyMS   %.3f\n", analysis.Load.AverageP99LatencyMS)
	fmt.Fprintf(b, "AvgThroughputRPS  %.3f\n", analysis.Load.AverageThroughputRPS)
	fmt.Fprintf(b, "LatestP95Latency  %dms\n", analysis.Load.LatestP95LatencyMS)
	fmt.Fprintf(b, "LatestP99Latency  %dms\n", analysis.Load.LatestP99LatencyMS)
	fmt.Fprintf(b, "LatestThroughput  %.3f rps\n", analysis.Load.LatestThroughputRPS)
}

func writeSuiteTrendSection(b *strings.Builder, title string, suites []core.SuiteTrend) {
	if len(suites) == 0 {
		return
	}
	writeAnalysisSectionHeader(b, title)
	for _, suite := range suites {
		fmt.Fprintf(b, "- %s\n", suiteLine(suite))
	}
}

func writeCaseTrendSection(b *strings.Builder, title string, cases []core.CaseTrend) {
	if len(cases) == 0 {
		return
	}
	writeAnalysisSectionHeader(b, title)
	for _, c := range cases {
		fmt.Fprintf(b, "- %s\n", caseLine(c))
	}
}

func writeFailureBucketSection(b *strings.Builder, buckets []core.FailureBucket) {
	if len(buckets) == 0 {
		return
	}
	writeAnalysisSectionHeader(b, "Failure Buckets")
	for _, bucket := range buckets {
		fmt.Fprintf(b, "- %s\n", failureBucketLine(bucket))
	}
}

func writeRecentRunsSection(b *strings.Builder, runs []RunSnapshot) {
	if len(runs) == 0 {
		return
	}
	writeAnalysisSectionHeader(b, "Recent Runs")
	for _, run := range runs {
		fmt.Fprintf(b, "- %s", run.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"))
		if run.BuildID != "" {
			fmt.Fprintf(b, " %s", run.BuildID)
		}
		fmt.Fprintf(b, " %s failed_suites=%d failed_cases=%d duration=%s\n", passLabel(run.Passed), run.FailedSuites, run.FailedCases, run.Duration.Round(0).String())
	}
}

func writeAnalysisSectionHeader(b *strings.Builder, title string) {
	fmt.Fprintf(b, "\n%s\n", title)
	fmt.Fprintf(b, "%s\n", strings.Repeat("-", len(title)))
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
