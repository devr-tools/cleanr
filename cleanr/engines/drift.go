package engines

import (
	"context"
	"fmt"
	"time"

	"cleanr/cleanr/core"
)

type DriftEngine struct{}

func (DriftEngine) Name() string { return "drift" }

func (DriftEngine) Run(ctx context.Context, runCtx *core.RunContext) core.SuiteResult {
	cfg := runCtx.Config.Suites.Drift
	stable := filterStableScenarios(runCtx.Config.Scenarios, cfg.StableTags)
	if len(stable) == 0 {
		stable = runCtx.Config.Scenarios
	}
	cases := make([]core.CaseResult, 0, len(stable))
	for _, scenario := range stable {
		start := time.Now()
		responses := make([]string, 0, cfg.Iterations)
		findings := make([]core.Finding, 0)
		for i := 0; i < cfg.Iterations; i++ {
			resp := runCtx.Target.Invoke(ctx, core.Request{
				Scenario: scenario,
				System:   scenario.System,
				Prompt:   scenario.Input,
				Timeout:  runCtx.Config.Target.Timeout(),
			})
			if resp.Err != nil {
				findings = append(findings, core.Finding{Severity: "high", Message: resp.Err.Error()})
				continue
			}
			responses = append(responses, resp.Text)
		}
		drift, consistency := measureDrift(responses)
		if drift > cfg.MaxNormalizedDrift {
			findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("normalized drift %.3f exceeded threshold %.3f", drift, cfg.MaxNormalizedDrift)})
		}
		if consistency < cfg.MinConsistencyScore {
			findings = append(findings, core.Finding{Severity: "medium", Message: fmt.Sprintf("consistency score %.3f fell below threshold %.3f", consistency, cfg.MinConsistencyScore)})
		}
		cases = append(cases, core.CaseResult{
			Name:     scenario.Name,
			Passed:   len(findings) == 0,
			Duration: time.Since(start),
			Score:    consistency,
			Findings: findings,
			Details: map[string]any{
				"normalized_drift": drift,
				"samples":          len(responses),
			},
		})
	}
	return core.SuiteResult{Name: "drift", Passed: allPassed(cases), Cases: cases}
}
