package tests

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	_ "unsafe"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/cleanr/core"
)

//go:linkname judgeTargetFactory github.com/devr-tools/cleanr/cleanr/engines.judgeTargetFactory
var judgeTargetFactory func(core.TargetConfig) core.Target

//go:linkname baselineTargetFactory github.com/devr-tools/cleanr/cleanr/engines.baselineTargetFactory
var baselineTargetFactory func(core.TargetConfig) core.Target

//go:linkname medianScore github.com/devr-tools/cleanr/cleanr/engines.medianScore
func medianScore(scores []float64) float64

//go:linkname scoreSpread github.com/devr-tools/cleanr/cleanr/engines.scoreSpread
func scoreSpread(scores []float64) float64

//go:linkname normalizeWinner github.com/devr-tools/cleanr/cleanr/engines.normalizeWinner
func normalizeWinner(raw []byte) (string, bool)

//go:linkname preferenceFor github.com/devr-tools/cleanr/cleanr/engines.preferenceFor
func preferenceFor(winner string, response1IsCandidate bool) string

type stubTarget struct {
	fn func(core.Request) core.Response
}

func (s stubTarget) Invoke(_ context.Context, req core.Request) core.Response {
	return s.fn(req)
}

func withJudge(t *testing.T, judge core.Target) {
	t.Helper()
	prev := judgeTargetFactory
	judgeTargetFactory = func(core.TargetConfig) core.Target { return judge }
	t.Cleanup(func() { judgeTargetFactory = prev })
}

func withBaseline(t *testing.T, baseline core.Target) {
	t.Helper()
	prev := baselineTargetFactory
	baselineTargetFactory = func(core.TargetConfig) core.Target { return baseline }
	t.Cleanup(func() { baselineTargetFactory = prev })
}

func judgeConfig(judge core.LLMJudgeConfig, scenarios ...core.Scenario) core.Config {
	judge.Enabled = true
	if judge.Scale == 0 {
		judge.Scale = 5
	}
	if judge.MinScore == 0 {
		judge.MinScore = 0.6
	}
	if judge.Samples == 0 {
		judge.Samples = 1
	}
	return core.Config{
		Scenarios: scenarios,
		Suites:    core.SuitesConfig{LLMJudge: judge},
	}
}

func verdictResponse(score float64, rationale string) core.Response {
	return core.Response{StatusCode: 200, Text: fmt.Sprintf(`{"score": %v, "rationale": %q}`, score, rationale)}
}

func okTarget(text string) core.Target {
	return stubTarget{fn: func(core.Request) core.Response {
		return core.Response{StatusCode: 200, Text: text}
	}}
}

func runJudge(cfg core.Config, app core.Target) core.SuiteResult {
	report := cleanr.NewRunner(cfg, app).Run(context.Background())
	return report.Suites[0]
}

func TestLLMJudgePassesOnHighScore(t *testing.T) {
	withJudge(t, stubTarget{fn: func(req core.Request) core.Response {
		if !strings.Contains(req.Prompt, "Explain refunds") {
			t.Fatalf("judge prompt missing user request: %q", req.Prompt)
		}
		return verdictResponse(5, "fully correct and helpful")
	}})
	cfg := judgeConfig(core.LLMJudgeConfig{}, core.Scenario{Name: "refunds", Input: "Explain refunds"})

	result := runJudge(cfg, okTarget("Refunds take 5 days."))

	if !result.Passed || len(result.Cases) != 1 || !result.Cases[0].Passed {
		t.Fatalf("expected pass, got %+v", result)
	}
	if got := result.Cases[0].Details["normalized_score"]; got != 1.0 {
		t.Fatalf("expected normalized_score 1.0, got %v", got)
	}
}

func TestLLMJudgeFailsBelowThreshold(t *testing.T) {
	withJudge(t, stubTarget{fn: func(core.Request) core.Response {
		return verdictResponse(2, "missed the question")
	}})
	cfg := judgeConfig(core.LLMJudgeConfig{}, core.Scenario{Name: "weak", Input: "Explain refunds"})

	result := runJudge(cfg, okTarget("I don't know."))

	if result.Passed || result.Cases[0].Passed {
		t.Fatalf("expected failure for low score, got %+v", result.Cases[0])
	}
	if !hasFindingContaining(result.Cases[0].Findings, "below the") {
		t.Fatalf("expected a below-threshold finding, got %+v", result.Cases[0].Findings)
	}
}

func TestLLMJudgeSelfConsistencyFailsOnDisagreement(t *testing.T) {
	scores := []float64{5, 1, 5}
	i := 0
	withJudge(t, stubTarget{fn: func(core.Request) core.Response {
		s := scores[i%len(scores)]
		i++
		return verdictResponse(s, "varies")
	}})
	cfg := judgeConfig(core.LLMJudgeConfig{Samples: 3, MaxDisagreement: 0.4},
		core.Scenario{Name: "unstable", Input: "Explain refunds"})

	result := runJudge(cfg, okTarget("answer"))

	if result.Cases[0].Passed {
		t.Fatalf("expected failure from judge disagreement, got %+v", result.Cases[0])
	}
	if !hasFindingContaining(result.Cases[0].Findings, "self-consistency") {
		t.Fatalf("expected a self-consistency finding, got %+v", result.Cases[0].Findings)
	}
}

func TestLLMJudgeIncludesReferenceInPrompt(t *testing.T) {
	var captured string
	withJudge(t, stubTarget{fn: func(req core.Request) core.Response {
		captured = req.Prompt
		return verdictResponse(5, "ok")
	}})
	cfg := judgeConfig(core.LLMJudgeConfig{},
		core.Scenario{Name: "ref", Input: "Explain refunds", ReferenceAnswer: "Refunds take five business days."})

	result := runJudge(cfg, okTarget("answer"))

	if !strings.Contains(captured, "Refunds take five business days.") {
		t.Fatalf("reference answer not passed to judge: %q", captured)
	}
	if result.Cases[0].Details["reference_used"] != true {
		t.Fatalf("expected reference_used=true, got %v", result.Cases[0].Details["reference_used"])
	}
}

func TestLLMJudgeFailsOnJudgeError(t *testing.T) {
	withJudge(t, stubTarget{fn: func(core.Request) core.Response {
		return core.Response{Err: errors.New("judge timeout")}
	}})
	cfg := judgeConfig(core.LLMJudgeConfig{}, core.Scenario{Name: "err", Input: "Explain refunds"})

	result := runJudge(cfg, okTarget("answer"))

	if result.Cases[0].Passed {
		t.Fatalf("expected failure when judge errors")
	}
	if !hasFindingContaining(result.Cases[0].Findings, "judge model error") {
		t.Fatalf("expected judge error finding, got %+v", result.Cases[0].Findings)
	}
}

func TestLLMJudgeFailsOnUnparseableVerdict(t *testing.T) {
	withJudge(t, okTarget("the response was pretty good I think"))
	cfg := judgeConfig(core.LLMJudgeConfig{}, core.Scenario{Name: "garbage", Input: "Explain refunds"})

	result := runJudge(cfg, okTarget("answer"))

	if result.Cases[0].Passed {
		t.Fatalf("expected failure on unparseable verdict")
	}
	if !hasFindingContaining(result.Cases[0].Findings, "no parseable score") {
		t.Fatalf("expected parse finding, got %+v", result.Cases[0].Findings)
	}
}

func TestLLMJudgeSkipsGradingOnTargetError(t *testing.T) {
	judgeCalled := false
	withJudge(t, stubTarget{fn: func(core.Request) core.Response {
		judgeCalled = true
		return verdictResponse(5, "ok")
	}})
	cfg := judgeConfig(core.LLMJudgeConfig{}, core.Scenario{Name: "down", Input: "Explain refunds"})
	app := stubTarget{fn: func(core.Request) core.Response { return core.Response{StatusCode: 503} }}

	result := runJudge(cfg, app)

	if result.Cases[0].Passed {
		t.Fatalf("expected failure when target is unavailable")
	}
	if judgeCalled {
		t.Fatalf("judge should not run when the target response is unusable")
	}
}

func TestLLMJudgeStableTagsFilterScope(t *testing.T) {
	withJudge(t, okTarget(`{"score":5,"rationale":"ok"}`))
	cfg := judgeConfig(core.LLMJudgeConfig{StableTags: []string{"graded"}},
		core.Scenario{Name: "in", Input: "a", Tags: []string{"graded"}},
		core.Scenario{Name: "out", Input: "b", Tags: []string{"other"}},
	)

	result := runJudge(cfg, okTarget("answer"))

	if len(result.Cases) != 1 || result.Cases[0].Name != "in" {
		t.Fatalf("expected only the tagged scenario to be graded, got %+v", result.Cases)
	}
}

func TestMedianAndSpread(t *testing.T) {
	if got := medianScore([]float64{1, 5, 5}); got != 5 {
		t.Fatalf("median {1,5,5} = %v, want 5", got)
	}
	if got := medianScore([]float64{2, 4}); got != 3 {
		t.Fatalf("median {2,4} = %v, want 3", got)
	}
	if got := scoreSpread([]float64{2, 5, 3}); got != 3 {
		t.Fatalf("spread = %v, want 3", got)
	}
}

func winnerResponse(winner string) core.Response {
	return core.Response{StatusCode: 200, Text: fmt.Sprintf(`{"winner": %q, "rationale": "because"}`, winner)}
}

func pairwiseConfig(judge core.LLMJudgeConfig, scenarios ...core.Scenario) core.Config {
	judge.Mode = "pairwise"
	judge.Baseline = core.TargetConfig{Type: "http", URL: "http://baseline", PromptField: "p", ResponseField: "r"}
	if judge.MinWinRate == 0 {
		judge.MinWinRate = 0.5
	}
	return judgeConfig(judge, scenarios...)
}

func judgeSlotPicker(t *testing.T, candidateMarker string) core.Target {
	return stubTarget{fn: func(req core.Request) core.Response {
		idx1 := strings.Index(req.Prompt, "Response 1:")
		idx2 := strings.Index(req.Prompt, "Response 2:")
		if idx1 < 0 || idx2 < 0 || idx2 < idx1 {
			t.Fatalf("malformed pairwise prompt: %q", req.Prompt)
		}
		slot1 := req.Prompt[idx1:idx2]
		if strings.Contains(slot1, candidateMarker) {
			return winnerResponse("1")
		}
		return winnerResponse("2")
	}}
}

func TestLLMJudgePairwiseCandidateWins(t *testing.T) {
	withBaseline(t, okTarget("BASELINE answer"))
	withJudge(t, judgeSlotPicker(t, "CANDIDATE"))
	cfg := pairwiseConfig(core.LLMJudgeConfig{Samples: 2},
		core.Scenario{Name: "compare", Input: "Explain refunds"})

	result := runJudge(cfg, okTarget("CANDIDATE answer"))

	c := result.Cases[0]
	if !c.Passed {
		t.Fatalf("expected candidate to win, got %+v", c)
	}
	if c.Details["win_rate"] != 1.0 {
		t.Fatalf("expected win_rate 1.0, got %v", c.Details["win_rate"])
	}
	if c.Details["candidate_wins"] != 2 {
		t.Fatalf("expected 2 candidate wins, got %v", c.Details["candidate_wins"])
	}
}

func TestLLMJudgePairwiseCandidateLosesFails(t *testing.T) {
	withBaseline(t, okTarget("BASELINE answer"))
	withJudge(t, judgeSlotPicker(t, "BASELINE"))
	cfg := pairwiseConfig(core.LLMJudgeConfig{Samples: 2},
		core.Scenario{Name: "compare", Input: "Explain refunds"})

	result := runJudge(cfg, okTarget("CANDIDATE answer"))

	c := result.Cases[0]
	if c.Passed {
		t.Fatalf("expected candidate to lose, got %+v", c)
	}
	if c.Details["win_rate"] != 0.0 {
		t.Fatalf("expected win_rate 0.0, got %v", c.Details["win_rate"])
	}
	if !hasFindingContaining(c.Findings, "below the") {
		t.Fatalf("expected below-threshold finding, got %+v", c.Findings)
	}
}

func TestLLMJudgePairwisePositionBiasIsNeutralized(t *testing.T) {
	withBaseline(t, okTarget("BASELINE answer"))
	withJudge(t, stubTarget{fn: func(core.Request) core.Response { return winnerResponse("1") }})
	cfg := pairwiseConfig(core.LLMJudgeConfig{Samples: 3},
		core.Scenario{Name: "biased", Input: "Explain refunds"})

	result := runJudge(cfg, okTarget("CANDIDATE answer"))

	c := result.Cases[0]
	if c.Passed {
		t.Fatalf("position-biased judge must not pass, got %+v", c)
	}
	if c.Details["position_bias"] != 3 {
		t.Fatalf("expected 3 position-biased comparisons, got %v", c.Details["position_bias"])
	}
	if c.Details["candidate_wins"] != 0 || c.Details["baseline_wins"] != 0 {
		t.Fatalf("expected no decisive wins under position bias, got %+v", c.Details)
	}
	if !hasFindingContaining(c.Findings, "no decisive preference") {
		t.Fatalf("expected no-decisive-preference finding, got %+v", c.Findings)
	}
}

func TestLLMJudgePairwiseRequiresBaseline(t *testing.T) {
	withBaseline(t, okTarget("BASELINE answer"))
	withJudge(t, judgeSlotPicker(t, "CANDIDATE"))
	cfg := pairwiseConfig(core.LLMJudgeConfig{Samples: 5, MinWinRate: 0.6},
		core.Scenario{Name: "compare", Input: "Explain refunds"})

	result := runJudge(cfg, okTarget("CANDIDATE answer"))

	if !result.Cases[0].Passed {
		t.Fatalf("expected pass at win_rate 1.0 >= 0.6, got %+v", result.Cases[0])
	}
}

func TestNormalizeWinner(t *testing.T) {
	cases := map[string]string{`"1"`: "1", `"2"`: "2", `"tie"`: "tie", `1`: "1", `2`: "2", `"A"`: "1", `"B"`: "2"}
	for in, want := range cases {
		got, ok := normalizeWinner([]byte(in))
		if !ok || got != want {
			t.Fatalf("normalizeWinner(%s) = %q,%v; want %q", in, got, ok, want)
		}
	}
	if _, ok := normalizeWinner([]byte(`"maybe"`)); ok {
		t.Fatalf("expected invalid winner to be rejected")
	}
}

func TestPreferenceFor(t *testing.T) {
	if preferenceFor("1", true) != "candidate" || preferenceFor("2", true) != "baseline" {
		t.Fatalf("ordering-1 mapping wrong")
	}
	if preferenceFor("1", false) != "baseline" || preferenceFor("2", false) != "candidate" {
		t.Fatalf("ordering-2 (swapped) mapping wrong")
	}
	if preferenceFor("tie", true) != "tie" {
		t.Fatalf("tie should map to tie")
	}
}

func hasFindingContaining(findings []core.Finding, substr string) bool {
	for _, f := range findings {
		if strings.Contains(f.Message, substr) {
			return true
		}
	}
	return false
}
