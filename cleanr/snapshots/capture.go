package snapshots

import (
	"context"
	"fmt"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func Capture(ctx context.Context, cfg core.Config, target core.Target) (File, error) {
	scenarios := filterSnapshotScenarios(cfg.Scenarios, cfg.Suites.Drift.StableTags)
	if len(scenarios) == 0 {
		scenarios = cfg.Scenarios
	}

	snapshot := File{
		Version:     "v1alpha1",
		GeneratedAt: time.Now().UTC(),
		Target:      cfg.Target.Name,
		Scenarios:   make([]ScenarioSnapshot, 0, len(scenarios)),
	}

	for _, scenario := range scenarios {
		resp := target.Invoke(ctx, core.Request{
			Scenario: scenario,
			System:   scenario.System,
			Prompt:   scenario.Input,
			Timeout:  cfg.Target.Timeout(),
		})
		if resp.Err != nil {
			return File{}, resp.Err
		}
		if resp.ExtractError != nil {
			return File{}, fmt.Errorf("capture snapshot for %s: %w", scenario.Name, resp.ExtractError)
		}
		if resp.StatusCode >= 400 {
			return File{}, fmt.Errorf("capture snapshot for %s: received status %d", scenario.Name, resp.StatusCode)
		}
		snapshot.Scenarios = append(snapshot.Scenarios, ScenarioSnapshot{
			Name:       scenario.Name,
			System:     scenario.System,
			Input:      scenario.Input,
			StatusCode: resp.StatusCode,
			Text:       resp.Text,
			Usage:      resp.Usage,
			Normalized: resp.Normalized,
		})
	}

	return snapshot, nil
}

func filterSnapshotScenarios(scenarios []core.Scenario, tags []string) []core.Scenario {
	if len(tags) == 0 {
		return nil
	}
	var out []core.Scenario
	for _, scenario := range scenarios {
		for _, want := range tags {
			for _, tag := range scenario.Tags {
				if tag == want {
					out = append(out, scenario)
					goto nextScenario
				}
			}
		}
	nextScenario:
	}
	return out
}
