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
	usage   core.TokenUsage
}

type loadSummary struct {
	requests           int
	errorRatePct       int
	p50                time.Duration
	p95                time.Duration
	p99                time.Duration
	throughputRPS      float64
	usageRequests      int
	pricedRequests     int
	totalInputTokens   int
	totalOutputTokens  int
	totalTokens        int
	tokenThroughputTPS float64
	avgCostPerRequest  float64
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
	evaluation := evaluateLoadSummary(cfg, summarizeLoadSamples(samples, elapsed, cfg))
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
				ch <- loadSample{latency: resp.Latency, err: resp.Err, status: resp.StatusCode, usage: actualLoadUsage(resp)}
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

func summarizeLoadSamples(samples []loadSample, elapsed time.Duration, cfg core.LoadConfig) loadSummary {
	errCount := 0
	latencies := make([]time.Duration, 0, len(samples))
	totalInputTokens := 0
	totalOutputTokens := 0
	totalTokens := 0
	totalCost := 0.0
	usageRequests := 0
	pricedRequests := 0
	for _, s := range samples {
		latencies = append(latencies, s.latency)
		if s.err != nil || s.status >= 500 || s.status == 0 {
			errCount++
		}
		if usage, ok := normalizedLoadUsage(s.usage); ok {
			usageRequests++
			totalInputTokens += usage.InputTokens
			totalOutputTokens += usage.OutputTokens
			totalTokens += usage.TotalTokens
			if cost, ok := estimateLoadRequestCost(cfg, usage); ok {
				totalCost += cost
				pricedRequests++
			}
		}
	}
	summary := loadSummary{
		requests:          len(samples),
		errorRatePct:      percentOf(errCount, len(samples)),
		p50:               percentile(latencies, 50),
		p95:               percentile(latencies, 95),
		p99:               percentile(latencies, 99),
		usageRequests:     usageRequests,
		pricedRequests:    pricedRequests,
		totalInputTokens:  totalInputTokens,
		totalOutputTokens: totalOutputTokens,
		totalTokens:       totalTokens,
	}
	if elapsed > 0 {
		summary.throughputRPS = round3(float64(len(samples)) / elapsed.Seconds())
		if usageRequests > 0 {
			summary.tokenThroughputTPS = round3(float64(totalTokens) / elapsed.Seconds())
		}
	}
	if pricedRequests > 0 {
		summary.avgCostPerRequest = round6(totalCost / float64(pricedRequests))
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
	if cfg.MinTokensPerSecond > 0 {
		if summary.usageRequests == 0 {
			passed = false
			findings = append(findings, core.Finding{
				Severity: "critical",
				Message:  "tokens/sec gate requires provider token usage, but no load samples reported usage",
			})
		} else if summary.tokenThroughputTPS < cfg.MinTokensPerSecond {
			passed = false
			findings = append(findings, core.Finding{
				Severity: "high",
				Message:  fmt.Sprintf("token throughput %.3f tokens/sec fell below threshold %.3f", summary.tokenThroughputTPS, cfg.MinTokensPerSecond),
			})
		}
	}
	if cfg.MaxCostPerRequest > 0 {
		if summary.pricedRequests == 0 {
			passed = false
			findings = append(findings, core.Finding{
				Severity: "critical",
				Message:  "cost-per-request gate requires provider usage and configured token pricing, but no priced load samples were available",
			})
		} else if summary.avgCostPerRequest > cfg.MaxCostPerRequest {
			passed = false
			findings = append(findings, core.Finding{
				Severity: "high",
				Message:  fmt.Sprintf("average cost/request %.6f exceeded threshold %.6f", summary.avgCostPerRequest, cfg.MaxCostPerRequest),
			})
		}
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
				"requests":             evaluation.summary.requests,
				"virtual_users":        cfg.VirtualUsers,
				"requests_per_user":    cfg.RequestsPerUser,
				"scenario_count":       len(scenarios),
				"scenario_tags":        append([]string(nil), cfg.ScenarioTags...),
				"error_rate_pct":       evaluation.summary.errorRatePct,
				"latency_p50_ms":       evaluation.summary.p50.Milliseconds(),
				"latency_p95_ms":       evaluation.summary.p95.Milliseconds(),
				"latency_p99_ms":       evaluation.summary.p99.Milliseconds(),
				"throughput_rps":       evaluation.summary.throughputRPS,
				"usage_requests":       evaluation.summary.usageRequests,
				"priced_requests":      evaluation.summary.pricedRequests,
				"input_tokens":         evaluation.summary.totalInputTokens,
				"output_tokens":        evaluation.summary.totalOutputTokens,
				"total_tokens":         evaluation.summary.totalTokens,
				"token_throughput_tps": evaluation.summary.tokenThroughputTPS,
				"avg_cost_per_request": evaluation.summary.avgCostPerRequest,
			},
		}},
	}
}

func actualLoadUsage(resp core.Response) core.TokenUsage {
	usage, ok := normalizedLoadUsage(resp.Usage)
	if !ok {
		return core.TokenUsage{}
	}
	return usage
}

func normalizedLoadUsage(usage core.TokenUsage) (core.TokenUsage, bool) {
	if usage.TotalTokens == 0 && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	if usage.TotalTokens <= 0 && usage.InputTokens <= 0 && usage.OutputTokens <= 0 {
		return core.TokenUsage{}, false
	}
	return usage, true
}

func estimateLoadRequestCost(cfg core.LoadConfig, usage core.TokenUsage) (float64, bool) {
	if cfg.InputCostPer1MTokens == 0 && cfg.OutputCostPer1MTokens == 0 {
		return 0, false
	}
	cost := (float64(usage.InputTokens) * cfg.InputCostPer1MTokens / 1_000_000) +
		(float64(usage.OutputTokens) * cfg.OutputCostPer1MTokens / 1_000_000)
	return cost, true
}

func round6(v float64) float64 {
	return math.Round(v*1_000_000) / 1_000_000
}
