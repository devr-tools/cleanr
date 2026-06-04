package integrations

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func buildReviewDiff(candidate core.Scenario, existingByName map[string]core.Scenario, existingByBody map[string][]core.Scenario) DatasetReviewDiff {
	if existing, ok := existingByName[candidate.Name]; ok {
		diff := compareScenario(candidate, existing)
		if reviewDiffUnchanged(diff) {
			diff.Status = "unchanged"
			diff.Summary = []string{"matches existing scenario"}
			return diff
		}
		diff.Status = "modified"
		diff.Summary = appendDiffSummary(diff)
		return diff
	}

	key := scenarioBodyKey(candidate)
	if duplicates := existingByBody[key]; len(duplicates) > 0 {
		return DatasetReviewDiff{
			Status:      "duplicate",
			DuplicateOf: duplicates[0].Name,
			ComparedTo:  duplicates[0].Name,
			Summary:     []string{"duplicates an existing scenario body"},
		}
	}
	return DatasetReviewDiff{
		Status:  "new",
		Summary: []string{"new scenario candidate"},
	}
}

func compareScenario(candidate, existing core.Scenario) DatasetReviewDiff {
	return DatasetReviewDiff{
		ComparedTo:          existing.Name,
		SystemChanged:       strings.TrimSpace(existing.System) != strings.TrimSpace(candidate.System),
		InputChanged:        strings.TrimSpace(existing.Input) != strings.TrimSpace(candidate.Input),
		MetadataChanged:     !equalStringMap(existing.Metadata, candidate.Metadata),
		TagsChanged:         !equalStringSlice(existing.Tags, candidate.Tags),
		ExpectedChanged:     !equalStringSlice(existing.ExpectedContains, candidate.ExpectedContains),
		ForbiddenChanged:    !equalStringSlice(existing.ForbiddenContains, candidate.ForbiddenContains),
		AssertionsChanged:   !equalAssertions(existing.Assertions, candidate.Assertions),
		ContextChanged:      !equalContextSources(existing.ContextSources, candidate.ContextSources),
		MemoryReplayChanged: !equalMemoryReplay(existing.MemoryReplay, candidate.MemoryReplay),
	}
}

func reviewDiffUnchanged(diff DatasetReviewDiff) bool {
	return !diff.SystemChanged &&
		!diff.InputChanged &&
		!diff.MetadataChanged &&
		!diff.TagsChanged &&
		!diff.ExpectedChanged &&
		!diff.ForbiddenChanged &&
		!diff.AssertionsChanged &&
		!diff.ContextChanged &&
		!diff.MemoryReplayChanged
}

func appendDiffSummary(diff DatasetReviewDiff) []string {
	var out []string
	if diff.SystemChanged {
		out = append(out, "system")
	}
	if diff.InputChanged {
		out = append(out, "input")
	}
	if diff.MetadataChanged {
		out = append(out, "metadata")
	}
	if diff.TagsChanged {
		out = append(out, "tags")
	}
	if diff.ExpectedChanged {
		out = append(out, "expected_contains")
	}
	if diff.ForbiddenChanged {
		out = append(out, "forbidden_contains")
	}
	if diff.AssertionsChanged {
		out = append(out, "assertions")
	}
	if diff.ContextChanged {
		out = append(out, "context_sources")
	}
	if diff.MemoryReplayChanged {
		out = append(out, "memory_replay")
	}
	return out
}

func validateReviewOptions(dataset ScenarioDataset, opts DatasetReviewOptions) error {
	names := datasetScenarioNames(dataset)
	for _, name := range append(append(append(append([]string{}, opts.Approve...), opts.Reject...), opts.PromoteStable...), opts.PromoteRegression...) {
		if _, ok := names[name]; !ok {
			return fmt.Errorf("review error: unknown scenario %q", name)
		}
	}
	for _, group := range []map[string][]string{opts.AddTags, opts.SetTags} {
		for name := range group {
			if _, ok := names[name]; !ok {
				return fmt.Errorf("review error: unknown scenario %q", name)
			}
		}
	}
	for name := range opts.SetMetadata {
		if _, ok := names[name]; !ok {
			return fmt.Errorf("review error: unknown scenario %q", name)
		}
	}
	rejected := make(map[string]struct{}, len(opts.Reject))
	for _, name := range opts.Reject {
		rejected[name] = struct{}{}
	}
	for _, name := range opts.Approve {
		if _, ok := rejected[name]; ok {
			return fmt.Errorf("review error: scenario %q cannot be both approved and rejected", name)
		}
	}
	return nil
}

func datasetScenarioNames(dataset ScenarioDataset) map[string]struct{} {
	names := make(map[string]struct{}, len(dataset.Scenarios))
	for _, item := range dataset.Scenarios {
		names[item.Scenario.Name] = struct{}{}
	}
	return names
}

func withReviewProvenance(metadata map[string]string, reviewed ReviewedScenarioDataset, entry ReviewedScenarioEntry) map[string]string {
	out := cloneStringMap(metadata)
	if out == nil {
		out = map[string]string{}
	}
	if reviewed.InputSource != "" {
		out["cleanr.review.source"] = reviewed.InputSource
	}
	if reviewed.BuildID != "" {
		out["cleanr.review.build_id"] = reviewed.BuildID
	}
	if reviewed.PolicyPath != "" {
		out["cleanr.review.policy_path"] = reviewed.PolicyPath
	}
	if reviewed.PolicyVersion != "" {
		out["cleanr.review.policy_version"] = reviewed.PolicyVersion
	}
	if entry.Entry.Origin.Suite != "" {
		out["cleanr.review.origin_suite"] = entry.Entry.Origin.Suite
	}
	if entry.Entry.Origin.Case != "" {
		out["cleanr.review.origin_case"] = entry.Entry.Origin.Case
	}
	if entry.Entry.Origin.BuildID != "" {
		out["cleanr.review.origin_build_id"] = entry.Entry.Origin.BuildID
	}
	if reviewed.Generator != nil {
		if reviewed.Generator.Provider != "" {
			out["cleanr.review.generator_provider"] = reviewed.Generator.Provider
		}
		if reviewed.Generator.Model != "" {
			out["cleanr.review.generator_model"] = reviewed.Generator.Model
		}
	}
	if len(entry.Decision.PolicyRules) > 0 {
		out["cleanr.review.policy_rules"] = strings.Join(entry.Decision.PolicyRules, ",")
	}
	return out
}

func equalStringSlice(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]string(nil), left...)
	rightCopy := append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	for i := range leftCopy {
		if leftCopy[i] != rightCopy[i] {
			return false
		}
	}
	return true
}

func equalStringMap(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func equalAssertions(left, right []core.Assertion) bool {
	rawLeft, _ := json.Marshal(left)
	rawRight, _ := json.Marshal(right)
	return string(rawLeft) == string(rawRight)
}

func equalContextSources(left, right []core.ContextSource) bool {
	rawLeft, _ := json.Marshal(left)
	rawRight, _ := json.Marshal(right)
	return string(rawLeft) == string(rawRight)
}

func equalMemoryReplay(left, right []core.MemoryReplaySession) bool {
	rawLeft, _ := json.Marshal(left)
	rawRight, _ := json.Marshal(right)
	return string(rawLeft) == string(rawRight)
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
