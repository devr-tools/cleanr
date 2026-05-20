package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"cleanr/cleanr"
)

type trendStableTarget struct{}

func (trendStableTarget) Invoke(context.Context, cleanr.Request) cleanr.Response {
	return cleanr.Response{
		StatusCode: 200,
		Text:       "stable answer",
	}
}

func TestTrendHistoryRetainsConfiguredLimit(t *testing.T) {
	report := cleanr.NewRunner(cleanTrendConfig(), trendStableTarget{}).Run(context.Background())
	path := t.TempDir() + "/trends.yaml"

	if err := cleanr.AttachTrendHistory(&report, path, "build-1", 2); err != nil {
		t.Fatalf("attach trend history: %v", err)
	}
	if err := cleanr.AttachTrendHistory(&report, path, "build-2", 2); err != nil {
		t.Fatalf("attach trend history: %v", err)
	}
	if err := cleanr.AttachTrendHistory(&report, path, "build-3", 2); err != nil {
		t.Fatalf("attach trend history: %v", err)
	}

	history, err := cleanr.LoadTrendHistoryFile(path)
	if err != nil {
		t.Fatalf("load trend history: %v", err)
	}
	if len(history.Runs) != 2 {
		t.Fatalf("expected 2 retained runs, got %+v", history)
	}
	if history.Runs[0].BuildID != "build-2" || history.Runs[1].BuildID != "build-3" {
		t.Fatalf("unexpected retained runs: %+v", history.Runs)
	}
}

func TestAnalyzeTrendHistoryBuildsWindowSummary(t *testing.T) {
	history := cleanr.TrendHistoryFile{
		Version: "v1alpha1",
		Target:  "assistant-api",
		Runs: []cleanr.TrendHistoryRun{
			{
				BuildID:      "build-1",
				GeneratedAt:  time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
				Passed:       true,
				Duration:     2 * time.Second,
				FailedSuites: 0,
				FailedCases:  0,
				Suites: []cleanr.HistorySuite{
					{Name: "drift", Passed: true, Drift: &cleanr.HistoryDriftMetrics{NormalizedDrift: 0.05, SemanticDrift: 0.02, ConsistencyScore: 0.95, SemanticConsistencyScore: 0.98}},
				},
			},
			{
				BuildID:      "build-2",
				GeneratedAt:  time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
				Passed:       false,
				Duration:     3 * time.Second,
				FailedSuites: 1,
				FailedCases:  2,
				Suites: []cleanr.HistorySuite{
					{Name: "drift", Passed: false, FailedCases: 1, AverageScore: 0.75, Drift: &cleanr.HistoryDriftMetrics{NormalizedDrift: 0.25, SemanticDrift: 0.14, ConsistencyScore: 0.76, SemanticConsistencyScore: 0.83, BaselineSemanticDrift: 0.11}},
				},
			},
		},
	}
	path := t.TempDir() + "/trends.json"
	if err := cleanr.WriteTrendHistoryFile(path, history); err != nil {
		t.Fatalf("write trend history: %v", err)
	}

	analysis, err := cleanr.AnalyzeTrendHistoryFile(path, 2)
	if err != nil {
		t.Fatalf("analyze trend history: %v", err)
	}
	if analysis.Target != "assistant-api" || analysis.WindowSize != 2 {
		t.Fatalf("unexpected analysis header: %+v", analysis)
	}
	if analysis.Delta == nil || analysis.Delta.RegressedSuites != 1 {
		t.Fatalf("expected regression delta: %+v", analysis)
	}
	if len(analysis.Regressions) != 1 || analysis.Regressions[0].Name != "drift" {
		t.Fatalf("expected drift regression: %+v", analysis.Regressions)
	}
	if analysis.Drift == nil || analysis.Drift.AverageSemanticDrift != 0.08 {
		t.Fatalf("unexpected drift summary: %+v", analysis.Drift)
	}

	var text bytes.Buffer
	if err := cleanr.WriteTrendAnalysis(&text, analysis, "text"); err != nil {
		t.Fatalf("write text analysis: %v", err)
	}
	if !strings.Contains(text.String(), "Trend Summary") || !strings.Contains(text.String(), "Regressions") {
		t.Fatalf("unexpected text analysis: %s", text.String())
	}

	var jsonBuf bytes.Buffer
	if err := cleanr.WriteTrendAnalysis(&jsonBuf, analysis, "json"); err != nil {
		t.Fatalf("write json analysis: %v", err)
	}
	var decoded cleanr.TrendAnalysis
	if err := json.Unmarshal(jsonBuf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json analysis: %v", err)
	}
	if decoded.PassRate != 0.5 {
		t.Fatalf("unexpected decoded pass rate: %+v", decoded)
	}
}

func cleanTrendConfig() cleanr.Config {
	cfg := cleanr.ExampleConfig()
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Suites.Drift.Enabled = true
	cfg.Suites.Drift.Iterations = 2
	cfg.Suites.Drift.MaxNormalizedDrift = 1
	cfg.Suites.Drift.MaxSemanticDrift = 1
	cfg.Suites.Drift.MinConsistencyScore = 0
	cfg.Suites.Drift.MinSemanticConsistencyScore = 0
	return cfg
}
