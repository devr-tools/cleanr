package engines

import (
	"context"
	"fmt"
	"math"
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
	confidenceLevel float64
	minPassRate     float64
	maxFlakeRate    float64
	cascadeMargin   float64
}

type scoreCaseInput struct {
	runCtx   *core.RunContext
	cfg      core.LLMJudgeConfig
	judge    core.Target
	judges   []namedTarget
	scenario core.Scenario
	run      scoreRun
}

type scoreSampleInput struct {
	cfg       core.LLMJudgeConfig
	scenario  core.Scenario
	response  string
	criteria  []string
	reference string
	outputs   []core.JudgeOutput
	run       scoreRun
}

type scoreSamples struct {
	scores           []float64
	rationales       []string
	parseFailures    int
	judgeErr         error
	cascadeTriggered int
}

func (e LLMJudgeEngine) runScore(ctx context.Context, runCtx *core.RunContext, cfg core.LLMJudgeConfig, judge core.Target, scenarios []core.Scenario) core.SuiteResult {
	run := scoreRun{
		scale:           cfg.ScaleValue(),
		samples:         cfg.SamplesValue(),
		minScore:        cfg.MinScoreValue(),
		maxDisagreement: cfg.MaxDisagreement,
		confidenceLevel: cfg.ConfidenceLevelValue(),
		minPassRate:     cfg.MinPassRate,
		maxFlakeRate:    cfg.MaxFlakeRate,
		cascadeMargin:   cfg.CascadeMargin,
	}
	// Build the judge pool once for the whole run instead of once per case; each
	// ensemble entry otherwise allocates a fresh http.Client on every scenario.
	judges := buildJudgePool(cfg, judge)
	cases := make([]core.CaseResult, 0, len(scenarios))
	for _, scenario := range scenarios {
		if ctx.Err() != nil {
			break
		}
		cases = append(cases, e.runScoreCase(ctx, scoreCaseInput{
			runCtx:   runCtx,
			cfg:      cfg,
			judge:    judge,
			judges:   judges,
			scenario: scenario,
			run:      run,
		}))
	}
	return applyJudgePostAnalysis(ctx, runCtx, cfg, judge, scenarios, core.SuiteResult{Name: "llm-judge", Passed: allPassed(cases), Cases: cases})
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
	outputs := resolvedJudgeOutputs(resp, in.scenario.JudgeOutputs)
	details := map[string]any{
		"judge_model":    judgeModelLabel(in.cfg.Provider, resp),
		"scale":          in.run.scale,
		"min_score":      in.run.minScore,
		"samples":        in.run.samples,
		"criteria":       criteria,
		"reference_used": reference != "",
		"response":       trimForReport(resp.Text),
	}
	if len(outputs) > 0 {
		details["judge_outputs"] = outputs
	}
	judges := in.judges
	if judges == nil {
		judges = buildJudgePool(in.cfg, in.judge)
	}
	samples := collectScoreSamples(ctx, judges, scoreSampleInput{
		cfg:       in.cfg,
		scenario:  in.scenario,
		response:  resp.Text,
		criteria:  criteria,
		reference: reference,
		outputs:   outputs,
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

func collectScoreSamples(ctx context.Context, judges []namedTarget, in scoreSampleInput) scoreSamples {
	system, prompt := buildJudgePrompt(in.scenario, in.response, in.criteria, in.reference, in.run.scale, in.outputs)
	out := scoreSamples{
		scores:     make([]float64, 0, in.run.samples),
		rationales: make([]string, 0, in.run.samples),
	}
	for i := 0; i < in.run.samples; i++ {
		score, rationale, cascadeTriggered, ok, err := collectScoreSample(ctx, judges, system, prompt, in)
		if err != nil {
			out.judgeErr = err
			return out
		}
		if !ok {
			out.parseFailures++
			continue
		}
		out.scores = append(out.scores, score)
		if rationale != "" {
			out.rationales = append(out.rationales, rationale)
		}
		if cascadeTriggered {
			out.cascadeTriggered++
		}
	}
	return out
}

func collectScoreSample(ctx context.Context, judges []namedTarget, system, prompt string, in scoreSampleInput) (float64, string, bool, bool, error) {
	scores := make([]float64, 0, len(judges))
	rationales := make([]string, 0, len(judges))
	cascadeTriggered := false
	for idx, item := range judges {
		jresp := item.target.Invoke(ctx, core.Request{
			Scenario:     in.scenario,
			System:       system,
			Prompt:       prompt,
			Messages:     in.scenario.TurnsValue(),
			Images:       in.scenario.Images,
			Audio:        in.scenario.Audio,
			PDFs:         in.scenario.PDFs,
			JudgeOutputs: in.outputs,
			Timeout:      item.cfg.Timeout(),
		})
		if jresp.Err != nil {
			return 0, "", false, false, jresp.Err
		}
		verdict, ok := parseJudgeVerdict(jresp)
		if !ok {
			return 0, "", false, false, nil
		}
		score := clampScore(verdict.Score, in.run.scale)
		scores = append(scores, score)
		if rationale := strings.TrimSpace(verdict.Rationale); rationale != "" {
			rationales = append(rationales, rationale)
		}
		if idx == 0 && len(judges) > 1 && in.run.cascadeMargin > 0 {
			normalized := score / float64(in.run.scale)
			if math.Abs(normalized-in.run.minScore) > in.run.cascadeMargin {
				break
			}
			cascadeTriggered = true
		}
	}
	if len(scores) == 0 {
		return 0, "", false, false, nil
	}
	return medianScore(scores), firstString(rationales), cascadeTriggered, true, nil
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
	if samples.cascadeTriggered > 0 {
		details["cascade_triggered_samples"] = samples.cascadeTriggered
	}

	passes := 0
	for _, sample := range samples.scores {
		if round4(sample/float64(run.scale)) >= run.minScore {
			passes++
		}
	}
	interval := passRateCI(passes, len(samples.scores), run.confidenceLevel)
	flake := flakeRate(passes, len(samples.scores))
	details["pass_samples"] = passes
	details["fail_samples"] = len(samples.scores) - passes
	details["pass_rate"] = interval.rate
	details["pass_rate_ci"] = []float64{interval.lowerBound, interval.upperBound}
	details["flake_rate"] = flake
	details["flake_detected"] = flake > 0

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
	if run.minPassRate > 0 && len(samples.scores) > 0 && interval.lowerBound < run.minPassRate {
		passed = false
		*findings = append(*findings, core.Finding{
			Severity: "medium",
			Message:  fmt.Sprintf("judge pass-rate lower bound %.2f fell below min_pass_rate %.2f at confidence %.2f", interval.lowerBound, run.minPassRate, run.confidenceLevel),
		})
	}
	if run.maxFlakeRate > 0 && len(samples.scores) > 1 && flake > run.maxFlakeRate {
		passed = false
		*findings = append(*findings, core.Finding{
			Severity: "medium",
			Message:  fmt.Sprintf("judge flake rate %.2f exceeded max_flake_rate %.2f", flake, run.maxFlakeRate),
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

func firstString(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
}
