package engines

import (
	"math"
	"regexp"
	"strings"

	"cleanr/cleanr/core"
)

func inferTokenUsage(scenario core.Scenario, resp core.Response) core.TokenUsage {
	if resp.Usage.TotalTokens > 0 || resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		usage := resp.Usage
		if usage.TotalTokens == 0 {
			usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		}
		return usage
	}
	inputTokens := estimateTokens(strings.TrimSpace(scenario.System + "\n" + scenario.Input))
	outputTokens := estimateTokens(resp.Text)
	return core.TokenUsage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		Heuristic:    true,
	}
}

func estimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	parts := regexp.MustCompile(`[A-Za-z0-9_]+|[^\sA-Za-z0-9_]`).FindAllString(text, -1)
	total := 0
	for _, part := range parts {
		runes := []rune(part)
		if len(runes) == 1 && !isAlphaNumericRune(runes[0]) {
			total++
			continue
		}
		total += int(math.Ceil(float64(len(runes)) / 4.0))
	}
	if total == 0 {
		return int(math.Ceil(float64(len([]rune(text))) / 4.0))
	}
	return total
}

func duplicationRatio(text string) float64 {
	totalTokens := estimateTokens(text)
	if totalTokens == 0 {
		return 0
	}
	units := splitTokenUnits(text)
	if len(units) == 0 {
		return 0
	}
	seen := map[string]int{}
	duplicatedTokens := 0
	for _, unit := range units {
		key := strings.ToLower(strings.TrimSpace(unit))
		if key == "" {
			continue
		}
		tokens := estimateTokens(key)
		if tokens == 0 {
			continue
		}
		if seen[key] > 0 {
			duplicatedTokens += tokens
		}
		seen[key]++
	}
	return math.Min(float64(duplicatedTokens)/float64(totalTokens), 1)
}

func splitTokenUnits(text string) []string {
	splitter := regexp.MustCompile(`[\n\r]+|[.!?;]+`)
	raw := splitter.Split(text, -1)
	units := make([]string, 0, len(raw))
	for _, unit := range raw {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			continue
		}
		units = append(units, unit)
	}
	return units
}

func estimatedTokenSavings(usage core.TokenUsage, promptRatio, responseRatio float64, cfg core.TokenOptimizationConfig) int {
	savings := 0
	if promptRatio > cfg.MaxPromptDuplicationRatio {
		savings += int(float64(usage.InputTokens) * (promptRatio - cfg.MaxPromptDuplicationRatio))
	}
	if responseRatio > cfg.MaxResponseDuplicationRatio {
		savings += int(float64(usage.OutputTokens) * (responseRatio - cfg.MaxResponseDuplicationRatio))
	}
	if usage.OutputTokens > cfg.SuggestedMaxOutputTokens {
		savings += usage.OutputTokens - cfg.SuggestedMaxOutputTokens
	}
	return max(savings, 0)
}

func tokenOptimizationHints(usage core.TokenUsage, promptRatio, responseRatio, outputInputRatio float64, cfg core.TokenOptimizationConfig) []string {
	hints := make([]string, 0, 4)
	if usage.InputTokens > cfg.MaxInputTokens || promptRatio > cfg.MaxPromptDuplicationRatio {
		hints = append(hints, "deduplicate repeated system instructions and trim low-signal retrieved context")
	}
	if usage.OutputTokens > cfg.MaxOutputTokens || outputInputRatio > cfg.MaxOutputInputRatio {
		hints = append(hints, "add explicit response length caps and require concise output formats")
	}
	if responseRatio > cfg.MaxResponseDuplicationRatio {
		hints = append(hints, "enforce structured answers to reduce repeated or circular completions")
	}
	if usage.TotalTokens > cfg.MaxTotalTokens {
		hints = append(hints, "split large scenarios into smaller task-specific calls or introduce retrieval chunk limits")
	}
	if len(hints) == 0 {
		hints = append(hints, "token profile is within budget")
	}
	return hints
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func isAlphaNumericRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}
