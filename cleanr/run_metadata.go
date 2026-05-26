package cleanr

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func buildRunMetadata(cfg Config) *core.RunMetadata {
	metadata := &core.RunMetadata{
		BuildID:       strings.TrimSpace(cfg.Reporting.BuildID),
		TargetType:    cfg.Target.TargetType(),
		ProviderModel: configuredModel(cfg.Target),
	}
	if len(cfg.Scenarios) > 0 {
		metadata.ScenarioFingerprints = make([]core.ScenarioFingerprint, 0, len(cfg.Scenarios))
		for _, scenario := range cfg.Scenarios {
			metadata.ScenarioFingerprints = append(metadata.ScenarioFingerprints, buildScenarioFingerprint(scenario))
		}
	}
	if metadata.BuildID == "" && metadata.TargetType == "" && metadata.ProviderModel == "" && len(metadata.ScenarioFingerprints) == 0 {
		return nil
	}
	return metadata
}

func configuredModel(cfg TargetConfig) string {
	switch cfg.TargetType() {
	case "openai":
		return strings.TrimSpace(cfg.OpenAI.Model)
	case "anthropic":
		return strings.TrimSpace(cfg.Anthropic.Model)
	default:
		return ""
	}
}

func buildScenarioFingerprint(scenario Scenario) core.ScenarioFingerprint {
	tags := append([]string(nil), scenario.Tags...)
	sort.Strings(tags)
	return core.ScenarioFingerprint{
		Name:              scenario.Name,
		SystemHash:        stableHash(strings.TrimSpace(scenario.System)),
		InputHash:         stableHash(strings.TrimSpace(scenario.Input)),
		ContextHash:       stableObjectHash(scenario.ContextSources),
		MemoryReplayHash:  stableObjectHash(scenario.MemoryReplay),
		MemoryReplaySteps: len(scenario.MemoryReplay),
		TagsHash:          stableObjectHash(tags),
		Tags:              tags,
	}
}

func stableObjectHash(value any) string {
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 || string(data) == "null" || string(data) == "[]" {
		return ""
	}
	return stableHash(string(data))
}

func stableHash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}
