package report

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"cleanr/cleanr/core"
)

func suiteSummaryText(suite core.SuiteResult) string {
	parts := make([]string, 0, 3)
	failedCases := 0
	for _, c := range suite.Cases {
		if !c.Passed {
			failedCases++
		}
	}
	if len(suite.Cases) > 0 {
		parts = append(parts, fmt.Sprintf("%d cases, %d failed", len(suite.Cases), failedCases))
	}
	if suite.Duration > 0 {
		parts = append(parts, suite.Duration.Round(time.Millisecond).String())
	}
	return strings.Join(parts, " | ")
}

func caseSummaryText(c core.CaseResult) string {
	parts := make([]string, 0, 8)
	if c.Duration > 0 {
		parts = append(parts, "duration "+c.Duration.Round(time.Millisecond).String())
	}
	if c.Score > 0 {
		parts = append(parts, fmt.Sprintf("score %.2f", c.Score))
	}
	if c.LatencyP95 > 0 {
		parts = append(parts, "p95 "+c.LatencyP95.Round(time.Millisecond).String())
	}
	parts = append(parts, scalarDetailParts(c.Details)...)
	return strings.Join(parts, " | ")
}

func suiteMetaText(meta map[string]any) string {
	return strings.Join(scalarDetailParts(meta), " | ")
}

func trendComparedText(trend core.TrendReport) string {
	if trend.PreviousBuildID != "" {
		return fmt.Sprintf("%s at %s", trend.PreviousBuildID, trend.PreviousAt.Format(time.RFC3339))
	}
	if !trend.PreviousAt.IsZero() {
		return trend.PreviousAt.Format(time.RFC3339)
	}
	return "previous recorded run"
}

func trendSummaryText(summary core.TrendSummary) string {
	parts := []string{
		fmt.Sprintf("failed_suites_delta=%+d", summary.FailedSuitesDelta),
		fmt.Sprintf("failed_cases_delta=%+d", summary.FailedCasesDelta),
		fmt.Sprintf("duration_delta=%s", summary.DurationDelta.Round(time.Millisecond).String()),
	}
	if summary.RegressedSuites > 0 {
		parts = append(parts, fmt.Sprintf("regressed_suites=%d", summary.RegressedSuites))
	}
	if summary.ImprovedSuites > 0 {
		parts = append(parts, fmt.Sprintf("improved_suites=%d", summary.ImprovedSuites))
	}
	return strings.Join(parts, " | ")
}

func suiteTrendText(trend core.SuiteTrend) string {
	parts := []string{trend.Status}
	if trend.FailedCasesDelta != 0 {
		parts = append(parts, fmt.Sprintf("failed_cases_delta=%+d", trend.FailedCasesDelta))
	}
	if trend.ScoreDelta != 0 {
		parts = append(parts, fmt.Sprintf("score_delta=%+.3f", trend.ScoreDelta))
	}
	if trend.Drift != nil {
		parts = append(parts, driftTrendParts(*trend.Drift)...)
	}
	return strings.Join(parts, " | ")
}

func driftTrendParts(trend core.DriftTrend) []string {
	parts := make([]string, 0, 6)
	if trend.NormalizedDriftDelta != 0 {
		parts = append(parts, fmt.Sprintf("normalized_drift_delta=%+.3f", trend.NormalizedDriftDelta))
	}
	if trend.SemanticDriftDelta != 0 {
		parts = append(parts, fmt.Sprintf("semantic_drift_delta=%+.3f", trend.SemanticDriftDelta))
	}
	if trend.ConsistencyScoreDelta != 0 {
		parts = append(parts, fmt.Sprintf("consistency_score_delta=%+.3f", trend.ConsistencyScoreDelta))
	}
	if trend.SemanticConsistencyScoreDelta != 0 {
		parts = append(parts, fmt.Sprintf("semantic_consistency_score_delta=%+.3f", trend.SemanticConsistencyScoreDelta))
	}
	if trend.BaselineDriftDelta != 0 {
		parts = append(parts, fmt.Sprintf("baseline_drift_delta=%+.3f", trend.BaselineDriftDelta))
	}
	if trend.BaselineSemanticDriftDelta != 0 {
		parts = append(parts, fmt.Sprintf("baseline_semantic_drift_delta=%+.3f", trend.BaselineSemanticDriftDelta))
	}
	return parts
}

func scalarDetailParts(values map[string]any) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if !isScalarReportValue(value) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, formatReportValue(values[key])))
	}
	return parts
}

func isScalarReportValue(value any) bool {
	switch value.(type) {
	case string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	default:
		return false
	}
}

func formatReportValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float32:
		return fmt.Sprintf("%.2f", v)
	case float64:
		return fmt.Sprintf("%.2f", v)
	default:
		return fmt.Sprint(v)
	}
}
