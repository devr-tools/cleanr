package engines

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type pairwiseRun struct {
	samples         int
	minWinRate      float64
	confidenceLevel float64
	minPassRate     float64
	maxFlakeRate    float64
	cascadeMargin   float64
}

type pairwiseCaseInput struct {
	runCtx   *core.RunContext
	cfg      core.LLMJudgeConfig
	judge    core.Target
	judges   []namedTarget
	baseline core.Target
	scenario core.Scenario
	run      pairwiseRun
}

type pairwiseInput struct {
	scenario  core.Scenario
	criteria  []string
	reference string
	response1 string
	outputs1  []core.JudgeOutput
	response2 string
	outputs2  []core.JudgeOutput
}

type pairwiseSamples struct {
	candidateWins    int
	baselineWins     int
	ties             int
	positionBias     int
	parseFailures    int
	rationales       []string
	judgeErr         error
	cascadeTriggered int
}

func (e LLMJudgeEngine) runPairwise(ctx context.Context, runCtx *core.RunContext, cfg core.LLMJudgeConfig, judge core.Target, scenarios []core.Scenario) core.SuiteResult {
	run := pairwiseRun{
		samples:         cfg.SamplesValue(),
		minWinRate:      cfg.MinWinRate,
		confidenceLevel: cfg.ConfidenceLevelValue(),
		minPassRate:     cfg.MinPassRate,
		maxFlakeRate:    cfg.MaxFlakeRate,
		cascadeMargin:   cfg.CascadeMargin,
	}
	baseline := baselineTargetFactory(cfg.Baseline)
	// Build the judge pool once for the run rather than once per case.
	judges := buildJudgePool(cfg, judge)
	cases := make([]core.CaseResult, 0, len(scenarios))
	for _, scenario := range scenarios {
		if ctx.Err() != nil {
			break
		}
		cases = append(cases, e.runPairwiseCase(ctx, pairwiseCaseInput{
			runCtx:   runCtx,
			cfg:      cfg,
			judge:    judge,
			judges:   judges,
			baseline: baseline,
			scenario: scenario,
			run:      run,
		}))
	}
	return applyJudgePostAnalysis(ctx, runCtx, cfg, judge, scenarios, core.SuiteResult{Name: "llm-judge", Passed: allPassed(cases), Cases: cases})
}

func (e LLMJudgeEngine) runPairwiseCase(ctx context.Context, in pairwiseCaseInput) core.CaseResult {
	start := time.Now()
	candidateResp := in.runCtx.Target.Invoke(ctx, scenarioRequest(in.scenario, in.runCtx.Config.Target.Timeout()))
	baselineResp := in.baseline.Invoke(ctx, scenarioRequest(in.scenario, in.cfg.Baseline.Timeout()))
	findings := responseFindings(candidateResp, nil)
	details := map[string]any{
		"judge_model":        judgeModelLabel(in.cfg.Provider, candidateResp),
		"mode":               "pairwise",
		"baseline_model":     judgeModelLabel(in.cfg.Baseline, baselineResp),
		"samples":            in.run.samples,
		"min_win_rate":       in.run.minWinRate,
		"candidate_response": trimForReport(candidateResp.Text),
		"baseline_response":  trimForReport(baselineResp.Text),
	}
	if candidateResp.Err != nil || candidateResp.StatusCode >= 500 || baselineResp.Err != nil || baselineResp.StatusCode >= 500 {
		findings = append(findings, core.Finding{Severity: "high", Message: "candidate or baseline response unavailable for comparison"})
		return core.CaseResult{
			Name:     in.scenario.Name,
			Passed:   false,
			Duration: time.Since(start),
			Findings: findings,
			Details:  responseDetails(candidateResp, details),
		}
	}

	input := pairwiseInput{
		scenario:  in.scenario,
		criteria:  judgeCriteria(in.cfg.Criteria, in.scenario.Rubric),
		reference: strings.TrimSpace(in.scenario.ReferenceAnswer),
		response1: candidateResp.Text,
		outputs1:  resolvedJudgeOutputs(candidateResp, in.scenario.JudgeOutputs),
		response2: baselineResp.Text,
		outputs2:  resolvedJudgeOutputs(baselineResp, in.scenario.JudgeOutputs),
	}
	if len(input.outputs1) > 0 {
		details["candidate_judge_outputs"] = input.outputs1
	}
	if len(input.outputs2) > 0 {
		details["baseline_judge_outputs"] = input.outputs2
	}
	judges := in.judges
	if judges == nil {
		judges = buildJudgePool(in.cfg, in.judge)
	}
	samples := collectPairwiseSamples(ctx, judges, in.cfg.Provider.Timeout(), input, in.run.samples)
	passed := finalizePairwiseCase(in.run, details, &findings, samples)
	return core.CaseResult{
		Name:     in.scenario.Name,
		Passed:   passed,
		Duration: time.Since(start),
		Findings: findings,
		Details:  responseDetails(candidateResp, details),
	}
}

func collectPairwiseSamples(ctx context.Context, judges []namedTarget, timeout time.Duration, input pairwiseInput, sampleCount int) pairwiseSamples {
	out := pairwiseSamples{rationales: make([]string, 0, sampleCount)}
	for i := 0; i < sampleCount; i++ {
		w1, r1, cascadeTriggered, ok1, err1 := pairwiseDecisionSet(ctx, judges, timeout, input, false)
		if err1 != nil {
			out.judgeErr = err1
			return out
		}
		w2, r2, _, ok2, err2 := pairwiseDecisionSet(ctx, judges, timeout, input, true)
		if err2 != nil {
			out.judgeErr = err2
			return out
		}
		if !ok1 || !ok2 {
			out.parseFailures++
			continue
		}
		if cascadeTriggered {
			out.cascadeTriggered++
		}
		appendPairwiseRationales(&out, r1, r2)
		recordPairwiseOutcome(&out, w1, w2)
	}
	return out
}

func appendPairwiseRationales(out *pairwiseSamples, rationales ...string) {
	for _, rationale := range rationales {
		if trimmed := strings.TrimSpace(rationale); trimmed != "" {
			out.rationales = append(out.rationales, trimmed)
		}
	}
}

func recordPairwiseOutcome(out *pairwiseSamples, winner1, winner2 string) {
	pref1 := preferenceFor(winner1, true)
	pref2 := preferenceFor(winner2, false)
	switch {
	case pref1 == "candidate" && pref2 == "candidate":
		out.candidateWins++
	case pref1 == "baseline" && pref2 == "baseline":
		out.baselineWins++
	default:
		out.ties++
		if winner1 == winner2 && (winner1 == "1" || winner1 == "2") {
			out.positionBias++
		}
	}
}

func finalizePairwiseCase(run pairwiseRun, details map[string]any, findings *[]core.Finding, samples pairwiseSamples) bool {
	decisive := samples.candidateWins + samples.baselineWins
	winRate := 0.0
	if decisive > 0 {
		winRate = round3(float64(samples.candidateWins) / float64(decisive))
	}
	details["candidate_wins"] = samples.candidateWins
	details["baseline_wins"] = samples.baselineWins
	details["ties"] = samples.ties
	details["position_bias"] = samples.positionBias
	details["win_rate"] = winRate
	if samples.cascadeTriggered > 0 {
		details["cascade_triggered_samples"] = samples.cascadeTriggered
	}
	if len(samples.rationales) > 0 {
		details["rationale"] = samples.rationales[0]
		details["rationales"] = samples.rationales
	}
	if samples.parseFailures > 0 {
		details["parse_failures"] = samples.parseFailures
	}
	appendPairwiseFindings(findings, run.samples, samples)
	totalDecisions := samples.candidateWins + samples.baselineWins + samples.ties
	interval := passRateCI(samples.candidateWins, totalDecisions, run.confidenceLevel)
	flake := flakeRate(samples.candidateWins, totalDecisions)
	details["pass_samples"] = samples.candidateWins
	details["fail_samples"] = totalDecisions - samples.candidateWins
	details["pass_rate"] = interval.rate
	details["pass_rate_ci"] = []float64{interval.lowerBound, interval.upperBound}
	details["flake_rate"] = flake
	details["flake_detected"] = flake > 0

	switch {
	case samples.judgeErr != nil:
		return false
	case samples.candidateWins+samples.baselineWins+samples.ties == 0:
		*findings = append(*findings, core.Finding{
			Severity: "high",
			Message:  fmt.Sprintf("judge returned no parseable pairwise decision across %d sample(s)", run.samples),
		})
		return false
	case decisive == 0:
		*findings = append(*findings, core.Finding{
			Severity: "medium",
			Message:  "no decisive preference between candidate and baseline (all comparisons tied or position-biased)",
		})
		return false
	}

	passed := winRate >= run.minWinRate
	if !passed {
		msg := fmt.Sprintf(
			"candidate win rate %.2f over baseline is below the %.2f threshold (%d candidate / %d baseline / %d tie)",
			winRate, run.minWinRate, samples.candidateWins, samples.baselineWins, samples.ties,
		)
		if len(samples.rationales) > 0 {
			msg += ": " + trimForReport(samples.rationales[0])
		}
		*findings = append(*findings, core.Finding{Severity: "high", Message: msg})
	}
	if run.minPassRate > 0 && totalDecisions > 0 && interval.lowerBound < run.minPassRate {
		passed = false
		*findings = append(*findings, core.Finding{
			Severity: "medium",
			Message:  fmt.Sprintf("pairwise pass-rate lower bound %.2f fell below min_pass_rate %.2f at confidence %.2f", interval.lowerBound, run.minPassRate, run.confidenceLevel),
		})
	}
	if run.maxFlakeRate > 0 && totalDecisions > 1 && flake > run.maxFlakeRate {
		passed = false
		*findings = append(*findings, core.Finding{
			Severity: "medium",
			Message:  fmt.Sprintf("pairwise flake rate %.2f exceeded max_flake_rate %.2f", flake, run.maxFlakeRate),
		})
	}
	return passed
}

func appendPairwiseFindings(findings *[]core.Finding, sampleCount int, samples pairwiseSamples) {
	if samples.judgeErr != nil {
		*findings = append(*findings, core.Finding{
			Severity: "high",
			Message:  fmt.Sprintf("judge model error: %s", samples.judgeErr.Error()),
		})
	}
	if samples.parseFailures > 0 {
		*findings = append(*findings, core.Finding{
			Severity: "low",
			Message:  fmt.Sprintf("%d of %d pairwise samples were unparseable", samples.parseFailures, sampleCount),
		})
	}
	if samples.positionBias > 0 {
		*findings = append(*findings, core.Finding{
			Severity: "low",
			Message:  fmt.Sprintf("judge showed position bias in %d of %d comparisons (excluded from the win rate)", samples.positionBias, sampleCount),
		})
	}
}

// pairwiseDecision asks the judge which of two responses is better and returns
// the chosen slot ("1", "2", or "tie") with its rationale.
func pairwiseDecision(ctx context.Context, judge core.Target, timeout time.Duration, input pairwiseInput) (string, string, bool, error) {
	system, prompt := buildPairwisePrompt(input.scenario, input.criteria, input.reference, input.response1, input.outputs1, input.response2, input.outputs2)
	jresp := judge.Invoke(ctx, core.Request{
		Scenario: input.scenario,
		System:   system,
		Prompt:   prompt,
		Messages: input.scenario.TurnsValue(),
		Images:   input.scenario.Images,
		Audio:    input.scenario.Audio,
		PDFs:     input.scenario.PDFs,
		Timeout:  timeout,
	})
	if jresp.Err != nil {
		return "", "", false, jresp.Err
	}
	winner, rationale, ok := parsePairwiseVerdict(jresp)
	return winner, rationale, ok, nil
}

func pairwiseDecisionSet(ctx context.Context, judges []namedTarget, timeout time.Duration, input pairwiseInput, swapped bool) (string, string, bool, bool, error) {
	ordered := input
	if swapped {
		ordered.response1, ordered.response2 = input.response2, input.response1
		ordered.outputs1, ordered.outputs2 = input.outputs2, input.outputs1
	}
	decisions := make([]string, 0, len(judges))
	rationales := make([]string, 0, len(judges))
	cascadeTriggered := false
	for idx, item := range judges {
		winner, rationale, ok, err := pairwiseDecision(ctx, item.target, item.cfg.Timeout(), ordered)
		if err != nil {
			return "", "", false, false, err
		}
		if !ok {
			return "", "", false, false, nil
		}
		decisions = append(decisions, winner)
		if strings.TrimSpace(rationale) != "" {
			rationales = append(rationales, rationale)
		}
		if idx == 0 && len(judges) > 1 && winner != "tie" {
			break
		}
		if idx == 0 && len(judges) > 1 && winner == "tie" {
			cascadeTriggered = true
		}
	}
	return majorityWinner(decisions), firstString(rationales), cascadeTriggered, true, nil
}

func majorityWinner(decisions []string) string {
	if len(decisions) == 0 {
		return "tie"
	}
	counts := map[string]int{}
	best := decisions[0]
	for _, decision := range decisions {
		counts[decision]++
		if counts[decision] > counts[best] {
			best = decision
		}
	}
	return best
}

func buildPairwisePrompt(scenario core.Scenario, criteria []string, reference, response1 string, outputs1 []core.JudgeOutput, response2 string, outputs2 []core.JudgeOutput) (string, string) {
	system := strings.TrimSpace(`
You are a strict, impartial evaluator comparing two AI assistant responses to the same request.
Decide which response better satisfies the criteria. Ignore the order they are presented in, their length, and their style; judge only substance against the criteria and the reference answer when provided.
If the two responses are equally good, answer "tie".
Return only valid JSON with this exact shape and no markdown fences or commentary:
{"winner": "1" | "2" | "tie", "rationale": "<one or two sentences justifying the choice>"}
`)

	var b strings.Builder
	appendJudgeScenarioContext(&b, scenario)
	b.WriteString("Evaluation criteria:\n")
	for i, c := range criteria {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c)
	}
	if reference != "" {
		fmt.Fprintf(&b, "\nReference answer (treat as the correct ground truth):\n%s\n", reference)
	}
	fmt.Fprintf(&b, "\nResponse 1:\n%s\n", strings.TrimSpace(response1))
	appendJudgeOutputSection(&b, "Response 1 multimodal outputs", outputs1)
	fmt.Fprintf(&b, "\nResponse 2:\n%s\n", strings.TrimSpace(response2))
	appendJudgeOutputSection(&b, "Response 2 multimodal outputs", outputs2)
	b.WriteString("\nReturn only the JSON object.\n")
	return system, b.String()
}

type pairwiseVerdict struct {
	Winner    json.RawMessage `json:"winner"`
	Rationale string          `json:"rationale"`
}

func parsePairwiseVerdict(resp core.Response) (string, string, bool) {
	raw := strings.TrimSpace(resp.Text)
	if raw == "" && len(resp.Body) > 0 {
		raw = strings.TrimSpace(string(resp.Body))
	}
	if raw == "" {
		return "", "", false
	}
	var verdict pairwiseVerdict
	if err := json.Unmarshal([]byte(raw), &verdict); err != nil {
		extracted := firstJSONObject(raw)
		if extracted == "" {
			return "", "", false
		}
		if err := json.Unmarshal([]byte(extracted), &verdict); err != nil {
			return "", "", false
		}
	}
	winner, ok := normalizeWinner(verdict.Winner)
	if !ok {
		return "", "", false
	}
	return winner, verdict.Rationale, true
}

// normalizeWinner accepts the winner as a JSON string or number and maps it to
// "1", "2", or "tie".
func normalizeWinner(raw json.RawMessage) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(strings.Trim(string(raw), `"`)))
	switch s {
	case "1", "a", "first":
		return "1", true
	case "2", "b", "second":
		return "2", true
	case "tie", "draw", "equal", "neither", "0":
		return "tie", true
	default:
		return "", false
	}
}
