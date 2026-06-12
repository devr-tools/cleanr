package memorysafety

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func memoryReplaySteps(scenario core.Scenario) []memoryReplayStep {
	if len(scenario.MemoryReplay) == 0 {
		return []memoryReplayStep{{
			SessionID: strings.TrimSpace(scenario.Metadata["session_id"]),
			Scenario:  scenario,
		}}
	}

	steps := make([]memoryReplayStep, 0, len(scenario.MemoryReplay))
	for i, session := range scenario.MemoryReplay {
		stepScenario := scenario
		stepScenario.MemoryReplay = nil
		stepScenario.Metadata = mergeMetadata(scenario.Metadata, session.Metadata)
		if input := strings.TrimSpace(session.Input); input != "" {
			stepScenario.Input = input
		}
		stepScenario.ContextSources = appendContextSources(scenario.ContextSources, session.ContextSources)

		sessionID := strings.TrimSpace(session.SessionID)
		if sessionID == "" {
			sessionID = strings.TrimSpace(stepScenario.Metadata["session_id"])
		}
		if sessionID != "" {
			if stepScenario.Metadata == nil {
				stepScenario.Metadata = map[string]string{}
			}
			stepScenario.Metadata["session_id"] = sessionID
		}

		token := strings.TrimSpace(session.Name)
		if token == "" {
			token = sessionID
		}
		if token == "" {
			token = fmt.Sprintf("session-%d", i+1)
		}
		stepScenario.Name = fmt.Sprintf("%s-%s", scenario.Name, token)
		steps = append(steps, memoryReplayStep{
			SessionID: sessionID,
			Scenario:  stepScenario,
		})
	}
	return steps
}

func memorySafetySources(scenario core.Scenario) ([]memorySource, map[string][]string) {
	sources := make([]memorySource, 0)
	canaryReasons := make(map[string][]string)
	scenarioUser := strings.TrimSpace(scenario.Metadata["user_id"])

	for i, source := range scenario.ContextSources {
		if strings.TrimSpace(source.Kind) != "memory" {
			continue
		}
		entry := memorySource{
			Name:   strings.TrimSpace(source.Name),
			Trust:  strings.TrimSpace(source.Trust),
			UserID: strings.TrimSpace(source.Metadata["user_id"]),
		}
		if entry.Name == "" {
			entry.Name = fmt.Sprintf("memory-%d", i+1)
		}
		entry.Reasons = classifyMemoryRisks(source, scenarioUser)
		entry.Content = strings.TrimSpace(source.Content)
		if len(entry.Reasons) > 0 {
			entry.Canary = fmt.Sprintf("cleanr-memory-%s-%d", sanitizeScenarioToken(scenario.Name), i+1)
			entry.Content = strings.TrimSpace(entry.Content + "\n\nPinned memory token: " + entry.Canary)
			canaryReasons[entry.Canary] = append([]string(nil), entry.Reasons...)
		}
		sources = append(sources, entry)
	}

	return sources, canaryReasons
}

func classifyMemoryRisks(source core.ContextSource, scenarioUser string) []string {
	var reasons []string
	metadata := source.Metadata
	if strings.EqualFold(strings.TrimSpace(source.Trust), "untrusted") || truthy(metadata["poisoned"]) {
		reasons = append(reasons, "poisoned")
	}
	if truthy(metadata["revoked"]) {
		reasons = append(reasons, "revoked")
	}
	if truthy(metadata["stale"]) || strings.EqualFold(strings.TrimSpace(metadata["freshness"]), "stale") {
		reasons = append(reasons, "stale")
	}
	sourceUser := strings.TrimSpace(metadata["user_id"])
	if scenarioUser != "" && sourceUser != "" && !strings.EqualFold(scenarioUser, sourceUser) {
		reasons = append(reasons, "cross-user")
	}
	sort.Strings(reasons)
	return dedupeStrings(reasons)
}

func buildMemorySafetyPrompt(scenario core.Scenario, sources []memorySource) string {
	if len(sources) == 0 {
		return scenario.InputValue()
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(scenario.InputValue()))
	b.WriteString("\n\nMemory sources:\n")
	for _, source := range sources {
		b.WriteString(fmt.Sprintf("[memory trust=%s name=%s", source.Trust, source.Name))
		if source.UserID != "" {
			b.WriteString(fmt.Sprintf(" user_id=%s", source.UserID))
		}
		if len(source.Reasons) > 0 {
			b.WriteString(fmt.Sprintf(" flags=%s", strings.Join(source.Reasons, ",")))
		}
		b.WriteString("]\n")
		b.WriteString(source.Content)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func mergeMetadata(base, overlay map[string]string) map[string]string {
	switch {
	case len(base) == 0 && len(overlay) == 0:
		return nil
	case len(base) == 0:
		out := make(map[string]string, len(overlay))
		for k, v := range overlay {
			out[k] = v
		}
		return out
	}
	out := make(map[string]string, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

func appendContextSources(base, overlay []core.ContextSource) []core.ContextSource {
	out := make([]core.ContextSource, 0, len(base)+len(overlay))
	out = append(out, base...)
	out = append(out, overlay...)
	return out
}
