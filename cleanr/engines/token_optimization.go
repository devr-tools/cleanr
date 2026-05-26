package engines

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type TokenOptimizationEngine struct{}

func (TokenOptimizationEngine) Name() string { return "token-optimization" }

func (TokenOptimizationEngine) Run(ctx context.Context, runCtx *core.RunContext) core.SuiteResult {
	cfg := runCtx.Config.Suites.TokenOptimization
	cases := make([]core.CaseResult, 0, len(runCtx.Config.Scenarios))
	totalInput := 0
	totalOutput := 0
	totalSavings := 0
	heuristicOnly := true

	for _, scenario := range runCtx.Config.Scenarios {
		start := time.Now()
		resp := runCtx.Target.Invoke(ctx, core.Request{
			Scenario: scenario,
			System:   scenario.System,
			Prompt:   scenario.Input,
			Timeout:  runCtx.Config.Target.Timeout(),
		})
		findings := responseFindings(resp, nil)
		usage := inferTokenUsage(scenario, resp)
		promptRatio := duplicationRatio(strings.TrimSpace(scenario.System + "\n" + scenario.Input))
		responseRatio := duplicationRatio(resp.Text)
		outputInputRatio := 0.0
		if usage.InputTokens > 0 {
			outputInputRatio = float64(usage.OutputTokens) / float64(usage.InputTokens)
		}
		savings := estimatedTokenSavings(usage, promptRatio, responseRatio, cfg)
		totalInput += usage.InputTokens
		totalOutput += usage.OutputTokens
		totalSavings += savings
		if !usage.Heuristic {
			heuristicOnly = false
		}

		if usage.InputTokens > cfg.MaxInputTokens {
			findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("estimated input tokens %d exceeded threshold %d", usage.InputTokens, cfg.MaxInputTokens)})
		}
		if usage.OutputTokens > cfg.MaxOutputTokens {
			findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("estimated output tokens %d exceeded threshold %d", usage.OutputTokens, cfg.MaxOutputTokens)})
		}
		if usage.TotalTokens > cfg.MaxTotalTokens {
			findings = append(findings, core.Finding{Severity: "critical", Message: fmt.Sprintf("estimated total tokens %d exceeded threshold %d", usage.TotalTokens, cfg.MaxTotalTokens)})
		}
		if outputInputRatio > cfg.MaxOutputInputRatio {
			findings = append(findings, core.Finding{Severity: "medium", Message: fmt.Sprintf("output/input token ratio %.2f exceeded threshold %.2f", outputInputRatio, cfg.MaxOutputInputRatio)})
		}
		if promptRatio > cfg.MaxPromptDuplicationRatio {
			findings = append(findings, core.Finding{Severity: "medium", Message: fmt.Sprintf("prompt duplication ratio %.2f exceeded threshold %.2f", promptRatio, cfg.MaxPromptDuplicationRatio)})
		}
		if responseRatio > cfg.MaxResponseDuplicationRatio {
			findings = append(findings, core.Finding{Severity: "medium", Message: fmt.Sprintf("response duplication ratio %.2f exceeded threshold %.2f", responseRatio, cfg.MaxResponseDuplicationRatio)})
		}

		cases = append(cases, core.CaseResult{
			Name:     scenario.Name,
			Passed:   len(findings) == 0,
			Duration: time.Since(start),
			Findings: findings,
			Details: responseDetails(resp, map[string]any{
				"input_tokens":                usage.InputTokens,
				"output_tokens":               usage.OutputTokens,
				"total_tokens":                usage.TotalTokens,
				"heuristic_usage":             usage.Heuristic,
				"output_input_ratio":          round2(outputInputRatio),
				"prompt_duplication_ratio":    round2(promptRatio),
				"response_duplication_ratio":  round2(responseRatio),
				"suggested_max_output_tokens": cfg.SuggestedMaxOutputTokens,
				"estimated_savings_tokens":    savings,
				"optimization_hints":          tokenOptimizationHints(usage, promptRatio, responseRatio, outputInputRatio, cfg),
			}),
		})
	}

	passed := allPassed(cases)
	return core.SuiteResult{
		Name:   "token-optimization",
		Passed: passed,
		Cases:  cases,
		Meta: map[string]any{
			"total_input_tokens":   totalInput,
			"total_output_tokens":  totalOutput,
			"total_tokens":         totalInput + totalOutput,
			"estimated_savings":    totalSavings,
			"heuristic_estimation": heuristicOnly,
		},
	}
}
