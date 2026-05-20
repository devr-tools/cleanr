package tests

import (
	"context"
	"testing"

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
