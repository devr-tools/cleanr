package tests

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

type taggedLoadTarget struct {
	mu      sync.Mutex
	prompts []string
}

type meteredLoadTarget struct {
	latency time.Duration
	usage   cleanr.TokenUsage
}

func (t *taggedLoadTarget) Invoke(_ context.Context, req cleanr.Request) cleanr.Response {
	t.mu.Lock()
	t.prompts = append(t.prompts, req.Prompt)
	t.mu.Unlock()
	return cleanr.Response{StatusCode: 200, Text: "ok"}
}

func (t meteredLoadTarget) Invoke(_ context.Context, req cleanr.Request) cleanr.Response {
	time.Sleep(t.latency)
	return cleanr.Response{
		StatusCode: 200,
		Text:       "ok",
		Latency:    t.latency,
		Usage:      t.usage,
	}
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

func TestLoadEngineTracksTokenThroughputAndCost(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{Name: "metered", Input: "measure throughput"}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = true
	cfg.Suites.Load.VirtualUsers = 2
	cfg.Suites.Load.RequestsPerUser = 2
	cfg.Suites.Load.MinTokensPerSecond = 1000
	cfg.Suites.Load.MaxCostPerRequest = 0.001
	cfg.Suites.Load.InputCostPer1MTokens = 1
	cfg.Suites.Load.OutputCostPer1MTokens = 2
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.ShadowState.Enabled = false
	cfg.Suites.Provenance.Enabled = false
	cfg.Suites.ClaimTrace.Enabled = false
	cfg.Suites.ReleasePolicy.Enabled = false
	cfg.Suites.MemorySafety.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false

	target := meteredLoadTarget{
		latency: 20 * time.Millisecond,
		usage: cleanr.TokenUsage{
			InputTokens:  50,
			OutputTokens: 50,
			TotalTokens:  100,
		},
	}
	report := cleanr.NewRunner(cfg, target).Run(context.Background())
	if !report.Passed {
		t.Fatalf("expected metered load run to pass: %+v", report)
	}
	details := report.Suites[0].Cases[0].Details
	if details["usage_requests"] != 4 {
		t.Fatalf("expected usage_requests=4, got %+v", details)
	}
	if details["priced_requests"] != 4 {
		t.Fatalf("expected priced_requests=4, got %+v", details)
	}
	if details["total_tokens"] != 400 {
		t.Fatalf("expected total_tokens=400, got %+v", details)
	}
	if got, ok := details["avg_cost_per_request"].(float64); !ok || got <= 0 {
		t.Fatalf("expected avg_cost_per_request detail, got %+v", details)
	}
	if got, ok := details["token_throughput_tps"].(float64); !ok || got < cfg.Suites.Load.MinTokensPerSecond {
		t.Fatalf("expected token_throughput_tps >= %.0f, got %+v", cfg.Suites.Load.MinTokensPerSecond, details)
	}
}

func TestLoadEngineFailsTokensPerSecondGateWithoutUsage(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{Name: "unmetered", Input: "no usage"}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = true
	cfg.Suites.Load.VirtualUsers = 1
	cfg.Suites.Load.RequestsPerUser = 1
	cfg.Suites.Load.MinTokensPerSecond = 1
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.ShadowState.Enabled = false
	cfg.Suites.Provenance.Enabled = false
	cfg.Suites.ClaimTrace.Enabled = false
	cfg.Suites.ReleasePolicy.Enabled = false
	cfg.Suites.MemorySafety.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false

	report := cleanr.NewRunner(cfg, meteredLoadTarget{latency: 5 * time.Millisecond}).Run(context.Background())
	if report.Passed {
		t.Fatalf("expected load run without usage to fail token throughput gate")
	}
	findings := report.Suites[0].Cases[0].Findings
	if len(findings) == 0 || !strings.Contains(findings[0].Message, "no load samples reported usage") {
		t.Fatalf("expected missing-usage finding, got %+v", findings)
	}
}
