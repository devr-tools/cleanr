package engines

import (
	"sort"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func matchExpectedStateChanges(expected []core.ExpectedStateChange, observed []core.StateChange) ([]string, []string, []string) {
	if len(expected) == 0 {
		return nil, nil, nil
	}
	used := make([]bool, len(observed))
	matched := make([]string, 0, len(expected))
	missing := make([]string, 0)

	for _, want := range expected {
		matchIndex := -1
		for i, got := range observed {
			if used[i] {
				continue
			}
			if expectedStateChangeMatches(want, got) {
				matchIndex = i
				break
			}
		}
		if matchIndex == -1 {
			missing = append(missing, renderExpectedStateChange(want))
			continue
		}
		used[matchIndex] = true
		matched = append(matched, renderExpectedStateChange(want))
	}

	unexpected := make([]string, 0)
	for i, got := range observed {
		if used[i] {
			continue
		}
		unexpected = append(unexpected, renderObservedStateChange(got))
	}

	sort.Strings(matched)
	sort.Strings(missing)
	sort.Strings(unexpected)
	return matched, missing, unexpected
}

func expectedStateChangeMatches(expected core.ExpectedStateChange, observed core.StateChange) bool {
	if expected.Kind != "" && !strings.EqualFold(strings.TrimSpace(expected.Kind), strings.TrimSpace(observed.Kind)) {
		return false
	}
	if expected.Target != "" && !strings.EqualFold(strings.TrimSpace(expected.Target), strings.TrimSpace(observed.Target)) {
		return false
	}
	if expected.Action != "" && !strings.EqualFold(strings.TrimSpace(expected.Action), strings.TrimSpace(observed.Action)) {
		return false
	}
	if expected.Status != "" && !strings.EqualFold(strings.TrimSpace(expected.Status), strings.TrimSpace(observed.Status)) {
		return false
	}
	if expected.SummaryContains != "" && !strings.Contains(strings.ToLower(observed.Summary), strings.ToLower(expected.SummaryContains)) {
		return false
	}
	return true
}

func renderExpectedStateChanges(expected []core.ExpectedStateChange) []string {
	out := make([]string, 0, len(expected))
	for _, change := range expected {
		out = append(out, renderExpectedStateChange(change))
	}
	sort.Strings(out)
	return out
}

func renderExpectedStateChange(change core.ExpectedStateChange) string {
	parts := make([]string, 0, 5)
	if strings.TrimSpace(change.Kind) != "" {
		parts = append(parts, "kind="+change.Kind)
	}
	if strings.TrimSpace(change.Target) != "" {
		parts = append(parts, "target="+change.Target)
	}
	if strings.TrimSpace(change.Action) != "" {
		parts = append(parts, "action="+change.Action)
	}
	if strings.TrimSpace(change.Status) != "" {
		parts = append(parts, "status="+change.Status)
	}
	if strings.TrimSpace(change.SummaryContains) != "" {
		parts = append(parts, "summary_contains="+change.SummaryContains)
	}
	return strings.Join(parts, " ")
}

func renderObservedStateChange(change core.StateChange) string {
	parts := make([]string, 0, 4)
	if strings.TrimSpace(change.Kind) != "" {
		parts = append(parts, "kind="+change.Kind)
	}
	if strings.TrimSpace(change.Target) != "" {
		parts = append(parts, "target="+change.Target)
	}
	if strings.TrimSpace(change.Action) != "" {
		parts = append(parts, "action="+change.Action)
	}
	if strings.TrimSpace(change.Status) != "" {
		parts = append(parts, "status="+change.Status)
	}
	if strings.TrimSpace(change.Summary) != "" {
		parts = append(parts, "summary="+trimForReport(change.Summary))
	}
	return strings.Join(parts, " ")
}

func stateChangeIdentity(change core.StateChange) string {
	if strings.TrimSpace(change.Action) != "" {
		return strings.TrimSpace(change.Action)
	}
	if strings.TrimSpace(change.Kind) != "" {
		return strings.TrimSpace(change.Kind)
	}
	return "state_change"
}
