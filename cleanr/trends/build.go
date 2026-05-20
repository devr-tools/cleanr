package trends

import (
	"math"

	"cleanr/cleanr/core"
)

func buildSuite(suite core.SuiteResult) HistorySuite {
	historySuite := HistorySuite{
		Name:        suite.Name,
		Passed:      suite.Passed,
		FailedCases: countFailedCases(suite.Cases),
	}

	scoreTotal := 0.0
	scoreCount := 0
	for _, c := range suite.Cases {
		if c.Score > 0 {
			scoreTotal += c.Score
			scoreCount++
		}
	}
	if scoreCount > 0 {
		historySuite.AverageScore = round3(scoreTotal / float64(scoreCount))
	}

	if suite.Name == "drift" {
		historySuite.Drift = summarizeDriftSuite(suite)
	}
	return historySuite
}

func summarizeDriftSuite(suite core.SuiteResult) *HistoryDriftMetrics {
	metrics := &HistoryDriftMetrics{}
	for _, c := range suite.Cases {
		metrics.Cases++
		metrics.NormalizedDrift += detailFloat(c.Details, "normalized_drift")
		metrics.SemanticDrift += detailFloat(c.Details, "semantic_drift")
		metrics.ConsistencyScore += detailFloat(c.Details, "consistency_score")
		metrics.SemanticConsistencyScore += detailFloat(c.Details, "semantic_consistency_score")
		metrics.BaselineDrift += detailFloat(c.Details, "baseline_drift")
		metrics.BaselineSemanticDrift += detailFloat(c.Details, "baseline_semantic_drift")
	}
	if metrics.Cases == 0 {
		return nil
	}
	divisor := float64(metrics.Cases)
	metrics.NormalizedDrift = round3(metrics.NormalizedDrift / divisor)
	metrics.SemanticDrift = round3(metrics.SemanticDrift / divisor)
	metrics.ConsistencyScore = round3(metrics.ConsistencyScore / divisor)
	metrics.SemanticConsistencyScore = round3(metrics.SemanticConsistencyScore / divisor)
	metrics.BaselineDrift = round3(metrics.BaselineDrift / divisor)
	metrics.BaselineSemanticDrift = round3(metrics.BaselineSemanticDrift / divisor)
	return metrics
}

func detailFloat(details map[string]any, key string) float64 {
	if len(details) == 0 {
		return 0
	}
	value, ok := details[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func countFailedCases(cases []core.CaseResult) int {
	failed := 0
	for _, c := range cases {
		if !c.Passed {
			failed++
		}
	}
	return failed
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
