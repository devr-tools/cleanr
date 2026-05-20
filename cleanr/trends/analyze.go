package trends

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"cleanr/cleanr/core"
)

func Analyze(history HistoryFile, window int) Analysis {
	totalRuns := len(history.Runs)
	if totalRuns == 0 {
		return Analysis{
			Version:           history.Version,
			Target:            history.Target,
			TotalRetainedRuns: 0,
		}
	}

	start := 0
	if window > 0 && window < totalRuns {
		start = totalRuns - window
	}
	selected := history.Runs[start:]
	latest := selected[len(selected)-1]
	analysis := Analysis{
		Version:           history.Version,
		Target:            history.Target,
		TotalRetainedRuns: totalRuns,
		WindowSize:        len(selected),
		OldestAt:          selected[0].GeneratedAt,
		Latest:            buildSnapshot(latest),
		FailureBuckets:    buildFailureBuckets(latest),
		RecentRuns:        buildRecentSnapshots(selected),
	}

	if len(selected) > 1 {
		previous := selected[len(selected)-2]
		previousSnapshot := buildSnapshot(previous)
		analysis.Previous = &previousSnapshot
		comparison := Compare(latest, &previous, len(selected))
		analysis.Delta = &AnalysisDelta{
			FailedSuitesDelta: comparison.Summary.FailedSuitesDelta,
			FailedCasesDelta:  comparison.Summary.FailedCasesDelta,
			DurationDelta:     comparison.Summary.DurationDelta,
			RegressedSuites:   comparison.Summary.RegressedSuites,
			ImprovedSuites:    comparison.Summary.ImprovedSuites,
		}
		analysis.Regressions = filterSuiteTrends(comparison.Suites, "regressed")
		analysis.Improvements = filterSuiteTrends(comparison.Suites, "improved")
		analysis.CaseRegressions = comparison.CaseRegressions
		analysis.CaseImprovements = comparison.CaseImprovements
		analysis.FailureBuckets = comparison.FailureBuckets
	}

	totalDuration := int64(0)
	passedRuns := 0
	failedRuns := 0
	analysis.Drift = analyzeDriftWindow(selected)
	for _, run := range selected {
		totalDuration += int64(run.Duration)
		if run.Passed {
			passedRuns++
		} else {
			failedRuns++
		}
	}
	analysis.FailedRuns = failedRuns
	analysis.PassRate = round3(float64(passedRuns) / float64(len(selected)))
	analysis.AverageDuration = averageDuration(totalDuration, len(selected))
	return analysis
}

func AnalyzeFile(path string, window int) (Analysis, error) {
	history, err := LoadFile(path)
	if err != nil {
		return Analysis{}, err
	}
	return Analyze(history, window), nil
}

func WriteAnalysis(w io.Writer, analysis Analysis, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		_, err := io.WriteString(w, RenderAnalysisText(analysis))
		return err
	case "json":
		enc := json.NewEncoder(w)
		return enc.Encode(analysis)
	default:
		return fmt.Errorf("unsupported trends format: %s", format)
	}
}

func buildSnapshot(run HistoryRun) RunSnapshot {
	return RunSnapshot{
		BuildID:      run.BuildID,
		GeneratedAt:  run.GeneratedAt,
		Passed:       run.Passed,
		FailedSuites: run.FailedSuites,
		FailedCases:  run.FailedCases,
		Duration:     run.Duration,
	}
}

func buildRecentSnapshots(runs []HistoryRun) []RunSnapshot {
	out := make([]RunSnapshot, 0, len(runs))
	for _, run := range runs {
		out = append(out, buildSnapshot(run))
	}
	return out
}

func filterSuiteTrends(suites []core.SuiteTrend, status string) []core.SuiteTrend {
	out := make([]core.SuiteTrend, 0)
	for _, suite := range suites {
		if suite.Status == status {
			out = append(out, suite)
		}
	}
	return out
}

func analyzeDriftWindow(runs []HistoryRun) *DriftWindow {
	driftRuns := 0
	summary := &DriftWindow{}
	for _, run := range runs {
		suite, ok := findSuite(run, "drift")
		if !ok || suite.Drift == nil {
			continue
		}
		driftRuns++
		summary.AverageNormalizedDrift += suite.Drift.NormalizedDrift
		summary.AverageSemanticDrift += suite.Drift.SemanticDrift
		summary.AverageConsistencyScore += suite.Drift.ConsistencyScore
		summary.AverageSemanticConsistency += suite.Drift.SemanticConsistencyScore
		if suite.Drift.NormalizedDrift > summary.MaxNormalizedDrift {
			summary.MaxNormalizedDrift = suite.Drift.NormalizedDrift
		}
		if suite.Drift.SemanticDrift > summary.MaxSemanticDrift {
			summary.MaxSemanticDrift = suite.Drift.SemanticDrift
		}
	}
	if driftRuns == 0 {
		return nil
	}
	latestSuite, _ := findSuite(runs[len(runs)-1], "drift")
	if latestSuite.Drift != nil {
		summary.LatestNormalizedDrift = latestSuite.Drift.NormalizedDrift
		summary.LatestSemanticDrift = latestSuite.Drift.SemanticDrift
		summary.LatestBaselineDrift = latestSuite.Drift.BaselineDrift
		summary.LatestBaselineSemanticDrift = latestSuite.Drift.BaselineSemanticDrift
	}
	divisor := float64(driftRuns)
	summary.AverageNormalizedDrift = round3(summary.AverageNormalizedDrift / divisor)
	summary.AverageSemanticDrift = round3(summary.AverageSemanticDrift / divisor)
	summary.AverageConsistencyScore = round3(summary.AverageConsistencyScore / divisor)
	summary.AverageSemanticConsistency = round3(summary.AverageSemanticConsistency / divisor)
	summary.MaxNormalizedDrift = round3(summary.MaxNormalizedDrift)
	summary.MaxSemanticDrift = round3(summary.MaxSemanticDrift)
	summary.LatestNormalizedDrift = round3(summary.LatestNormalizedDrift)
	summary.LatestSemanticDrift = round3(summary.LatestSemanticDrift)
	summary.LatestBaselineDrift = round3(summary.LatestBaselineDrift)
	summary.LatestBaselineSemanticDrift = round3(summary.LatestBaselineSemanticDrift)
	return summary
}

func findSuite(run HistoryRun, name string) (HistorySuite, bool) {
	for _, suite := range run.Suites {
		if suite.Name == name {
			return suite, true
		}
	}
	return HistorySuite{}, false
}

func averageDuration(total int64, count int) time.Duration {
	if count == 0 {
		return 0
	}
	return time.Duration(total / int64(count))
}
