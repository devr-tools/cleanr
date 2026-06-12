package trends

import (
	"sort"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func compareBuildMetadata(current, previous *core.RunMetadata) *core.BuildDiff {
	if current == nil && previous == nil {
		return nil
	}

	diff := &core.BuildDiff{}
	if previous != nil && current != nil && previous.TargetType != current.TargetType {
		diff.TargetTypeBefore = previous.TargetType
		diff.TargetTypeAfter = current.TargetType
	}
	if previous != nil && current != nil && previous.ProviderModel != current.ProviderModel {
		diff.ModelBefore = previous.ProviderModel
		diff.ModelAfter = current.ProviderModel
	}
	diff.ScenarioChanges = compareScenarioFingerprints(current, previous)

	if diff.TargetTypeBefore == "" &&
		diff.TargetTypeAfter == "" &&
		diff.ModelBefore == "" &&
		diff.ModelAfter == "" &&
		len(diff.ScenarioChanges) == 0 {
		return nil
	}
	return diff
}

func compareScenarioFingerprints(current, previous *core.RunMetadata) []core.ScenarioDiff {
	currentByName := scenarioFingerprintsByName(current)
	previousByName := scenarioFingerprintsByName(previous)
	names := make(map[string]struct{}, len(currentByName)+len(previousByName))
	for name := range currentByName {
		names[name] = struct{}{}
	}
	for name := range previousByName {
		names[name] = struct{}{}
	}

	out := make([]core.ScenarioDiff, 0)
	for name := range names {
		if change, ok := diffScenarioFingerprint(name, currentByName[name], previousByName[name]); ok {
			out = append(out, change)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Status == out[j].Status {
			return out[i].Name < out[j].Name
		}
		return out[i].Status < out[j].Status
	})
	if len(out) == 0 {
		return nil
	}
	return out
}

func scenarioFingerprintsByName(metadata *core.RunMetadata) map[string]core.ScenarioFingerprint {
	byName := make(map[string]core.ScenarioFingerprint)
	if metadata == nil {
		return byName
	}
	for _, scenario := range metadata.ScenarioFingerprints {
		byName[scenario.Name] = scenario
	}
	return byName
}

func diffScenarioFingerprint(name string, currentScenario, previousScenario core.ScenarioFingerprint) (core.ScenarioDiff, bool) {
	switch {
	case currentScenario.Name != "" && previousScenario.Name == "":
		return core.ScenarioDiff{Name: name, Status: "new"}, true
	case currentScenario.Name == "" && previousScenario.Name != "":
		return core.ScenarioDiff{Name: name, Status: "removed"}, true
	}
	change := core.ScenarioDiff{
		Name:                name,
		Status:              "changed",
		SystemChanged:       previousScenario.SystemHash != currentScenario.SystemHash,
		InputChanged:        previousScenario.InputHash != currentScenario.InputHash,
		TurnsChanged:        previousScenario.TurnsHash != currentScenario.TurnsHash || previousScenario.TurnCount != currentScenario.TurnCount,
		ContextChanged:      previousScenario.ContextHash != currentScenario.ContextHash,
		MemoryReplayChanged: previousScenario.MemoryReplayHash != currentScenario.MemoryReplayHash || previousScenario.MemoryReplaySteps != currentScenario.MemoryReplaySteps,
		TagsChanged:         previousScenario.TagsHash != currentScenario.TagsHash,
	}
	return change, scenarioDiffChanged(change)
}

func scenarioDiffChanged(change core.ScenarioDiff) bool {
	return change.SystemChanged ||
		change.InputChanged ||
		change.TurnsChanged ||
		change.ContextChanged ||
		change.MemoryReplayChanged ||
		change.TagsChanged
}
