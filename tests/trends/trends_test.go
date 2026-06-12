package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
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

func TestAnalyzeTrendHistorySummarizesLoadWindow(t *testing.T) {
	history := cleanr.TrendHistoryFile{
		Version: "v1alpha1",
		Target:  "assistant-api",
		Runs: []cleanr.TrendHistoryRun{
			{
				BuildID:     "build-1",
				GeneratedAt: time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
				Passed:      true,
				Duration:    2 * time.Second,
				Suites: []cleanr.HistorySuite{{
					Name: "load",
					Load: &cleanr.HistoryLoadMetrics{
						Requests:        20,
						VirtualUsers:    4,
						RequestsPerUser: 5,
						ScenarioCount:   2,
						ErrorRatePct:    0,
						P50LatencyMS:    80,
						P95LatencyMS:    150,
						P99LatencyMS:    220,
						ThroughputRPS:   10,
					},
				}},
			},
			{
				BuildID:     "build-2",
				GeneratedAt: time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC),
				Passed:      true,
				Duration:    3 * time.Second,
				Suites: []cleanr.HistorySuite{{
					Name: "load",
					Load: &cleanr.HistoryLoadMetrics{
						Requests:        20,
						VirtualUsers:    4,
						RequestsPerUser: 5,
						ScenarioCount:   2,
						ErrorRatePct:    5,
						P50LatencyMS:    100,
						P95LatencyMS:    180,
						P99LatencyMS:    250,
						ThroughputRPS:   8,
					},
				}},
			},
		},
	}
	path := t.TempDir() + "/load-trends.yaml"
	if err := cleanr.WriteTrendHistoryFile(path, history); err != nil {
		t.Fatalf("write trend history: %v", err)
	}

	analysis, err := cleanr.AnalyzeTrendHistoryFile(path, 2)
	if err != nil {
		t.Fatalf("analyze trend history: %v", err)
	}
	if analysis.Load == nil {
		t.Fatalf("expected load window in analysis: %+v", analysis)
	}
	if analysis.Load.AverageP95LatencyMS != 165 || analysis.Load.LatestP99LatencyMS != 250 {
		t.Fatalf("unexpected load summary: %+v", analysis.Load)
	}

	var text bytes.Buffer
	if err := cleanr.WriteTrendAnalysis(&text, analysis, "text"); err != nil {
		t.Fatalf("write text analysis: %v", err)
	}
	for _, want := range []string{
		"Load Window",
		"AvgP95LatencyMS   165.000",
		"LatestP99Latency  250ms",
	} {
		if !strings.Contains(text.String(), want) {
			t.Fatalf("expected %q in load analysis, got %s", want, text.String())
		}
	}
}

func TestAnalyzeTrendHistoryReportsBuildPromptAndModelDiffs(t *testing.T) {
	history := cleanr.TrendHistoryFile{
		Version: "v1alpha1",
		Target:  "assistant-api",
		Runs: []cleanr.TrendHistoryRun{
			{
				BuildID:      "build-1",
				GeneratedAt:  time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC),
				Passed:       true,
				Duration:     2 * time.Second,
				FailedSuites: 0,
				FailedCases:  0,
				Metadata: &cleanr.RunMetadata{
					TargetType:    "openai",
					ProviderModel: "gpt-4.1-mini",
					ScenarioFingerprints: []cleanr.ScenarioFingerprint{
						{Name: "workflow-a", SystemHash: "sys-a", InputHash: "input-a", ContextHash: "ctx-a"},
						{Name: "workflow-b", SystemHash: "sys-b", InputHash: "input-b"},
					},
				},
				Suites: []cleanr.HistorySuite{{Name: "claim-trace", Passed: true}},
			},
			{
				BuildID:      "build-2",
				GeneratedAt:  time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
				Passed:       false,
				Duration:     3 * time.Second,
				FailedSuites: 1,
				FailedCases:  1,
				Metadata: &cleanr.RunMetadata{
					TargetType:    "openai",
					ProviderModel: "gpt-4.1",
					ScenarioFingerprints: []cleanr.ScenarioFingerprint{
						{Name: "workflow-a", SystemHash: "sys-a", InputHash: "input-a-2", ContextHash: "ctx-a"},
						{Name: "workflow-c", SystemHash: "sys-c", InputHash: "input-c", MemoryReplayHash: "mem-c", MemoryReplaySteps: 2},
					},
				},
				Suites: []cleanr.HistorySuite{{Name: "claim-trace", Passed: false, FailedCases: 1}},
			},
		},
	}

	path := t.TempDir() + "/trends.yaml"
	if err := cleanr.WriteTrendHistoryFile(path, history); err != nil {
		t.Fatalf("write trend history: %v", err)
	}

	analysis, err := cleanr.AnalyzeTrendHistoryFile(path, 2)
	if err != nil {
		t.Fatalf("analyze trend history: %v", err)
	}
	if analysis.BuildDiff == nil {
		t.Fatalf("expected build diff, got %+v", analysis)
	}
	if analysis.BuildDiff.ModelBefore != "gpt-4.1-mini" || analysis.BuildDiff.ModelAfter != "gpt-4.1" {
		t.Fatalf("unexpected model diff: %+v", analysis.BuildDiff)
	}
	if len(analysis.BuildDiff.ScenarioChanges) != 3 {
		t.Fatalf("expected 3 scenario changes, got %+v", analysis.BuildDiff.ScenarioChanges)
	}

	var text bytes.Buffer
	if err := cleanr.WriteTrendAnalysis(&text, analysis, "text"); err != nil {
		t.Fatalf("write text analysis: %v", err)
	}
	for _, want := range []string{
		"Build Changes",
		"model=gpt-4.1-mini -> gpt-4.1",
		"workflow-a | changed | input",
		"workflow-b | removed",
		"workflow-c | new",
	} {
		if !strings.Contains(text.String(), want) {
			t.Fatalf("expected %q in trend analysis, got %s", want, text.String())
		}
	}
}

func TestAnalyzeTrendHistoryTracksCaseRegressionsAndFailureBuckets(t *testing.T) {
	history := cleanr.TrendHistoryFile{
		Version: "v1alpha1",
		Target:  "assistant-api",
		Runs: []cleanr.TrendHistoryRun{
			{
				BuildID:      "build-1",
				GeneratedAt:  time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC),
				Passed:       true,
				Duration:     2 * time.Second,
				FailedSuites: 0,
				FailedCases:  0,
				Suites: []cleanr.HistorySuite{
					{
						Name:   "claim-trace",
						Passed: true,
						Cases: []cleanr.HistoryCase{
							{Name: "workflow-a", Passed: true},
							{Name: "workflow-b", Passed: false, FindingSignatures: []string{"claimed tool execution with no matching invocation"}},
						},
					},
				},
			},
			{
				BuildID:      "build-2",
				GeneratedAt:  time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
				Passed:       false,
				Duration:     3 * time.Second,
				FailedSuites: 1,
				FailedCases:  2,
				Suites: []cleanr.HistorySuite{
					{
						Name:        "claim-trace",
						Passed:      false,
						FailedCases: 2,
						Cases: []cleanr.HistoryCase{
							{
								Name:                  "workflow-a",
								Passed:                false,
								FindingSignatures:     []string{"claimed tool execution with no matching invocation"},
								FirstUnsupportedClaim: "called lookup_policy",
								ToolCalls:             []string{"lookup_policy"},
							},
							{
								Name:      "workflow-b",
								Passed:    true,
								ToolCalls: []string{"lookup_policy"},
							},
							{
								Name:              "workflow-c",
								Passed:            false,
								FindingSignatures: []string{"claimed tool execution with no matching invocation"},
							},
						},
					},
				},
			},
		},
	}
	path := t.TempDir() + "/trends.yaml"
	if err := cleanr.WriteTrendHistoryFile(path, history); err != nil {
		t.Fatalf("write trend history: %v", err)
	}

	analysis, err := cleanr.AnalyzeTrendHistoryFile(path, 2)
	if err != nil {
		t.Fatalf("analyze trend history: %v", err)
	}
	if len(analysis.CaseRegressions) != 2 {
		t.Fatalf("expected 2 case regressions, got %+v", analysis.CaseRegressions)
	}
	if analysis.CaseRegressions[0].Suite != "claim-trace" || analysis.CaseRegressions[0].Name != "workflow-a" {
		t.Fatalf("unexpected first case regression: %+v", analysis.CaseRegressions[0])
	}
	if analysis.CaseRegressions[0].Status != "regressed" {
		t.Fatalf("expected workflow-a to regress, got %+v", analysis.CaseRegressions[0])
	}
	if analysis.CaseRegressions[1].Status != "new" || analysis.CaseRegressions[1].Name != "workflow-c" {
		t.Fatalf("expected workflow-c to be a new failing workflow, got %+v", analysis.CaseRegressions[1])
	}
	if len(analysis.CaseImprovements) != 1 || analysis.CaseImprovements[0].Name != "workflow-b" {
		t.Fatalf("expected workflow-b improvement, got %+v", analysis.CaseImprovements)
	}
	if len(analysis.FailureBuckets) != 1 {
		t.Fatalf("expected 1 grouped failure bucket, got %+v", analysis.FailureBuckets)
	}
	if analysis.FailureBuckets[0].Signature != "claimed tool execution with no matching invocation" || analysis.FailureBuckets[0].Count != 2 {
		t.Fatalf("unexpected leading failure bucket: %+v", analysis.FailureBuckets[0])
	}

	var text bytes.Buffer
	if err := cleanr.WriteTrendAnalysis(&text, analysis, "text"); err != nil {
		t.Fatalf("write text analysis: %v", err)
	}
	for _, want := range []string{
		"Case Regressions",
		"workflow-a | regressed",
		"Case Improvements",
		"workflow-b | improved",
		"Failure Buckets",
		"cases=2",
	} {
		if !strings.Contains(text.String(), want) {
			t.Fatalf("expected %q in text analysis, got %s", want, text.String())
		}
	}
}

func TestTrendHistoryRetainsFailedWorkflowCasesWithoutEvidence(t *testing.T) {
	report := cleanr.Report{
		Name:         "workflow-history",
		GeneratedAt:  time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		Passed:       false,
		Duration:     time.Second,
		TotalSuites:  1,
		FailedSuites: 1,
		TotalCases:   2,
		FailedCases:  1,
		Suites: []cleanr.SuiteResult{
			{
				Name:   "claim-trace",
				Passed: false,
				Cases: []cleanr.CaseResult{
					{Name: "failed-without-evidence", Passed: false},
					{Name: "passed-without-evidence", Passed: true},
				},
			},
		},
	}
	path := t.TempDir() + "/workflow-trends.json"

	if err := cleanr.AttachTrendHistory(&report, path, "build-1", 5); err != nil {
		t.Fatalf("attach trend history: %v", err)
	}

	history, err := cleanr.LoadTrendHistoryFile(path)
	if err != nil {
		t.Fatalf("load trend history: %v", err)
	}
	if len(history.Runs) != 1 || len(history.Runs[0].Suites) != 1 {
		t.Fatalf("unexpected history layout: %+v", history)
	}
	cases := history.Runs[0].Suites[0].Cases
	if len(cases) != 1 {
		t.Fatalf("expected only retained failed workflow case, got %+v", cases)
	}
	if cases[0].Name != "failed-without-evidence" || cases[0].Passed {
		t.Fatalf("unexpected retained case: %+v", cases[0])
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
