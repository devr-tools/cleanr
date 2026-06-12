package trends

import (
	"math"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func buildSuite(suite core.SuiteResult) HistorySuite {
	historySuite := HistorySuite{
		Name:        suite.Name,
		Passed:      suite.Passed,
		FailedCases: countFailedCases(suite.Cases),
	}
	if shouldRetainCaseEvidence(suite.Name) {
		historySuite.Cases = make([]HistoryCase, 0, len(suite.Cases))
		for _, c := range suite.Cases {
			evidence := buildCaseEvidence(suite.Name, c)
			if evidence != nil {
				historySuite.Cases = append(historySuite.Cases, *evidence)
			}
		}
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
	if suite.Name == "load" {
		historySuite.Load = summarizeLoadSuite(suite)
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

func summarizeLoadSuite(suite core.SuiteResult) *HistoryLoadMetrics {
	if len(suite.Cases) == 0 {
		return nil
	}
	caseResult := suite.Cases[0]
	return &HistoryLoadMetrics{
		Requests:        detailInt(caseResult.Details, "requests"),
		VirtualUsers:    detailInt(caseResult.Details, "virtual_users"),
		RequestsPerUser: detailInt(caseResult.Details, "requests_per_user"),
		ScenarioCount:   detailInt(caseResult.Details, "scenario_count"),
		ErrorRatePct:    detailInt(caseResult.Details, "error_rate_pct"),
		P50LatencyMS:    detailInt64(caseResult.Details, "latency_p50_ms"),
		P95LatencyMS:    detailInt64(caseResult.Details, "latency_p95_ms"),
		P99LatencyMS:    detailInt64(caseResult.Details, "latency_p99_ms"),
		ThroughputRPS:   round3(detailFloat(caseResult.Details, "throughput_rps")),
	}
}

func detailFloat(details map[string]any, key string) float64 {
	return numericDetail(details, key)
}

func detailInt(details map[string]any, key string) int {
	return int(numericDetail(details, key))
}

func detailInt64(details map[string]any, key string) int64 {
	return int64(numericDetail(details, key))
}

func numericDetail(details map[string]any, key string) float64 {
	var value any
	if len(details) > 0 {
		value = details[key]
	}
	number := 0.0
	switch typed := value.(type) {
	case float64:
		number = typed
	case float32:
		number = float64(typed)
	case int:
		number = float64(typed)
	case int64:
		number = float64(typed)
	}
	return number
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
