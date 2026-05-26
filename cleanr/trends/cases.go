package trends

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

var (
	quotedValuePattern = regexp.MustCompile(`"[^"]*"|'[^']*'`)
	numberPattern      = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	spacePattern       = regexp.MustCompile(`\s+`)
)

var workflowEvidenceSuites = map[string]struct{}{
	"claim-trace":    {},
	"memory-safety":  {},
	"provenance":     {},
	"release-policy": {},
	"shadow-state":   {},
}

type suiteCase struct {
	Suite string
	Case  HistoryCase
}

func shouldRetainCaseEvidence(suiteName string) bool {
	_, ok := workflowEvidenceSuites[suiteName]
	return ok
}

func buildCaseEvidence(suiteName string, c core.CaseResult) *HistoryCase {
	if !shouldRetainCaseEvidence(suiteName) {
		return nil
	}

	evidence := HistoryCase{
		Name:                  c.Name,
		Passed:                c.Passed,
		FindingSignatures:     findingSignatures(c.Findings),
		FirstUnsupportedClaim: detailString(c.Details, "first_unsupported_claim"),
		ToolCalls:             toolCallNames(c.Details["tool_calls"]),
		StateChanges:          stateChangeSummaries(c.Details["state_changes"]),
		FileChanges:           fileChangeSummaries(c.Details),
		MemoryMarkers:         memoryMarkers(c.Details),
	}
	if !shouldRetainHistoryCase(evidence) {
		return nil
	}
	return &evidence
}

func shouldRetainHistoryCase(evidence HistoryCase) bool {
	return !evidence.Passed || hasRetainedEvidence(evidence)
}

func hasRetainedEvidence(evidence HistoryCase) bool {
	return len(evidence.FindingSignatures) > 0 ||
		evidence.FirstUnsupportedClaim != "" ||
		len(evidence.ToolCalls) > 0 ||
		len(evidence.StateChanges) > 0 ||
		len(evidence.FileChanges) > 0 ||
		len(evidence.MemoryMarkers) > 0
}

func findingSignatures(findings []core.Finding) []string {
	signatures := make([]string, 0, len(findings))
	for _, finding := range findings {
		signature := normalizeFindingMessage(finding.Message)
		if signature != "" {
			signatures = append(signatures, signature)
		}
	}
	return uniqueStrings(signatures)
}

func normalizeFindingMessage(message string) string {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return ""
	}
	message = quotedValuePattern.ReplaceAllString(message, `"<value>"`)
	message = numberPattern.ReplaceAllString(message, "<n>")
	message = spacePattern.ReplaceAllString(message, " ")
	return strings.TrimSpace(message)
}

func detailString(details map[string]any, key string) string {
	if len(details) == 0 {
		return ""
	}
	value, ok := details[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func toolCallNames(value any) []string {
	switch typed := value.(type) {
	case []core.ToolCall:
		out := make([]string, 0, len(typed))
		for _, call := range typed {
			name := strings.TrimSpace(call.Name)
			if name != "" {
				out = append(out, name)
			}
		}
		return uniqueStrings(out)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if mapped, ok := item.(map[string]any); ok {
				if name, ok := mapped["name"].(string); ok {
					name = strings.TrimSpace(name)
					if name != "" {
						out = append(out, name)
					}
				}
			}
		}
		return uniqueStrings(out)
	default:
		return nil
	}
}

func stateChangeSummaries(value any) []string {
	switch typed := value.(type) {
	case []core.StateChange:
		out := make([]string, 0, len(typed))
		for _, change := range typed {
			summary := summarizeStateChange(change.Kind, change.Action, change.Target)
			if summary != "" {
				out = append(out, summary)
			}
		}
		return uniqueStrings(out)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			mapped, ok := item.(map[string]any)
			if !ok {
				continue
			}
			summary := summarizeStateChange(
				stringField(mapped["kind"]),
				stringField(mapped["action"]),
				stringField(mapped["target"]),
			)
			if summary != "" {
				out = append(out, summary)
			}
		}
		return uniqueStrings(out)
	default:
		return nil
	}
}

func summarizeStateChange(kind, action, target string) string {
	parts := make([]string, 0, 3)
	if value := strings.TrimSpace(kind); value != "" {
		parts = append(parts, value)
	}
	if value := strings.TrimSpace(action); value != "" {
		parts = append(parts, value)
	}
	if value := strings.TrimSpace(target); value != "" {
		parts = append(parts, value)
	}
	return strings.Join(parts, ":")
}

func fileChangeSummaries(details map[string]any) []string {
	keys := []string{
		"approved_changes",
		"changed_files",
		"missing_expected_mutations",
		"observed_file_changes",
		"unexpected_changes",
		"undeclared_approved_changes",
	}
	var out []string
	for _, key := range keys {
		out = append(out, stringSlice(details[key])...)
	}
	return uniqueStrings(out)
}

func memoryMarkers(details map[string]any) []string {
	markers := make([]string, 0)
	markers = appendPrefixedMarkers(markers, "attack_canary", stringSlice(details["attack_canaries"]))
	markers = appendPrefixedMarkers(markers, "cross_user", stringSlice(details["cross_user_memory_operations"]))
	markers = appendPrefixedMarkers(markers, "echoed_canary", stringSlice(details["echoed_canaries"]))
	markers = appendPrefixedMarkers(markers, "response_canary", stringSlice(details["unsafe_response_canaries"]))
	markers = appendPrefixedMarkers(markers, "tool_canary", stringSlice(details["tool_call_canaries"]))
	markers = appendPrefixedMarkers(markers, "tool_canary", stringSlice(details["unsafe_tool_call_canaries"]))
	return uniqueStrings(markers)
}

func appendPrefixedMarkers(dst []string, prefix string, items []string) []string {
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		dst = append(dst, fmt.Sprintf("%s:%s", prefix, item))
	}
	return dst
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return trimAndUniqueStrings(typed)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				text = strings.TrimSpace(text)
				if text != "" {
					out = append(out, text)
				}
			}
		}
		return uniqueStrings(out)
	default:
		return nil
	}
}

func stringField(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func flattenCases(run HistoryRun) []suiteCase {
	out := make([]suiteCase, 0)
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			out = append(out, suiteCase{Suite: suite.Name, Case: c})
		}
	}
	return out
}

func caseKey(suite, name string) string {
	return suite + "\x00" + name
}

func caseEvidenceChanged(previous, current HistoryCase) bool {
	return previous.FirstUnsupportedClaim != current.FirstUnsupportedClaim ||
		!equalStrings(previous.ToolCalls, current.ToolCalls) ||
		!equalStrings(previous.StateChanges, current.StateChanges) ||
		!equalStrings(previous.FileChanges, current.FileChanges) ||
		!equalStrings(previous.MemoryMarkers, current.MemoryMarkers) ||
		!equalStrings(previous.FindingSignatures, current.FindingSignatures)
}

func buildCaseTrend(suite, status string, current HistoryCase, previous *HistoryCase) core.CaseTrend {
	trend := core.CaseTrend{
		Suite:                 suite,
		Name:                  current.Name,
		Status:                status,
		Passed:                current.Passed,
		FindingSignatures:     append([]string(nil), current.FindingSignatures...),
		FirstUnsupportedClaim: current.FirstUnsupportedClaim,
		ToolCalls:             append([]string(nil), current.ToolCalls...),
		StateChanges:          append([]string(nil), current.StateChanges...),
		FileChanges:           append([]string(nil), current.FileChanges...),
		MemoryMarkers:         append([]string(nil), current.MemoryMarkers...),
	}
	if previous == nil {
		trend.NewFindingSignatures = append([]string(nil), current.FindingSignatures...)
		return trend
	}
	trend.NewFindingSignatures = diffStrings(current.FindingSignatures, previous.FindingSignatures)
	trend.ClearedFindingSignatures = diffStrings(previous.FindingSignatures, current.FindingSignatures)
	return trend
}

func compareCases(current HistoryRun, previous *HistoryRun) ([]core.CaseTrend, []core.CaseTrend) {
	if previous == nil {
		return nil, nil
	}

	previousByKey := make(map[string]HistoryCase)
	for _, item := range flattenCases(*previous) {
		previousByKey[caseKey(item.Suite, item.Case.Name)] = item.Case
	}

	regressions := make([]core.CaseTrend, 0)
	improvements := make([]core.CaseTrend, 0)
	for _, item := range flattenCases(current) {
		previousCase, ok := previousByKey[caseKey(item.Suite, item.Case.Name)]
		switch {
		case !ok && !item.Case.Passed:
			regressions = append(regressions, buildCaseTrend(item.Suite, "new", item.Case, nil))
		case ok && previousCase.Passed && !item.Case.Passed:
			regressions = append(regressions, buildCaseTrend(item.Suite, "regressed", item.Case, &previousCase))
		case ok && !previousCase.Passed && item.Case.Passed:
			improvements = append(improvements, buildCaseTrend(item.Suite, "improved", item.Case, &previousCase))
		case ok && !previousCase.Passed && !item.Case.Passed && caseEvidenceChanged(previousCase, item.Case):
			regressions = append(regressions, buildCaseTrend(item.Suite, "changed", item.Case, &previousCase))
		}
	}

	sortCaseTrends(regressions)
	sortCaseTrends(improvements)
	return regressions, improvements
}

func buildFailureBuckets(run HistoryRun) []core.FailureBucket {
	buckets := make(map[string]*core.FailureBucket)
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			if c.Passed {
				continue
			}
			signatures := append([]string(nil), c.FindingSignatures...)
			if len(signatures) == 0 && c.FirstUnsupportedClaim != "" {
				signatures = []string{"first unsupported claim: " + normalizeFindingMessage(c.FirstUnsupportedClaim)}
			}
			if len(signatures) == 0 {
				signatures = []string{"case failed"}
			}
			caseRef := suite.Name + "/" + c.Name
			for _, signature := range signatures {
				bucket := buckets[signature]
				if bucket == nil {
					bucket = &core.FailureBucket{Signature: signature}
					buckets[signature] = bucket
				}
				bucket.Count++
				bucket.Cases = append(bucket.Cases, caseRef)
			}
		}
	}

	if len(buckets) == 0 {
		return nil
	}
	out := make([]core.FailureBucket, 0, len(buckets))
	for _, bucket := range buckets {
		bucket.Cases = uniqueStrings(bucket.Cases)
		out = append(out, *bucket)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Signature < out[j].Signature
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func sortCaseTrends(items []core.CaseTrend) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Suite == items[j].Suite {
			return items[i].Name < items[j].Name
		}
		return items[i].Suite < items[j].Suite
	})
}

func diffStrings(left, right []string) []string {
	if len(left) == 0 {
		return nil
	}
	rightSet := make(map[string]struct{}, len(right))
	for _, item := range right {
		rightSet[item] = struct{}{}
	}
	out := make([]string, 0, len(left))
	for _, item := range left {
		if _, ok := rightSet[item]; ok {
			continue
		}
		out = append(out, item)
	}
	return uniqueStrings(out)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func trimAndUniqueStrings(values []string) []string {
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			trimmed = append(trimmed, value)
		}
	}
	return uniqueStrings(trimmed)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
