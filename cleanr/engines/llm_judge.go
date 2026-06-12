package engines

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	adapterspkg "github.com/devr-tools/cleanr/cleanr/adapters"
	"github.com/devr-tools/cleanr/cleanr/core"
)

// LLMJudgeEngine grades each target response with a separate judge model.
//
// For every in-scope scenario it invokes the target under test, then asks the
// configured judge provider to score the response against a rubric on a
// 1..Scale Likert scale, optionally comparing against a per-scenario reference
// answer. The case passes when the aggregated normalized score meets MinScore.
//
// To guard against an unreliable judge, the engine supports self-consistency
// sampling: with Samples > 1 it queries the judge repeatedly, gates on the
// median score, and fails the case when the samples disagree by more than
// MaxDisagreement (a judge that contradicts itself is not a trustworthy gate).
type LLMJudgeEngine struct{}

func (LLMJudgeEngine) Name() string { return "llm-judge" }

// judgeTargetFactory builds the judge model client. It is a package var so
// tests can substitute a deterministic judge without standing up a server.
var judgeTargetFactory = func(cfg core.TargetConfig) core.Target {
	return adapterspkg.NewTargetFromConfig(cfg, &http.Client{Timeout: cfg.Timeout()})
}

// baselineTargetFactory builds the pairwise comparison target. Like
// judgeTargetFactory it is a package var so tests can inject a stub baseline.
var baselineTargetFactory = func(cfg core.TargetConfig) core.Target {
	return adapterspkg.NewTargetFromConfig(cfg, &http.Client{Timeout: cfg.Timeout()})
}

type judgeVerdict struct {
	Score     float64 `json:"score"`
	Rationale string  `json:"rationale"`
}

func (e LLMJudgeEngine) Run(ctx context.Context, runCtx *core.RunContext) core.SuiteResult {
	cfg := runCtx.Config.Suites.LLMJudge
	judge := judgeTargetFactory(cfg.Provider)
	scenarios := judgeScenarios(runCtx.Config.Scenarios, cfg.StableTags)
	if cfg.ModeValue() == "pairwise" {
		return e.runPairwise(ctx, runCtx, cfg, judge, scenarios)
	}
	return e.runScore(ctx, runCtx, cfg, judge, scenarios)
}

// preferenceFor maps the judge's chosen slot back to "candidate" or "baseline"
// given which competitor occupied response slot 1 in that ordering.
func preferenceFor(winner string, response1IsCandidate bool) string {
	switch winner {
	case "1":
		if response1IsCandidate {
			return "candidate"
		}
		return "baseline"
	case "2":
		if response1IsCandidate {
			return "baseline"
		}
		return "candidate"
	default:
		return "tie"
	}
}

// judgeScenarios returns the scenarios in scope for grading. When stableTags is
// empty every scenario is graded; otherwise only scenarios carrying at least
// one of the listed tags are included.
func judgeScenarios(scenarios []core.Scenario, stableTags []string) []core.Scenario {
	if len(stableTags) == 0 {
		return scenarios
	}
	want := make(map[string]struct{}, len(stableTags))
	for _, tag := range stableTags {
		want[tag] = struct{}{}
	}
	out := make([]core.Scenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		for _, tag := range scenario.Tags {
			if _, ok := want[tag]; ok {
				out = append(out, scenario)
				break
			}
		}
	}
	return out
}

// judgeCriteria merges the suite-level rubric with any per-scenario additions,
// falling back to a sensible default when none are configured.
func judgeCriteria(global, perScenario []string) []string {
	merged := make([]string, 0, len(global)+len(perScenario))
	seen := make(map[string]struct{})
	for _, c := range append(append([]string{}, global...), perScenario...) {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		key := strings.ToLower(c)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, c)
	}
	if len(merged) == 0 {
		return []string{"The response is correct, helpful, and directly addresses the user's request."}
	}
	return merged
}

func buildJudgePrompt(scenario core.Scenario, response string, criteria []string, reference string, scale int) (string, string) {
	system := fmt.Sprintf(strings.TrimSpace(`
You are a strict, impartial evaluator of an AI assistant's response.
Score the response from 1 to %d against the listed criteria, where 1 is the worst and %d is fully satisfactory.
Judge only against the criteria and the reference answer when provided; do not reward verbosity, style, or confidence.
Return only valid JSON with this exact shape and no markdown fences or commentary:
{"score": <integer 1-%d>, "rationale": "<one or two sentences justifying the score>"}
`), scale, scale, scale)

	var b strings.Builder
	if sys := strings.TrimSpace(scenario.SystemValue()); sys != "" {
		fmt.Fprintf(&b, "Assistant system instructions:\n%s\n\n", sys)
	}
	fmt.Fprintf(&b, "User request:\n%s\n\n", strings.TrimSpace(scenario.InputValue()))
	b.WriteString("Evaluation criteria:\n")
	for i, c := range criteria {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c)
	}
	if reference != "" {
		fmt.Fprintf(&b, "\nReference answer (treat as the correct ground truth):\n%s\n", reference)
	}
	fmt.Fprintf(&b, "\nAssistant response to evaluate:\n%s\n", strings.TrimSpace(response))
	b.WriteString("\nReturn only the JSON object.\n")
	return system, b.String()
}

func parseJudgeVerdict(resp core.Response) (judgeVerdict, bool) {
	raw := strings.TrimSpace(resp.Text)
	if raw == "" && len(resp.Body) > 0 {
		raw = strings.TrimSpace(string(resp.Body))
	}
	if raw == "" {
		return judgeVerdict{}, false
	}
	var verdict judgeVerdict
	if err := json.Unmarshal([]byte(raw), &verdict); err != nil {
		extracted := firstJSONObject(raw)
		if extracted == "" {
			return judgeVerdict{}, false
		}
		if err := json.Unmarshal([]byte(extracted), &verdict); err != nil {
			return judgeVerdict{}, false
		}
	}
	if verdict.Score <= 0 {
		return judgeVerdict{}, false
	}
	return verdict, true
}

func firstJSONObject(raw string) string {
	start := strings.IndexByte(raw, '{')
	end := strings.LastIndexByte(raw, '}')
	if start < 0 || end <= start {
		return ""
	}
	return raw[start : end+1]
}

func clampScore(score float64, scale int) float64 {
	if score < 1 {
		return 1
	}
	if score > float64(scale) {
		return float64(scale)
	}
	return score
}

func judgeModelLabel(provider core.TargetConfig, resp core.Response) string {
	switch provider.TargetType() {
	case "openai":
		if m := strings.TrimSpace(provider.OpenAI.Model); m != "" {
			return "openai/" + m
		}
	case "openai_compatible":
		if m := strings.TrimSpace(provider.OpenAI.Model); m != "" {
			return provider.OpenAI.ProviderValue(provider.TargetType()) + "/" + m
		}
	case "anthropic":
		if m := strings.TrimSpace(provider.Anthropic.Model); m != "" {
			return "anthropic/" + m
		}
	case "mcp":
		if tool := strings.TrimSpace(provider.MCP.Tool); tool != "" {
			return "mcp/" + tool
		}
	}
	if name := strings.TrimSpace(provider.Name); name != "" {
		return name
	}
	return provider.TargetType()
}
