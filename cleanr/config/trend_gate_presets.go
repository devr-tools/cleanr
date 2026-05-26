package config

import (
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

const (
	trendGatePresetStrict      = "strict"
	trendGatePresetModerate    = "moderate"
	trendGatePresetExploratory = "exploratory"
)

func applyTrendGatePreset(gates *core.TrendGateConfig) {
	if gates == nil {
		return
	}
	switch normalizeTrendGatePreset(gates.Preset) {
	case trendGatePresetStrict:
		gates.Preset = trendGatePresetStrict
		gates.Enabled = true
		if gates.RequiredWindow == 0 {
			gates.RequiredWindow = 2
		}
		if gates.MaxFailedSuitesDelta == nil {
			gates.MaxFailedSuitesDelta = intPtr(0)
		}
		if gates.MaxFailedCasesDelta == nil {
			gates.MaxFailedCasesDelta = intPtr(0)
		}
		if gates.MaxDurationIncreasePct == nil {
			gates.MaxDurationIncreasePct = float64Ptr(15)
		}
		if gates.MaxSemanticDriftDelta == nil {
			gates.MaxSemanticDriftDelta = float64Ptr(0.05)
		}
		if gates.MaxBaselineSemanticDriftDelta == nil {
			gates.MaxBaselineSemanticDriftDelta = float64Ptr(0.03)
		}
		gates.FailOnRegressedSuites = true
	case trendGatePresetModerate:
		gates.Preset = trendGatePresetModerate
		gates.Enabled = true
		if gates.RequiredWindow == 0 {
			gates.RequiredWindow = 2
		}
		if gates.MaxFailedSuitesDelta == nil {
			gates.MaxFailedSuitesDelta = intPtr(0)
		}
		if gates.MaxFailedCasesDelta == nil {
			gates.MaxFailedCasesDelta = intPtr(0)
		}
		if gates.MaxDurationIncreasePct == nil {
			gates.MaxDurationIncreasePct = float64Ptr(25)
		}
		if gates.MaxSemanticDriftDelta == nil {
			gates.MaxSemanticDriftDelta = float64Ptr(0.08)
		}
		if gates.MaxBaselineSemanticDriftDelta == nil {
			gates.MaxBaselineSemanticDriftDelta = float64Ptr(0.05)
		}
		gates.FailOnRegressedSuites = true
	case trendGatePresetExploratory:
		gates.Preset = trendGatePresetExploratory
		gates.Enabled = false
		if gates.RequiredWindow == 0 {
			gates.RequiredWindow = 2
		}
		if gates.MaxDurationIncreasePct == nil {
			gates.MaxDurationIncreasePct = float64Ptr(50)
		}
	}
}

func normalizeTrendGatePreset(preset string) string {
	return strings.ToLower(strings.TrimSpace(preset))
}

func isValidTrendGatePreset(preset string) bool {
	switch normalizeTrendGatePreset(preset) {
	case "", trendGatePresetStrict, trendGatePresetModerate, trendGatePresetExploratory:
		return true
	default:
		return false
	}
}

func intPtr(v int) *int {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}
