package trends

import (
	"fmt"
	"math"
	"time"

	"cleanr/cleanr/core"
)

func EvaluateGates(report *core.Report, cfg core.TrendGateConfig) {
	if report == nil || !cfg.Enabled {
		return
	}

	gate := &core.TrendGateReport{
		Enabled:        true,
		Passed:         true,
		RequiredWindow: cfg.RequiredWindow,
		GeneratedAt:    time.Now().UTC(),
	}
	report.TrendGate = gate

	if report.Trend == nil {
		gate.Passed = false
		gate.Findings = append(gate.Findings, core.Finding{
			Severity: "high",
			Message:  "trend gates could not be evaluated because no trend report was attached",
		})
		report.Passed = false
		report.Recommendations = append(report.Recommendations, "Attach trend history before enabling trend gates so CI can compare builds against prior retained runs.")
		return
	}

	gate.AvailableWindow = report.Trend.HistoryLength
	if report.Trend.HistoryLength < cfg.RequiredWindow || report.Trend.Baseline {
		return
	}

	gate.Evaluated = true
	breach := func(severity, message string) {
		gate.Passed = false
		gate.Findings = append(gate.Findings, core.Finding{Severity: severity, Message: message})
	}

	if cfg.MaxFailedSuitesDelta != nil && report.Trend.Summary.FailedSuitesDelta > *cfg.MaxFailedSuitesDelta {
		breach("high", fmt.Sprintf("failed suites delta %d exceeded gate %d", report.Trend.Summary.FailedSuitesDelta, *cfg.MaxFailedSuitesDelta))
	}
	if cfg.MaxFailedCasesDelta != nil && report.Trend.Summary.FailedCasesDelta > *cfg.MaxFailedCasesDelta {
		breach("high", fmt.Sprintf("failed cases delta %d exceeded gate %d", report.Trend.Summary.FailedCasesDelta, *cfg.MaxFailedCasesDelta))
	}
	if cfg.FailOnRegressedSuites && report.Trend.Summary.RegressedSuites > 0 {
		breach("high", fmt.Sprintf("regressed suites %d exceeded gate 0", report.Trend.Summary.RegressedSuites))
	}
	if cfg.MaxDurationIncreasePct != nil && report.Trend.PreviousDuration > 0 && report.Trend.Summary.DurationDelta > 0 {
		durationPct := (float64(report.Trend.Summary.DurationDelta) / float64(report.Trend.PreviousDuration)) * 100
		if durationPct > *cfg.MaxDurationIncreasePct {
			breach("medium", fmt.Sprintf("duration increase %.1f%% exceeded gate %.1f%%", round1(durationPct), *cfg.MaxDurationIncreasePct))
		}
	}

	drift := findDriftTrend(report.Trend)
	if drift != nil {
		if cfg.MaxSemanticDriftDelta != nil && drift.SemanticDriftDelta > *cfg.MaxSemanticDriftDelta {
			breach("high", fmt.Sprintf("semantic drift delta %.3f exceeded gate %.3f", drift.SemanticDriftDelta, *cfg.MaxSemanticDriftDelta))
		}
		if cfg.MaxBaselineSemanticDriftDelta != nil && drift.BaselineSemanticDriftDelta > *cfg.MaxBaselineSemanticDriftDelta {
			breach("high", fmt.Sprintf("baseline semantic drift delta %.3f exceeded gate %.3f", drift.BaselineSemanticDriftDelta, *cfg.MaxBaselineSemanticDriftDelta))
		}
	}

	if !gate.Passed {
		report.Passed = false
		report.Recommendations = append(report.Recommendations, "Investigate build-over-build regressions or relax trend gate thresholds if the current CI budget is intentionally higher.")
	}
}

func findDriftTrend(trend *core.TrendReport) *core.DriftTrend {
	if trend == nil {
		return nil
	}
	for _, suite := range trend.Suites {
		if suite.Name == "drift" && suite.Drift != nil {
			return suite.Drift
		}
	}
	return nil
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
