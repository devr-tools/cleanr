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
	currentByName := make(map[string]core.ScenarioFingerprint)
	previousByName := make(map[string]core.ScenarioFingerprint)
	if current != nil {
		for _, scenario := range current.ScenarioFingerprints {
			currentByName[scenario.Name] = scenario
		}
	}
	if previous != nil {
		for _, scenario := range previous.ScenarioFingerprints {
			previousByName[scenario.Name] = scenario
		}
	}

	names := make(map[string]struct{}, len(currentByName)+len(previousByName))
	for name := range currentByName {
		names[name] = struct{}{}
	}
	for name := range previousByName {
		names[name] = struct{}{}
	}

	out := make([]core.ScenarioDiff, 0)
	for name := range names {
		currentScenario, hasCurrent := currentByName[name]
		previousScenario, hasPrevious := previousByName[name]
		switch {
		case hasCurrent && !hasPrevious:
			out = append(out, core.ScenarioDiff{Name: name, Status: "new"})
		case !hasCurrent && hasPrevious:
			out = append(out, core.ScenarioDiff{Name: name, Status: "removed"})
		default:
			change := core.ScenarioDiff{
				Name:                name,
				Status:              "changed",
				SystemChanged:       previousScenario.SystemHash != currentScenario.SystemHash,
				InputChanged:        previousScenario.InputHash != currentScenario.InputHash,
				ContextChanged:      previousScenario.ContextHash != currentScenario.ContextHash,
				MemoryReplayChanged: previousScenario.MemoryReplayHash != currentScenario.MemoryReplayHash || previousScenario.MemoryReplaySteps != currentScenario.MemoryReplaySteps,
				TagsChanged:         previousScenario.TagsHash != currentScenario.TagsHash,
			}
			if change.SystemChanged || change.InputChanged || change.ContextChanged || change.MemoryReplayChanged || change.TagsChanged {
				out = append(out, change)
			}
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
