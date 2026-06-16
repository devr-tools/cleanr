package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

func TestCompareTrendSourcesImportsLangSmithEmbeddedCleanrReport(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	payload := map[string]any{
		"runs": []map[string]any{{
			"id":         "ls-run-1",
			"start_time": "2026-06-15T12:00:00Z",
			"cleanr": map[string]any{
				"report": map[string]any{
					"name":         "assistant-api",
					"passed":       true,
					"generated_at": "2026-06-15T12:00:00Z",
					"duration":     float64((2 * time.Second).Nanoseconds()),
					"total_suites": 1,
					"total_cases":  1,
					"metadata": map[string]any{
						"build_id":       "build-langsmith-1",
						"provider_model": "gpt-4.1-mini",
					},
					"suites": []map[string]any{{
						"name":   "llm-judge",
						"passed": true,
						"cases":  []map[string]any{{"name": "refunds", "passed": true}},
					}},
				},
			},
		}},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	path := filepath.Join(dir, "langsmith.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	results := cleanr.CompareTrendSources(context.Background(), cleanr.IntegrationsConfig{
		TrendSources: []cleanr.TrendSourceConfig{{
			Name: "langsmith",
			Type: "langsmith",
			Path: path,
		}},
	}, cleanr.Report{
		Name:        "assistant-api",
		Passed:      true,
		GeneratedAt: time.Date(2026, 6, 15, 12, 30, 0, 0, time.UTC),
		Metadata:    &cleanr.RunMetadata{BuildID: "build-current"},
		Suites:      []cleanr.SuiteResult{{Name: "llm-judge", Passed: true}},
	}, filepath.Join(dir, "cleanr.yaml"))

	if len(results) != 1 || results[0].Status != "compared" {
		t.Fatalf("unexpected trend source results: %+v", results)
	}
	if results[0].LatestBuildID != "build-langsmith-1" {
		t.Fatalf("unexpected imported build id: %+v", results[0])
	}
}

func TestCompareTrendSourcesImportsProviderLogsGenericRows(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	payload := map[string]any{
		"logs": []map[string]any{{
			"id":            "provider-log-1",
			"generated_at":  "2026-06-15T11:00:00Z",
			"duration_ms":   1200,
			"passed":        false,
			"failed_suites": 1,
			"failed_cases":  2,
			"model":         "claude-sonnet-4-20250514",
			"provider":      "anthropic",
		}},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	path := filepath.Join(dir, "provider-logs.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	results := cleanr.CompareTrendSources(context.Background(), cleanr.IntegrationsConfig{
		TrendSources: []cleanr.TrendSourceConfig{{
			Name: "provider-logs",
			Type: "provider_logs",
			Path: path,
		}},
	}, cleanr.Report{
		Name:        "assistant-api",
		Passed:      true,
		GeneratedAt: time.Date(2026, 6, 15, 12, 30, 0, 0, time.UTC),
		Metadata:    &cleanr.RunMetadata{BuildID: "build-current"},
		Suites:      []cleanr.SuiteResult{{Name: "llm-judge", Passed: true}},
	}, filepath.Join(dir, "cleanr.yaml"))

	if len(results) != 1 || results[0].Status != "compared" {
		t.Fatalf("unexpected trend source results: %+v", results)
	}
	if results[0].HistoryLength != 1 {
		t.Fatalf("unexpected imported history length: %+v", results[0])
	}
}
