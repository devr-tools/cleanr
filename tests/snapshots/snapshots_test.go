package tests

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"cleanr/cleanr"
)

type snapshotTarget struct {
	responses []cleanr.Response
	index     int
}

func (t *snapshotTarget) Invoke(_ context.Context, req cleanr.Request) cleanr.Response {
	resp := t.responses[t.index]
	t.index++
	return resp
}

func TestSnapshotFacadeCoversLoadWriteCaptureAndLookup(t *testing.T) {
	t.Parallel()

	snapshot := cleanr.SnapshotFile{
		Version: "v1alpha1",
		Target:  "demo",
		Scenarios: []cleanr.ScenarioSnapshot{
			{Name: "one", Text: "hello", StatusCode: 200},
			{Name: "two", Text: "world", StatusCode: 201},
		},
	}

	jsonPath := filepath.Join(t.TempDir(), "snapshots.json")
	if err := cleanr.WriteSnapshotFile(jsonPath, snapshot); err != nil {
		t.Fatalf("write json snapshot: %v", err)
	}
	loaded, err := cleanr.LoadSnapshotFile(jsonPath)
	if err != nil {
		t.Fatalf("load json snapshot: %v", err)
	}
	if _, ok := loaded.FindScenario("one"); !ok {
		t.Fatalf("expected to find snapshot scenario: %+v", loaded)
	}
	if _, ok := loaded.FindScenario("missing"); ok {
		t.Fatalf("did not expect missing snapshot scenario")
	}

	yamlPath := filepath.Join(t.TempDir(), "snapshots.yaml")
	if err := cleanr.WriteSnapshotFile(yamlPath, snapshot); err != nil {
		t.Fatalf("write yaml snapshot: %v", err)
	}
	if _, err := cleanr.LoadSnapshotFile(yamlPath); err != nil {
		t.Fatalf("load yaml snapshot: %v", err)
	}

	cfg := cleanr.ExampleConfig()
	cfg.Target.Name = "capture-demo"
	cfg.Scenarios = []cleanr.Scenario{
		{Name: "stable", Input: "a", Tags: []string{"stable"}},
		{Name: "ignored", Input: "b"},
	}
	cfg.Suites.Drift.StableTags = []string{"stable"}
	target := &snapshotTarget{responses: []cleanr.Response{{
		StatusCode: 200,
		Text:       "snapshot text",
		Normalized: cleanr.ProviderResponse{Provider: "http"},
	}}}
	captured, err := cleanr.CaptureSnapshots(context.Background(), cfg, target)
	if err != nil {
		t.Fatalf("capture snapshots: %v", err)
	}
	if captured.Target != "capture-demo" || len(captured.Scenarios) != 1 {
		t.Fatalf("unexpected captured snapshots: %+v", captured)
	}

	target = &snapshotTarget{responses: []cleanr.Response{{Err: errors.New("boom")}}}
	if _, err := cleanr.CaptureSnapshots(context.Background(), cfg, target); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected capture error, got %v", err)
	}
}
