package tests

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"cleanr/cleanr"
	enginespkg "cleanr/cleanr/engines"
)

type sequenceTarget struct {
	mu        sync.Mutex
	responses []cleanr.Response
	requests  []cleanr.Request
}

func (t *sequenceTarget) Invoke(_ context.Context, req cleanr.Request) cleanr.Response {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.requests = append(t.requests, req)
	if len(t.responses) == 0 {
		return cleanr.Response{}
	}
	resp := t.responses[0]
	t.responses = t.responses[1:]
	return resp
}

func TestEngineNamesAndSecurityFindingsCoverage(t *testing.T) {
	t.Parallel()

	if (enginespkg.ChaosEngine{}).Name() != "chaos" ||
		(enginespkg.ClaimTraceEngine{}).Name() != "claim-trace" ||
		(enginespkg.DriftEngine{}).Name() != "drift" ||
		(enginespkg.LoadEngine{}).Name() != "load" ||
		(enginespkg.MemorySafetyEngine{}).Name() != "memory-safety" ||
		(enginespkg.PromptInjectionEngine{}).Name() != "prompt-injection" ||
		(enginespkg.ProvenanceEngine{}).Name() != "provenance" ||
		(enginespkg.SecurityEngine{}).Name() != "security" ||
		(enginespkg.ShadowStateEngine{}).Name() != "shadow-state" ||
		(enginespkg.TokenOptimizationEngine{}).Name() != "token-optimization" {
		t.Fatal("unexpected engine names")
	}

	target := &sequenceTarget{responses: []cleanr.Response{
		{Err: errors.New("boom")},
		{StatusCode: 200, ExtractError: io.EOF},
		{StatusCode: 200, ExtractError: io.EOF, Normalized: cleanr.ProviderResponse{ToolCalls: []cleanr.ToolCall{{Name: "lookup"}}}},
		{StatusCode: 503, Text: "ssn 123-45-6789 SECRET dangerous leaked"},
		{StatusCode: 404, Text: "missing expected"},
		{StatusCode: 200, Text: "token api key", Normalized: cleanr.ProviderResponse{
			Provider:     "demo",
			ID:           "resp_1",
			Model:        "model-x",
			Role:         "assistant",
			Status:       "complete",
			FinishReason: "stop",
			StopSequence: "</end>",
			ToolCalls:    []cleanr.ToolCall{{Name: "tool-a"}},
			Raw:          map[string]any{"debug": true},
		}},
	}}

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{
		{Name: "err", Input: "a"},
		{Name: "extract", Input: "b"},
		{Name: "tool-extract", Input: "c"},
		{Name: "server", Input: "d", ForbiddenContains: []string{"secret"}, ExpectedContains: []string{"expected"}},
		{Name: "client", Input: "e", ExpectedContains: []string{"expected"}},
		{Name: "normalized", Input: "f", ForbiddenContains: []string{"token"}, ExpectedContains: []string{"expected"}},
	}
	cfg.Suites.Security.Enabled = true
	cfg.Suites.Security.MaxPIIMatches = 0
	cfg.Suites.Security.DangerousToolIndicators = []string{"dangerous"}
	cfg.Suites.Security.SecretExposureIndicators = []string{"SECRET", "api key"}
	cfg.Suites.Security.LeakPatterns = []string{"leaked"}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false

	report := cleanr.NewRunner(cfg, target).Run(context.Background())
	if len(report.Suites) != 1 || report.Suites[0].Name != "security" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Suites[0].Cases[5].Details["provider"] != "demo" {
		t.Fatalf("expected normalized response details, got %+v", report.Suites[0].Cases[5].Details)
	}
}

func TestPromptChaosDriftAndTokenOptimizationCoverage(t *testing.T) {
	t.Parallel()

	t.Run("prompt injection and chaos", func(t *testing.T) {
		t.Parallel()

		target := &sequenceTarget{responses: []cleanr.Response{
			{StatusCode: 200, Text: strings.Repeat("refuse ", 60) + "system prompt token"},
			{StatusCode: 200, Text: "ok", Latency: 10 * time.Millisecond},
			{StatusCode: 500, Text: "bad", Latency: 20 * time.Millisecond},
			{StatusCode: 200, Text: "ok", Latency: 30 * time.Millisecond},
		}}

		cfg := cleanr.ExampleConfig()
		cfg.Scenarios = []cleanr.Scenario{{Name: "scenario", Input: "hello", ForbiddenContains: []string{"bad"}}}
		cfg.Suites.PromptInjection.Enabled = true
		cfg.Suites.PromptInjection.BlockIndicators = []string{"refuse"}
		cfg.Suites.Security.Enabled = false
		cfg.Suites.Load.Enabled = false
		cfg.Suites.Chaos.Enabled = true
		cfg.Suites.Chaos.Faults = nil
		cfg.Suites.Chaos.TimeoutScale = 0.5
		cfg.Suites.Chaos.NoiseBytes = 10
		cfg.Suites.Chaos.MaxErrorRate = 10
		cfg.Suites.Drift.Enabled = false
		cfg.Suites.TokenOptimization.Enabled = false

		report := cleanr.NewRunner(cfg, target).Run(context.Background())
		if len(report.Suites) != 2 {
			t.Fatalf("unexpected suite count: %+v", report.Suites)
		}
		if len(target.requests) != 4 {
			t.Fatalf("unexpected request count: %d", len(target.requests))
		}
		if !strings.Contains(target.requests[2].Prompt, "noisenoise") {
			t.Fatalf("expected context overflow prompt mutation, got %q", target.requests[2].Prompt)
		}
		if target.requests[1].Timeout >= cfg.Target.Timeout() {
			t.Fatalf("expected tight deadline mutation, got %s", target.requests[1].Timeout)
		}
		if !strings.Contains(target.requests[3].Prompt, "\nhello") {
			t.Fatalf("expected duplicate turn mutation, got %q", target.requests[3].Prompt)
		}
	})

	t.Run("drift and token optimization", func(t *testing.T) {
		t.Parallel()

		target := &sequenceTarget{responses: []cleanr.Response{
			{Text: "stable alpha"},
			{Text: "stable beta"},
			{Text: "stable gamma"},
			{Err: errors.New("network blip")},
			{StatusCode: 200, Text: "repeat repeat repeat repeat", Usage: cleanr.TokenUsage{InputTokens: 4, OutputTokens: 20}},
			{StatusCode: 200, Text: "", Usage: cleanr.TokenUsage{}},
		}}

		cfg := cleanr.ExampleConfig()
		cfg.Scenarios = []cleanr.Scenario{
			{Name: "stable", Input: "drift me", Tags: []string{"stable"}},
			{Name: "unstable", Input: "error", Tags: []string{"stable"}},
			{Name: "token-explicit", System: "same same", Input: "same same"},
			{Name: "token-heuristic", System: "", Input: ""},
		}
		cfg.Suites.PromptInjection.Enabled = false
		cfg.Suites.Security.Enabled = false
		cfg.Suites.Load.Enabled = false
		cfg.Suites.Chaos.Enabled = false
		cfg.Suites.Drift.Enabled = true
		cfg.Suites.Drift.StableTags = []string{"stable"}
		cfg.Suites.Drift.Iterations = 2
		cfg.Suites.Drift.MaxNormalizedDrift = 0
		cfg.Suites.Drift.MinConsistencyScore = 0.99
		cfg.Suites.TokenOptimization.Enabled = true
		cfg.Suites.TokenOptimization.MaxInputTokens = 1
		cfg.Suites.TokenOptimization.MaxOutputTokens = 1
		cfg.Suites.TokenOptimization.MaxTotalTokens = 2
		cfg.Suites.TokenOptimization.MaxOutputInputRatio = 0.5
		cfg.Suites.TokenOptimization.MaxPromptDuplicationRatio = 0
		cfg.Suites.TokenOptimization.MaxResponseDuplicationRatio = 0
		cfg.Suites.TokenOptimization.SuggestedMaxOutputTokens = 1

		report := cleanr.NewRunner(cfg, target).Run(context.Background())
		if len(report.Suites) != 2 {
			t.Fatalf("unexpected suites: %+v", report.Suites)
		}
	})

	t.Run("drift semantic paraphrase tolerance", func(t *testing.T) {
		t.Parallel()

		target := &sequenceTarget{responses: []cleanr.Response{
			{Text: "Refunds are available within 30 days of purchase."},
			{Text: "A refund is available within 30 days after purchase."},
			{Text: "Customers can get a refund within 30 days after purchase."},
		}}

		cfg := cleanr.ExampleConfig()
		cfg.Scenarios = []cleanr.Scenario{{Name: "semantic", Input: "refund policy", Tags: []string{"stable"}}}
		cfg.Suites.PromptInjection.Enabled = false
		cfg.Suites.Security.Enabled = false
		cfg.Suites.Load.Enabled = false
		cfg.Suites.Chaos.Enabled = false
		cfg.Suites.TokenOptimization.Enabled = false
		cfg.Suites.Drift.Enabled = true
		cfg.Suites.Drift.StableTags = []string{"stable"}
		cfg.Suites.Drift.Iterations = 3
		cfg.Suites.Drift.MaxNormalizedDrift = 0.05
		cfg.Suites.Drift.MaxSemanticDrift = 0.25
		cfg.Suites.Drift.MinConsistencyScore = 0.5
		cfg.Suites.Drift.MinSemanticConsistencyScore = 0.75

		report := cleanr.NewRunner(cfg, target).Run(context.Background())
		if !report.Passed {
			t.Fatalf("expected semantic paraphrases to pass drift: %+v", report)
		}
		details := report.Suites[0].Cases[0].Details
		if _, ok := details["lexical_drift_note"]; !ok {
			t.Fatalf("expected lexical drift note when semantic drift remains acceptable: %+v", details)
		}
	})
}

func TestLoadEngineCoverage(t *testing.T) {
	t.Parallel()

	target := &sequenceTarget{responses: []cleanr.Response{
		{StatusCode: 200, Latency: 5 * time.Millisecond},
		{StatusCode: 0, Latency: 10 * time.Millisecond},
		{StatusCode: 503, Latency: 15 * time.Millisecond},
		{Err: errors.New("boom"), Latency: 20 * time.Millisecond},
	}}

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{Name: "a", Input: "x"}, {Name: "b", Input: "y"}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = true
	cfg.Suites.Load.VirtualUsers = 2
	cfg.Suites.Load.RequestsPerUser = 2
	cfg.Suites.Load.MaxErrorRatePct = 10
	cfg.Suites.Load.P95LatencyMS = 1
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false

	report := cleanr.NewRunner(cfg, target).Run(context.Background())
	if len(report.Suites) != 1 || report.Suites[0].Name != "load" {
		t.Fatalf("unexpected load report: %+v", report)
	}
}
