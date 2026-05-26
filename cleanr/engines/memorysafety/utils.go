package memorysafety

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func responseFindings(resp core.Response, forbidden []string) []core.Finding {
	findings := make([]core.Finding, 0)
	if resp.Err != nil {
		return append(findings, core.Finding{Severity: "critical", Message: resp.Err.Error()})
	}
	if resp.ExtractError != nil && len(resp.Normalized.ToolCalls) == 0 {
		findings = append(findings, core.Finding{Severity: "high", Message: "failed to extract configured response field"})
	}
	if resp.StatusCode >= 500 {
		findings = append(findings, core.Finding{Severity: "critical", Message: fmt.Sprintf("upstream returned %d", resp.StatusCode)})
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("client-visible error status %d", resp.StatusCode)})
	}
	for _, item := range forbidden {
		if strings.Contains(strings.ToLower(resp.Text), strings.ToLower(item)) {
			findings = append(findings, core.Finding{Severity: "critical", Message: fmt.Sprintf("forbidden content detected: %s", item)})
		}
	}
	return findings
}

func responseDetails(resp core.Response, base map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	normalized := resp.Normalized
	if normalized.Provider != "" {
		base["provider"] = normalized.Provider
	}
	if normalized.ID != "" {
		base["provider_id"] = normalized.ID
	}
	if normalized.Model != "" {
		base["provider_model"] = normalized.Model
	}
	if normalized.Role != "" {
		base["provider_role"] = normalized.Role
	}
	if normalized.Status != "" {
		base["provider_status"] = normalized.Status
	}
	if normalized.FinishReason != "" {
		base["finish_reason"] = normalized.FinishReason
	}
	if normalized.StopSequence != "" {
		base["stop_sequence"] = normalized.StopSequence
	}
	if len(normalized.ToolCalls) > 0 {
		base["tool_call_count"] = len(normalized.ToolCalls)
		base["tool_calls"] = normalized.ToolCalls
	}
	if len(normalized.SourceUses) > 0 {
		base["source_use_count"] = len(normalized.SourceUses)
		base["source_uses"] = normalized.SourceUses
	}
	if len(normalized.Approvals) > 0 {
		base["approval_count"] = len(normalized.Approvals)
		base["approvals"] = normalized.Approvals
	}
	if len(normalized.StateChanges) > 0 {
		base["state_change_count"] = len(normalized.StateChanges)
		base["state_changes"] = normalized.StateChanges
	}
	if len(normalized.MemoryOperations) > 0 {
		base["memory_operation_count"] = len(normalized.MemoryOperations)
		base["memory_operations"] = normalized.MemoryOperations
	}
	if len(normalized.Raw) > 0 {
		base["provider_raw"] = normalized.Raw
	}
	return base
}

func sortedCanaryReasons(canaryReasons map[string][]string) []string {
	if len(canaryReasons) == 0 {
		return nil
	}
	out := make([]string, 0, len(canaryReasons))
	for canary, reasons := range canaryReasons {
		out = append(out, fmt.Sprintf("%s:%s", canary, strings.Join(reasons, ",")))
	}
	sort.Strings(out)
	return out
}

func renderMemorySources(sources []memorySource) []string {
	out := make([]string, 0, len(sources))
	for _, source := range sources {
		summary := fmt.Sprintf("%s trust=%s", source.Name, source.Trust)
		if source.UserID != "" {
			summary += " user_id=" + source.UserID
		}
		if len(source.Reasons) > 0 {
			summary += " flags=" + strings.Join(source.Reasons, ",")
		}
		out = append(out, summary)
	}
	sort.Strings(out)
	return out
}

func renderMemorySourcesForSession(sessionID string, sources []memorySource) []string {
	if len(sources) == 0 {
		return nil
	}
	out := make([]string, 0, len(sources))
	for _, source := range renderMemorySources(sources) {
		out = append(out, prefixItem(sessionID, source))
	}
	sort.Strings(out)
	return out
}

func renderMemorySessionSummary(sessionID string, sources []memorySource, ops []core.MemoryOperation, writes []memoryHazardWrite) string {
	return fmt.Sprintf("%s sources=%d ops=%d traced_writes=%d", displaySessionID(sessionID), len(sources), len(ops), len(writes))
}

func tracedSessionID(defaultSessionID, opSessionID string) string {
	if sessionID := strings.TrimSpace(opSessionID); sessionID != "" {
		return sessionID
	}
	return strings.TrimSpace(defaultSessionID)
}

func isMemoryWriteAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "write", "set", "store", "save", "create", "update", "upsert":
		return true
	default:
		return false
	}
}

func isMemoryReadAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "read", "get", "load", "fetch", "lookup":
		return true
	default:
		return false
	}
}

func formatSessionSuffix(sessionID string) string {
	if strings.TrimSpace(sessionID) == "" {
		return ""
	}
	return " in session " + strings.TrimSpace(sessionID)
}

func displaySessionID(sessionID string) string {
	if strings.TrimSpace(sessionID) == "" {
		return "session=untraced"
	}
	return "session=" + strings.TrimSpace(sessionID)
}

func prefixItem(sessionID, item string) string {
	if strings.TrimSpace(sessionID) == "" {
		return item
	}
	return strings.TrimSpace(sessionID) + ":" + item
}

func prefixItems(sessionID string, items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, prefixItem(sessionID, item))
	}
	return out
}

func sortAndDedupe(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	items = dedupeStrings(items)
	sort.Strings(items)
	return items
}

func sortedKeys(items map[string]struct{}) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for item := range items {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func sanitizeScenarioToken(v string) string {
	lower := strings.ToLower(strings.TrimSpace(v))
	if lower == "" {
		return "scenario"
	}
	replacer := strings.NewReplacer(" ", "-", "_", "-", "/", "-", ":", "-", ".", "-")
	return replacer.Replace(lower)
}

func allPassed(cases []core.CaseResult) bool {
	for _, c := range cases {
		if !c.Passed {
			return false
		}
	}
	return true
}

func tracedSessionCount(writes []memoryHazardWrite) int {
	if len(writes) == 0 {
		return 0
	}
	seen := make(map[string]struct{})
	for _, write := range writes {
		if write.SessionID == "" {
			continue
		}
		seen[write.SessionID] = struct{}{}
	}
	return len(seen)
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "t", "true", "yes", "y":
		return true
	default:
		return false
	}
}

func dedupeStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
