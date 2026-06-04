package integrations

import (
	"encoding/json"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func buildReviewedEntry(item ScenarioDatasetEntry, existingByName map[string]core.Scenario, existingByBody map[string][]core.Scenario) ReviewedScenarioEntry {
	var existing *core.Scenario
	if matched, ok := existingByName[item.Scenario.Name]; ok {
		copyScenario := matched
		existing = &copyScenario
	}
	diff := buildReviewDiff(item.Scenario, existingByName, existingByBody)
	analysis := buildReviewAnalysis(item, diff)
	return ReviewedScenarioEntry{
		Entry:            item,
		ExistingScenario: existing,
		Diff:             diff,
		Analysis:         analysis,
	}
}

func buildReviewAnalysis(item ScenarioDatasetEntry, diff DatasetReviewDiff) DatasetReviewAnalysis {
	highestSeverity, severityScore := highestSeverity(item.Origin.Findings)
	noveltyScore := reviewNoveltyScore(diff.Status)
	duplicatePenalty, exactDuplicate := reviewDuplicatePenalty(diff.Status)
	stableReasons, stableScore := stableSuitabilityReasons(item, diff, highestSeverity)
	stableSuitable := stableScore >= 8
	stableSuitability := reviewStableSuitability(stableScore)
	usefulness := severityScore + noveltyScore + stableScore - duplicatePenalty
	return DatasetReviewAnalysis{
		HighestSeverity:      highestSeverity,
		SeverityScore:        severityScore,
		NoveltyScore:         noveltyScore,
		DuplicatePenalty:     duplicatePenalty,
		StableSuitability:    stableSuitability,
		StableSuitable:       stableSuitable,
		StableReasons:        stableReasons,
		UsefulnessScore:      usefulness,
		ExactDuplicate:       exactDuplicate,
		PromoteStableDefault: stableSuitable && diff.Status != "duplicate" && diff.Status != "unchanged",
	}
}

func reviewNoveltyScore(status string) int {
	switch status {
	case "new":
		return 30
	case "modified":
		return 18
	case "duplicate":
		return 4
	default:
		return 0
	}
}

func reviewDuplicatePenalty(status string) (int, bool) {
	switch status {
	case "duplicate":
		return 20, true
	case "unchanged":
		return 28, true
	default:
		return 0, false
	}
}

func stableSuitabilityReasons(item ScenarioDatasetEntry, diff DatasetReviewDiff, highestSeverity string) ([]string, int) {
	reasons := []string{}
	score := 0
	if len(item.Scenario.Assertions) > 0 {
		reasons = append(reasons, "has explicit assertions")
		score += 8
	}
	if len(item.Scenario.ExpectedContains) > 0 || len(item.Scenario.ForbiddenContains) > 0 {
		reasons = append(reasons, "has expected or forbidden output markers")
		score += 6
	}
	if containsString(item.Scenario.Tags, "generated") {
		reasons = append(reasons, "generated scenarios usually need tighter review before becoming stable")
		score -= 6
	}
	if highestSeverity == "high" || highestSeverity == "critical" {
		reasons = append(reasons, "captures a high-severity regression candidate")
		score += 4
	}
	if diff.Status == "duplicate" || diff.Status == "unchanged" {
		reasons = append(reasons, "duplicates reduce the value of adding another stable case")
		score -= 4
	}
	return reasons, score
}

func reviewStableSuitability(score int) string {
	switch {
	case score >= 12:
		return "high"
	case score >= 6:
		return "medium"
	default:
		return "low"
	}
}

func highestSeverity(findings []core.Finding) (string, int) {
	best := ""
	bestScore := 0
	for _, finding := range findings {
		score := severityScore(finding.Severity)
		if score > bestScore {
			bestScore = score
			best = strings.ToLower(strings.TrimSpace(finding.Severity))
		}
	}
	return best, bestScore
}

func severityScore(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical":
		return 45
	case "high":
		return 35
	case "medium":
		return 20
	case "low":
		return 10
	case "info":
		return 4
	default:
		return 0
	}
}

func scenarioBodyKey(scenario core.Scenario) string {
	raw, _ := json.Marshal(map[string]string{
		"system": strings.TrimSpace(scenario.System),
		"input":  strings.TrimSpace(scenario.Input),
	})
	return string(raw)
}
