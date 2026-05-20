package trends

import (
	"math"

	"cleanr/cleanr/core"
)

func Compare(current HistoryRun, previous *HistoryRun, historyLength int) *core.TrendReport {
	trend := &core.TrendReport{
		Baseline:       previous == nil,
		HistoryLength:  historyLength,
		CurrentBuildID: current.BuildID,
	}
	if previous == nil {
		return trend
	}

	trend.PreviousBuildID = previous.BuildID
	trend.PreviousAt = previous.GeneratedAt
	trend.PreviousDuration = previous.Duration
	trend.Summary = core.TrendSummary{
		FailedSuitesDelta: current.FailedSuites - previous.FailedSuites,
		FailedCasesDelta:  current.FailedCases - previous.FailedCases,
		DurationDelta:     current.Duration - previous.Duration,
	}

	previousByName := make(map[string]HistorySuite, len(previous.Suites))
	for _, suite := range previous.Suites {
		previousByName[suite.Name] = suite
	}

	trend.Suites = make([]core.SuiteTrend, 0, len(current.Suites))
	for _, suite := range current.Suites {
		prevSuite, ok := previousByName[suite.Name]
		if !ok {
			trend.Suites = append(trend.Suites, core.SuiteTrend{
				Name:             suite.Name,
				Status:           "new",
				FailedCasesDelta: suite.FailedCases,
				ScoreDelta:       suite.AverageScore,
				Drift:            compareDrift(nil, suite.Drift),
			})
			continue
		}
		status := suiteTrendStatus(prevSuite.Passed, suite.Passed)
		if status == "regressed" {
			trend.Summary.RegressedSuites++
		}
		if status == "improved" {
			trend.Summary.ImprovedSuites++
		}
		trend.Suites = append(trend.Suites, core.SuiteTrend{
			Name:             suite.Name,
			Status:           status,
			FailedCasesDelta: suite.FailedCases - prevSuite.FailedCases,
			ScoreDelta:       roundDelta(suite.AverageScore - prevSuite.AverageScore),
			Drift:            compareDrift(prevSuite.Drift, suite.Drift),
		})
	}

	return trend
}

func suiteTrendStatus(previousPassed, currentPassed bool) string {
	switch {
	case previousPassed && !currentPassed:
		return "regressed"
	case !previousPassed && currentPassed:
		return "improved"
	default:
		return "unchanged"
	}
}

func compareDrift(previous, current *HistoryDriftMetrics) *core.DriftTrend {
	if previous == nil && current == nil {
		return nil
	}
	if current == nil {
		return nil
	}
	drift := &core.DriftTrend{
		NormalizedDriftDelta:          roundDelta(current.NormalizedDrift),
		SemanticDriftDelta:            roundDelta(current.SemanticDrift),
		ConsistencyScoreDelta:         roundDelta(current.ConsistencyScore),
		SemanticConsistencyScoreDelta: roundDelta(current.SemanticConsistencyScore),
		BaselineDriftDelta:            roundDelta(current.BaselineDrift),
		BaselineSemanticDriftDelta:    roundDelta(current.BaselineSemanticDrift),
	}
	if previous != nil {
		drift.NormalizedDriftDelta = roundDelta(current.NormalizedDrift - previous.NormalizedDrift)
		drift.SemanticDriftDelta = roundDelta(current.SemanticDrift - previous.SemanticDrift)
		drift.ConsistencyScoreDelta = roundDelta(current.ConsistencyScore - previous.ConsistencyScore)
		drift.SemanticConsistencyScoreDelta = roundDelta(current.SemanticConsistencyScore - previous.SemanticConsistencyScore)
		drift.BaselineDriftDelta = roundDelta(current.BaselineDrift - previous.BaselineDrift)
		drift.BaselineSemanticDriftDelta = roundDelta(current.BaselineSemanticDrift - previous.BaselineSemanticDrift)
	}
	if drift.NormalizedDriftDelta == 0 &&
		drift.SemanticDriftDelta == 0 &&
		drift.ConsistencyScoreDelta == 0 &&
		drift.SemanticConsistencyScoreDelta == 0 &&
		drift.BaselineDriftDelta == 0 &&
		drift.BaselineSemanticDriftDelta == 0 {
		return nil
	}
	return drift
}

func roundDelta(v float64) float64 {
	return math.Round(v*1000) / 1000
}
