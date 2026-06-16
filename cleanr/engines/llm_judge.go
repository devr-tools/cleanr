package engines

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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

// comparisonTargetFactory builds extra targets used by comparison matrices.
var comparisonTargetFactory = func(cfg core.TargetConfig) core.Target {
	return adapterspkg.NewTargetFromConfig(cfg, &http.Client{Timeout: cfg.Timeout()})
}

type judgeVerdict struct {
	Score     float64 `json:"score"`
	Rationale string  `json:"rationale"`
}

type namedTarget struct {
	label  string
	cfg    core.TargetConfig
	target core.Target
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

func buildJudgePrompt(scenario core.Scenario, response string, criteria []string, reference string, scale int, outputs []core.JudgeOutput) (string, string) {
	system := fmt.Sprintf(strings.TrimSpace(`
You are a strict, impartial evaluator of an AI assistant's response.
Score the response from 1 to %d against the listed criteria, where 1 is the worst and %d is fully satisfactory.
Judge only against the criteria and the reference answer when provided; do not reward verbosity, style, or confidence.
Return only valid JSON with this exact shape and no markdown fences or commentary:
{"score": <integer 1-%d>, "rationale": "<one or two sentences justifying the score>"}
`), scale, scale, scale)

	var b strings.Builder
	appendJudgeScenarioContext(&b, scenario)
	b.WriteString("Evaluation criteria:\n")
	for i, c := range criteria {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c)
	}
	if reference != "" {
		fmt.Fprintf(&b, "\nReference answer (treat as the correct ground truth):\n%s\n", reference)
	}
	fmt.Fprintf(&b, "\nAssistant response to evaluate:\n%s\n", strings.TrimSpace(response))
	appendJudgeOutputSection(&b, "Resolved multimodal outputs to inspect", outputs)
	b.WriteString("\nReturn only the JSON object.\n")
	return system, b.String()
}

func appendJudgeScenarioContext(b *strings.Builder, scenario core.Scenario) {
	if sys := strings.TrimSpace(scenario.SystemValue()); sys != "" {
		fmt.Fprintf(b, "Assistant system instructions:\n%s\n\n", sys)
	}
	if len(scenario.Turns) > 0 {
		fmt.Fprintf(b, "Conversation transcript:\n%s\n\n", strings.TrimSpace(scenario.TranscriptText()))
		return
	}
	fmt.Fprintf(b, "User request:\n%s\n", strings.TrimSpace(scenario.InputValue()))
	appendScenarioMediaInputs(b, scenario)
	b.WriteString("\n")
}

func appendScenarioMediaInputs(b *strings.Builder, scenario core.Scenario) {
	lines := make([]string, 0, len(scenario.Images)+len(scenario.Audio)+len(scenario.PDFs))
	lines = append(lines, judgeMediaLines("image", scenario.Images)...)
	lines = append(lines, judgeMediaLines("audio", scenario.Audio)...)
	lines = append(lines, judgeMediaLines("pdf", scenario.PDFs)...)
	if len(lines) == 0 {
		return
	}
	b.WriteString("\nScenario media inputs:\n")
	for _, line := range lines {
		fmt.Fprintf(b, "- %s\n", line)
	}
}

func appendJudgeOutputSection(b *strings.Builder, title string, outputs []core.JudgeOutput) {
	if len(outputs) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s:\n", title)
	for _, output := range outputs {
		fmt.Fprintf(b, "- %s\n", judgeOutputLabel(output))
	}
}

func judgeMediaLines(kind string, items []core.MediaInput) []string {
	if len(items) == 0 {
		return nil
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		parts := []string{kind, mediaLocator(item)}
		if mediaType := strings.TrimSpace(item.MediaType); mediaType != "" {
			parts = append(parts, mediaType)
		}
		if caption := strings.TrimSpace(item.Caption); caption != "" {
			parts = append(parts, fmt.Sprintf("caption=%q", caption))
		}
		lines = append(lines, strings.Join(parts, " | "))
	}
	return lines
}

func mediaLocator(item core.MediaInput) string {
	switch {
	case strings.TrimSpace(item.Filename) != "":
		return strings.TrimSpace(item.Filename)
	case strings.TrimSpace(item.Path) != "":
		return strings.TrimSpace(item.Path)
	case strings.TrimSpace(item.URL) != "":
		return strings.TrimSpace(item.URL)
	case strings.TrimSpace(item.MediaType) != "":
		return strings.TrimSpace(item.MediaType)
	default:
		return "embedded"
	}
}

func judgeOutputLabel(output core.JudgeOutput) string {
	parts := []string{strings.TrimSpace(output.Type)}
	if name := strings.TrimSpace(output.Name); name != "" {
		parts = append(parts, name)
	}
	if mediaType := strings.TrimSpace(output.MediaType); mediaType != "" {
		parts = append(parts, mediaType)
	}
	if path := strings.TrimSpace(output.Path); path != "" {
		parts = append(parts, "path="+path)
	}
	if value := strings.TrimSpace(output.Value); value != "" {
		parts = append(parts, "value="+value)
	}
	return strings.Join(parts, " | ")
}

func resolvedJudgeOutputs(resp core.Response, configured []core.JudgeOutput) []core.JudgeOutput {
	configured = normalizeJudgeOutputs(configured)
	if len(configured) == 0 {
		return nil
	}
	view := judgeResponseView(resp)
	out := make([]core.JudgeOutput, 0, len(configured))
	for _, item := range configured {
		value, ok := resolveJudgePath(view, item.Path)
		if !ok {
			continue
		}
		clone := item
		clone.Value = renderJudgeValue(value)
		if strings.TrimSpace(clone.Value) == "" {
			continue
		}
		out = append(out, clone)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeJudgeOutputs(items []core.JudgeOutput) []core.JudgeOutput {
	if len(items) == 0 {
		return nil
	}
	out := make([]core.JudgeOutput, 0, len(items))
	for _, item := range items {
		normalized := core.JudgeOutput{
			Name:      strings.TrimSpace(item.Name),
			Type:      strings.ToLower(strings.TrimSpace(item.Type)),
			Path:      strings.TrimSpace(item.Path),
			Value:     strings.TrimSpace(item.Value),
			MediaType: strings.TrimSpace(item.MediaType),
		}
		if normalized.Type == "" && normalized.Path == "" && normalized.Value == "" && normalized.Name == "" && normalized.MediaType == "" {
			continue
		}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func judgeResponseView(resp core.Response) map[string]any {
	return map[string]any{
		"response": map[string]any{
			"text":        strings.TrimSpace(resp.Text),
			"status_code": resp.StatusCode,
			"body":        decodeJudgeBody(resp.Body, resp.Text),
			"normalized":  normalizeJudgeProviderResponse(resp.Normalized),
		},
	}
}

func decodeJudgeBody(body []byte, text string) any {
	if len(body) > 0 {
		var payload any
		if err := json.Unmarshal(body, &payload); err == nil {
			return payload
		}
		return string(body)
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	var payload any
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
		return payload
	}
	return trimmed
}

func normalizeJudgeProviderResponse(normalized core.ProviderResponse) map[string]any {
	raw, err := json.Marshal(normalized)
	if err != nil {
		return map[string]any{}
	}
	var view map[string]any
	if err := json.Unmarshal(raw, &view); err != nil {
		return map[string]any{}
	}
	return view
}

func resolveJudgePath(root any, path string) (any, bool) {
	current := root
	for _, segment := range strings.Split(strings.TrimSpace(path), ".") {
		if segment == "" {
			return nil, false
		}
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func renderJudgeValue(v any) string {
	switch typed := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return strings.TrimSpace(fmt.Sprint(typed))
		}
		return string(data)
	}
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
