package tests

import (
	"context"
	"sync"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
)

type taggedLoadTarget struct {
	mu      sync.Mutex
	prompts []string
}

func (t *taggedLoadTarget) Invoke(_ context.Context, req cleanr.Request) cleanr.Response {
	t.mu.Lock()
	t.prompts = append(t.prompts, req.Prompt)
	t.mu.Unlock()
	return cleanr.Response{StatusCode: 200, Text: "ok"}
}

func TestLoadEngineScopesScenariosByTags(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{
		{Name: "stable-a", Input: "first", Tags: []string{"load"}},
		{Name: "stable-b", Input: "second", Tags: []string{"load"}},
		{Name: "excluded", Input: "third", Tags: []string{"other"}},
	}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = true
	cfg.Suites.Load.VirtualUsers = 2
	cfg.Suites.Load.RequestsPerUser = 2
	cfg.Suites.Load.ScenarioTags = []string{"load"}
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.ShadowState.Enabled = false
	cfg.Suites.Provenance.Enabled = false
	cfg.Suites.ClaimTrace.Enabled = false
	cfg.Suites.ReleasePolicy.Enabled = false
	cfg.Suites.MemorySafety.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false

	target := &taggedLoadTarget{}
	report := cleanr.NewRunner(cfg, target).Run(context.Background())
	if !report.Passed {
		t.Fatalf("expected load run to pass: %+v", report)
	}
	if len(target.prompts) != 4 {
		t.Fatalf("expected 4 load requests, got %d", len(target.prompts))
	}
	for _, prompt := range target.prompts {
		if prompt == "third" {
			t.Fatalf("expected tagged load profile to exclude third scenario: %+v", target.prompts)
		}
	}
	details := report.Suites[0].Cases[0].Details
	if details["scenario_count"] != 2 {
		t.Fatalf("expected retained scenario_count=2, got %+v", details)
	}
}
