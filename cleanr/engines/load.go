package engines

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"cleanr/cleanr/core"
)

type LoadEngine struct{}

func (LoadEngine) Name() string { return "load" }

func (LoadEngine) Run(ctx context.Context, runCtx *core.RunContext) core.SuiteResult {
	cfg := runCtx.Config.Suites.Load
	type sample struct {
		latency time.Duration
		err     error
		status  int
	}

	totalRequests := cfg.VirtualUsers * cfg.RequestsPerUser
	samples := make([]sample, 0, totalRequests)
	ch := make(chan sample, totalRequests)
	var wg sync.WaitGroup
	start := time.Now()

	for vu := 0; vu < cfg.VirtualUsers; vu++ {
		wg.Add(1)
		go func(vu int) {
			defer wg.Done()
			for i := 0; i < cfg.RequestsPerUser; i++ {
				scenario := runCtx.Config.Scenarios[(vu+i)%len(runCtx.Config.Scenarios)]
				resp := runCtx.Target.Invoke(ctx, core.Request{
					Scenario: scenario,
					System:   scenario.System,
					Prompt:   scenario.Input,
					Timeout:  runCtx.Config.Target.Timeout(),
				})
				ch <- sample{latency: resp.Latency, err: resp.Err, status: resp.StatusCode}
			}
		}(vu)
	}

	wg.Wait()
	close(ch)
	for s := range ch {
		samples = append(samples, s)
	}

	errCount := 0
	latencies := make([]time.Duration, 0, len(samples))
	for _, s := range samples {
		latencies = append(latencies, s.latency)
		if s.err != nil || s.status >= 500 || s.status == 0 {
			errCount++
		}
	}
	p95 := percentile(latencies, 95)
	errorRate := 0
	if len(samples) > 0 {
		errorRate = int(math.Round(float64(errCount) / float64(len(samples)) * 100))
	}
	findings := make([]core.Finding, 0)
	passed := true
	if errorRate > cfg.MaxErrorRatePct {
		passed = false
		findings = append(findings, core.Finding{Severity: "critical", Message: fmt.Sprintf("error rate %d%% exceeded threshold %d%%", errorRate, cfg.MaxErrorRatePct)})
	}
	if p95 > time.Duration(cfg.P95LatencyMS)*time.Millisecond {
		passed = false
		findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("p95 latency %s exceeded threshold %dms", p95, cfg.P95LatencyMS)})
	}
	return core.SuiteResult{
		Name:     "load",
		Passed:   passed,
		Duration: time.Since(start),
		Cases: []core.CaseResult{{
			Name:       "concurrency-benchmark",
			Passed:     passed,
			Duration:   time.Since(start),
			LatencyP95: p95,
			Findings:   findings,
			Details: map[string]any{
				"requests":       len(samples),
				"virtual_users":  cfg.VirtualUsers,
				"error_rate_pct": errorRate,
			},
		}},
	}
}
