package tests

import (
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
)

// Explicit zero thresholds must survive default application instead of being
// silently replaced by the default value.
func TestExplicitZeroDriftThresholdSurvivesLoad(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Suites.Drift.MaxNormalizedDrift = float64Ptr(0)
	cfg.Suites.Drift.MinConsistencyScore = float64Ptr(0)

	data, err := cleanr.MarshalConfig(cfg, "json")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	loaded, err := cleanr.LoadConfigData(data, "json")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := loaded.Suites.Drift.MaxNormalizedDriftValue(); got != 0 {
		t.Fatalf("explicit max_normalized_drift 0 replaced by %v", got)
	}
	if got := loaded.Suites.Drift.MinConsistencyScoreValue(); got != 0 {
		t.Fatalf("explicit min_consistency_score 0 replaced by %v", got)
	}
}

func TestUnsetDriftThresholdsUseDefaults(t *testing.T) {
	var drift cleanr.DriftConfig
	if got := drift.MaxNormalizedDriftValue(); got != 0.3 {
		t.Fatalf("expected default 0.3, got %v", got)
	}
	if got := drift.MaxSnapshotDriftValue(); got != 0.3 {
		t.Fatalf("expected snapshot default to follow normalized default, got %v", got)
	}
	drift.MaxNormalizedDrift = float64Ptr(0.1)
	if got := drift.MaxSnapshotDriftValue(); got != 0.1 {
		t.Fatalf("expected snapshot default to follow explicit normalized value, got %v", got)
	}
}

// An explicit require_review: false must be expressible; it used to be
// silently inverted back to true by applyDefaults.
func TestExplicitRequireReviewFalseSurvivesLoad(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.ScenarioGeneration = cleanr.ScenarioGenerationConfig{
		Enabled: true,
		Provider: cleanr.TargetConfig{
			Type:          "http",
			URL:           "https://generator.example.test/v1",
			Method:        "POST",
			PromptField:   "input",
			ResponseField: "output.text",
		},
		Spec: cleanr.ScenarioGenerationSpec{
			AppKind:   "support-assistant",
			Goals:     []string{"refund policy"},
			RiskAreas: []string{"prompt injection"},
		},
		Count:         1,
		RequireReview: boolPtr(false),
	}

	data, err := cleanr.MarshalConfig(cfg, "json")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	loaded, err := cleanr.LoadConfigData(data, "json")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if loaded.ScenarioGeneration.RequireReviewValue() {
		t.Fatalf("explicit require_review: false was inverted to true")
	}
	if (cleanr.ScenarioGenerationConfig{}).RequireReviewValue() != true {
		t.Fatalf("unset require_review must default to true")
	}
}

// The exploratory preset makes gates non-blocking by default, but an explicit
// enabled: true must keep the relaxed thresholds with gating active.
func TestExploratoryPresetRespectsExplicitEnabled(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Reporting.TrendFile = "reports/history.yaml"
	cfg.Reporting.TrendGates = cleanr.TrendGateConfig{
		Preset:         "exploratory",
		Enabled:        boolPtr(true),
		RequiredWindow: 2,
	}

	data, err := cleanr.MarshalConfig(cfg, "json")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	loaded, err := cleanr.LoadConfigData(data, "json")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	gates := loaded.Reporting.TrendGates
	if !gates.EnabledValue() {
		t.Fatalf("exploratory preset overrode explicit enabled: true: %+v", gates)
	}
	if gates.MaxDurationIncreasePct == nil || *gates.MaxDurationIncreasePct != 50 {
		t.Fatalf("expected exploratory thresholds to apply, got %+v", gates)
	}

	// Without an explicit enabled, exploratory stays non-blocking.
	cfg.Reporting.TrendGates = cleanr.TrendGateConfig{Preset: "exploratory"}
	data, err = cleanr.MarshalConfig(cfg, "json")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	loaded, err = cleanr.LoadConfigData(data, "json")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.Reporting.TrendGates.EnabledValue() {
		t.Fatalf("exploratory preset without explicit enabled must stay non-blocking")
	}
}
