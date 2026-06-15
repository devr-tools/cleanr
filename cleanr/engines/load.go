package engines

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type LoadEngine struct{}

func (LoadEngine) Name() string { return "load" }

type loadSample struct {
	latency time.Duration
	err     error
	status  int
}

type loadSummary struct {
	requests      int
	errorRatePct  int
	p50           time.Duration
	p95           time.Duration
	p99           time.Duration
	throughputRPS float64
}

type loadEvaluation struct {
	summary  loadSummary
	passed   bool
	findings []core.Finding
}

func (LoadEngine) Run(ctx context.Context, runCtx *core.RunContext) core.SuiteResult {
	cfg := runCtx.Config.Suites.Load
	scenarios := selectLoadScenarios(runCtx.Config.Scenarios, cfg.ScenarioTags)
	start := time.Now()
	samples := runLoadSamples(ctx, runCtx, scenarios, cfg)
	elapsed := time.Since(start)
	evaluation := evaluateLoadSummary(cfg, summarizeLoadSamples(samples, elapsed))
	return buildLoadSuiteResult(cfg, scenarios, evaluation, time.Since(start))
}

func selectLoadScenarios(scenarios []core.Scenario, tags []string) []core.Scenario {
	if scoped := filterScenariosByTags(scenarios, tags); len(scoped) > 0 {
		return scoped
	}
	return scenarios
}

func runLoadSamples(ctx context.Context, runCtx *core.RunContext, scenarios []core.Scenario, cfg core.LoadConfig) []loadSample {
	totalRequests := cfg.VirtualUsers * cfg.RequestsPerUser
	samples := make([]loadSample, 0, totalRequests)
	ch := make(chan loadSample, totalRequests)
	var wg sync.WaitGroup

	for vu := 0; vu < cfg.VirtualUsers; vu++ {
		wg.Add(1)
		go func(vu int) {
			defer wg.Done()
			for i := 0; i < cfg.RequestsPerUser; i++ {
				scenario := scenarios[(vu+i)%len(scenarios)]
				resp := runCtx.Target.Invoke(ctx, scenarioRequest(scenario, runCtx.Config.Target.Timeout()))
				ch <- loadSample{latency: resp.Latency, err: resp.Err, status: resp.StatusCode}
			}
		}(vu)
	}

	wg.Wait()
	close(ch)
	for s := range ch {
		samples = append(samples, s)
	}
	return samples
}

func summarizeLoadSamples(samples []loadSample, elapsed time.Duration) loadSummary {
	errCount := 0
	latencies := make([]time.Duration, 0, len(samples))
	for _, s := range samples {
		latencies = append(latencies, s.latency)
		if s.err != nil || s.status >= 500 || s.status == 0 {
			errCount++
		}
	}
	summary := loadSummary{
		requests:     len(samples),
		errorRatePct: percentOf(errCount, len(samples)),
		p50:          percentile(latencies, 50),
		p95:          percentile(latencies, 95),
		p99:          percentile(latencies, 99),
	}
	if elapsed > 0 {
		summary.throughputRPS = round3(float64(len(samples)) / elapsed.Seconds())
	}
	return summary
}

func percentOf(count, total int) int {
	if total == 0 {
		return 0
	}
	return int(math.Round(float64(count) / float64(total) * 100))
}

func evaluateLoadSummary(cfg core.LoadConfig, summary loadSummary) loadEvaluation {
	findings := make([]core.Finding, 0)
	passed := true
	if summary.errorRatePct > cfg.MaxErrorRatePct {
		passed = false
		findings = append(findings, core.Finding{
			Severity: "critical",
			Message:  fmt.Sprintf("error rate %d%% exceeded threshold %d%%", summary.errorRatePct, cfg.MaxErrorRatePct),
		})
	}
	if summary.p95 > time.Duration(cfg.P95LatencyMS)*time.Millisecond {
		passed = false
		findings = append(findings, core.Finding{
			Severity: "high",
			Message:  fmt.Sprintf("p95 latency %s exceeded threshold %dms", summary.p95, cfg.P95LatencyMS),
		})
	}
	return loadEvaluation{
		summary:  summary,
		passed:   passed,
		findings: findings,
	}
}

func buildLoadSuiteResult(cfg core.LoadConfig, scenarios []core.Scenario, evaluation loadEvaluation, duration time.Duration) core.SuiteResult {
	return core.SuiteResult{
		Name:     "load",
		Passed:   evaluation.passed,
		Duration: duration,
		Cases: []core.CaseResult{{
			Name:       "concurrency-benchmark",
			Passed:     evaluation.passed,
			Duration:   duration,
			LatencyP95: evaluation.summary.p95,
			Findings:   evaluation.findings,
			Details: map[string]any{
				"requests":          evaluation.summary.requests,
				"virtual_users":     cfg.VirtualUsers,
				"requests_per_user": cfg.RequestsPerUser,
				"scenario_count":    len(scenarios),
				"scenario_tags":     append([]string(nil), cfg.ScenarioTags...),
				"error_rate_pct":    evaluation.summary.errorRatePct,
				"latency_p50_ms":    evaluation.summary.p50.Milliseconds(),
				"latency_p95_ms":    evaluation.summary.p95.Milliseconds(),
				"latency_p99_ms":    evaluation.summary.p99.Milliseconds(),
				"throughput_rps":    evaluation.summary.throughputRPS,
			},
		}},
	}
}
