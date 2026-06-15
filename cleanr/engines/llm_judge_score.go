package engines

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type scoreRun struct {
	scale           int
	samples         int
	minScore        float64
	maxDisagreement float64
}

type scoreCaseInput struct {
	runCtx   *core.RunContext
	cfg      core.LLMJudgeConfig
	judge    core.Target
	scenario core.Scenario
	run      scoreRun
}

type scoreSampleInput struct {
	cfg       core.LLMJudgeConfig
	scenario  core.Scenario
	response  string
	criteria  []string
	reference string
	run       scoreRun
}

type scoreSamples struct {
	scores        []float64
	rationales    []string
	parseFailures int
	judgeErr      error
}

func (e LLMJudgeEngine) runScore(ctx context.Context, runCtx *core.RunContext, cfg core.LLMJudgeConfig, judge core.Target, scenarios []core.Scenario) core.SuiteResult {
	run := scoreRun{
		scale:           cfg.ScaleValue(),
		samples:         cfg.SamplesValue(),
		minScore:        cfg.MinScore,
		maxDisagreement: cfg.MaxDisagreement,
	}
	cases := make([]core.CaseResult, 0, len(scenarios))
	for _, scenario := range scenarios {
		cases = append(cases, e.runScoreCase(ctx, scoreCaseInput{
			runCtx:   runCtx,
			cfg:      cfg,
			judge:    judge,
			scenario: scenario,
			run:      run,
		}))
	}
	return core.SuiteResult{Name: "llm-judge", Passed: allPassed(cases), Cases: cases}
}

func (e LLMJudgeEngine) runScoreCase(ctx context.Context, in scoreCaseInput) core.CaseResult {
	start := time.Now()
	resp := in.runCtx.Target.Invoke(ctx, scenarioRequest(in.scenario, in.runCtx.Config.Target.Timeout()))
	findings := responseFindings(resp, nil)
	if resp.Err != nil || resp.StatusCode >= 500 {
		return core.CaseResult{
			Name:     in.scenario.Name,
			Passed:   false,
			Duration: time.Since(start),
			Findings: findings,
			Details: responseDetails(resp, map[string]any{
				"judge_skipped": "target response unavailable for grading",
			}),
		}
	}

	criteria := judgeCriteria(in.cfg.Criteria, in.scenario.Rubric)
	reference := strings.TrimSpace(in.scenario.ReferenceAnswer)
	details := map[string]any{
		"judge_model":    judgeModelLabel(in.cfg.Provider, resp),
		"scale":          in.run.scale,
		"min_score":      in.run.minScore,
		"samples":        in.run.samples,
		"criteria":       criteria,
		"reference_used": reference != "",
		"response":       trimForReport(resp.Text),
	}
	samples := collectScoreSamples(ctx, in.judge, scoreSampleInput{
		cfg:       in.cfg,
		scenario:  in.scenario,
		response:  resp.Text,
		criteria:  criteria,
		reference: reference,
		run:       in.run,
	})
	passed, findings := finalizeScoreCase(in.run, details, &findings, samples)

	return core.CaseResult{
		Name:     in.scenario.Name,
		Passed:   passed,
		Duration: time.Since(start),
		Findings: findings,
		Details:  responseDetails(resp, details),
	}
}

func collectScoreSamples(ctx context.Context, judge core.Target, in scoreSampleInput) scoreSamples {
	system, prompt := buildJudgePrompt(in.scenario, in.response, in.criteria, in.reference, in.run.scale)
	out := scoreSamples{
		scores:     make([]float64, 0, in.run.samples),
		rationales: make([]string, 0, in.run.samples),
	}
	for i := 0; i < in.run.samples; i++ {
		jresp := judge.Invoke(ctx, core.Request{
			System:  system,
			Prompt:  prompt,
			Timeout: in.cfg.Provider.Timeout(),
		})
		if jresp.Err != nil {
			out.judgeErr = jresp.Err
			return out
		}
		verdict, ok := parseJudgeVerdict(jresp)
		if !ok {
			out.parseFailures++
			continue
		}
		out.scores = append(out.scores, clampScore(verdict.Score, in.run.scale))
		if rationale := strings.TrimSpace(verdict.Rationale); rationale != "" {
			out.rationales = append(out.rationales, rationale)
		}
	}
	return out
}

func finalizeScoreCase(run scoreRun, details map[string]any, findings *[]core.Finding, samples scoreSamples) (bool, []core.Finding) {
	appendScoreFindings(findings, run.samples, samples)
	if len(samples.scores) == 0 {
		return false, *findings
	}

	median := medianScore(samples.scores)
	normalized := round3(median / float64(run.scale))
	disagreement := round3(scoreSpread(samples.scores) / float64(run.scale))
	details["raw_scores"] = samples.scores
	details["median_score"] = median
	details["normalized_score"] = normalized
	details["disagreement"] = disagreement
	if len(samples.rationales) > 0 {
		details["rationale"] = samples.rationales[0]
		details["rationales"] = samples.rationales
	}
	if samples.parseFailures > 0 {
		details["parse_failures"] = samples.parseFailures
	}

	passed := normalized >= run.minScore
	if !passed {
		msg := fmt.Sprintf("judge score %.2f/%d (normalized %.2f) is below the %.2f threshold", median, run.scale, normalized, run.minScore)
		if len(samples.rationales) > 0 {
			msg += ": " + trimForReport(samples.rationales[0])
		}
		*findings = append(*findings, core.Finding{Severity: "high", Message: msg})
	}
	if run.samples > 1 && run.maxDisagreement > 0 && disagreement > run.maxDisagreement {
		passed = false
		*findings = append(*findings, core.Finding{
			Severity: "medium",
			Message:  fmt.Sprintf("judge self-consistency spread %.2f exceeds max_disagreement %.2f across %d samples", disagreement, run.maxDisagreement, run.samples),
		})
	}
	return passed, *findings
}

func appendScoreFindings(findings *[]core.Finding, sampleCount int, samples scoreSamples) {
	if samples.judgeErr != nil {
		*findings = append(*findings, core.Finding{
			Severity: "high",
			Message:  fmt.Sprintf("judge model error: %s", samples.judgeErr.Error()),
		})
	}
	if len(samples.scores) == 0 && samples.judgeErr == nil {
		*findings = append(*findings, core.Finding{
			Severity: "high",
			Message:  fmt.Sprintf("judge returned no parseable score across %d sample(s)", sampleCount),
		})
	}
	if samples.parseFailures > 0 {
		*findings = append(*findings, core.Finding{
			Severity: "low",
			Message:  fmt.Sprintf("%d of %d judge samples were unparseable", samples.parseFailures, sampleCount),
		})
	}
}

func medianScore(scores []float64) float64 {
	if len(scores) == 0 {
		return 0
	}
	cp := append([]float64(nil), scores...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}

func scoreSpread(scores []float64) float64 {
	if len(scores) == 0 {
		return 0
	}
	lo, hi := scores[0], scores[0]
	for _, s := range scores[1:] {
		if s < lo {
			lo = s
		}
		if s > hi {
			hi = s
		}
	}
	return hi - lo
}
