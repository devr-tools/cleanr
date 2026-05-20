package engines

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"cleanr/cleanr/core"
)

type ChaosEngine struct{}

func (ChaosEngine) Name() string { return "chaos" }

func (ChaosEngine) Run(ctx context.Context, runCtx *core.RunContext) core.SuiteResult {
	cfg := runCtx.Config.Suites.Chaos
	faults := cfg.Faults
	if len(faults) == 0 {
		faults = []string{"tight_deadline", "context_overflow", "duplicate_turn"}
	}
	cases := make([]core.CaseResult, 0, len(runCtx.Config.Scenarios)*len(faults))
	errors := 0

	for _, scenario := range runCtx.Config.Scenarios {
		for _, fault := range faults {
			start := time.Now()
			req := core.Request{
				Scenario: scenario,
				System:   scenario.System,
				Prompt:   scenario.Input,
				Timeout:  runCtx.Config.Target.Timeout(),
			}
			switch fault {
			case "tight_deadline":
				req.Timeout = time.Duration(float64(req.Timeout) * cfg.TimeoutScale)
			case "context_overflow":
				req.Prompt = scenario.Input + "\n\n" + strings.Repeat("noise", max(1, cfg.NoiseBytes/5))
			case "duplicate_turn":
				req.Prompt = scenario.Input + "\n" + scenario.Input
			}
			resp := runCtx.Target.Invoke(ctx, req)
			findings := responseFindings(resp, scenario.ForbiddenContains)
			passed := resp.Err == nil && resp.StatusCode < 500 && len(findings) == 0
			if !passed {
				errors++
			}
			cases = append(cases, core.CaseResult{
				Name:     scenario.Name + ":" + fault,
				Passed:   passed,
				Duration: time.Since(start),
				Findings: findings,
				Details: map[string]any{
					"status_code": resp.StatusCode,
					"latency_ms":  resp.Latency.Milliseconds(),
				},
			})
		}
	}

	errorRate := 0
	if len(cases) > 0 {
		errorRate = int(math.Round(float64(errors) / float64(len(cases)) * 100))
	}
	passed := errorRate <= cfg.MaxErrorRate
	findings := []core.Finding{}
	if !passed {
		findings = append(findings, core.Finding{Severity: "critical", Message: fmt.Sprintf("chaos failure rate %d%% exceeded threshold %d%%", errorRate, cfg.MaxErrorRate)})
	}
	return core.SuiteResult{
		Name:     "chaos",
		Passed:   passed,
		Cases:    cases,
		Findings: findings,
		Meta: map[string]any{
			"error_rate_pct": errorRate,
		},
	}
}
