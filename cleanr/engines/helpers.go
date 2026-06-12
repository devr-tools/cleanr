package engines

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func scenarioRequest(scenario core.Scenario, timeout time.Duration) core.Request {
	return core.BuildScenarioRequest(scenario, timeout)
}

func scenarioPromptText(scenario core.Scenario) string {
	return strings.TrimSpace(scenario.SystemValue() + "\n" + scenario.InputValue())
}

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
	applyProviderDetails(base, normalized)
	applyProcessDetails(base, resp)
	applyStreamDetails(base, resp.Stream)
	applyEvidenceDetails(base, normalized)
	return base
}

func applyProviderDetails(base map[string]any, normalized core.ProviderResponse) {
	maybeSetDetail(base, "provider", normalized.Provider)
	maybeSetDetail(base, "provider_id", normalized.ID)
	maybeSetDetail(base, "provider_model", normalized.Model)
	maybeSetDetail(base, "provider_role", normalized.Role)
	maybeSetDetail(base, "provider_status", normalized.Status)
	maybeSetDetail(base, "finish_reason", normalized.FinishReason)
	maybeSetDetail(base, "stop_sequence", normalized.StopSequence)
}

func applyProcessDetails(base map[string]any, resp core.Response) {
	if resp.ExitCode != 0 {
		base["exit_code"] = resp.ExitCode
	}
	if strings.TrimSpace(resp.Stderr) != "" {
		base["stderr"] = trimForReport(resp.Stderr)
	}
}

func applyStreamDetails(base map[string]any, stream core.StreamMetrics) {
	if stream.TTFTMS > 0 || stream.DurationMS > 0 || stream.ChunkCount > 0 || stream.ErrorCount > 0 || stream.Recovered {
		base["stream"] = stream
	}
}

func applyEvidenceDetails(base map[string]any, normalized core.ProviderResponse) {
	maybeSetCountedDetail(base, "tool_call", normalized.ToolCalls)
	maybeSetCountedDetail(base, "source_use", normalized.SourceUses)
	maybeSetCountedDetail(base, "approval", normalized.Approvals)
	maybeSetCountedDetail(base, "state_change", normalized.StateChanges)
	maybeSetCountedDetail(base, "memory_operation", normalized.MemoryOperations)
	if len(normalized.Raw) > 0 {
		base["provider_raw"] = normalized.Raw
	}
}

func maybeSetDetail(base map[string]any, key, value string) {
	if strings.TrimSpace(value) != "" {
		base[key] = value
	}
}

func maybeSetCountedDetail[T any](base map[string]any, prefix string, items []T) {
	if len(items) == 0 {
		return
	}
	base[prefix+"_count"] = len(items)
	base[prefix+"s"] = items
}

func filterStableScenarios(scenarios []core.Scenario, tags []string) []core.Scenario {
	return filterScenariosByTags(scenarios, tags)
}

func filterScenariosByTags(scenarios []core.Scenario, tags []string) []core.Scenario {
	if len(tags) == 0 {
		return nil
	}
	var out []core.Scenario
	for _, scenario := range scenarios {
		for _, want := range tags {
			for _, tag := range scenario.Tags {
				if tag == want {
					out = append(out, scenario)
					goto nextScenario
				}
			}
		}
	nextScenario:
	}
	return out
}

func measureDrift(samples []string) (float64, float64) {
	if len(samples) < 2 {
		return 0, 1
	}
	total := 0.0
	count := 0.0
	for i := 0; i < len(samples); i++ {
		for j := i + 1; j < len(samples); j++ {
			total += normalizedDistance(samples[i], samples[j])
			count++
		}
	}
	drift := total / count
	return drift, 1 - drift
}

func normalizedDistance(a, b string) float64 {
	if a == b {
		return 0
	}
	ra := []rune(a)
	rb := []rune(b)
	if len(ra) == 0 && len(rb) == 0 {
		return 0
	}
	dist := levenshtein(ra, rb)
	denom := float64(max(len(ra), len(rb)))
	if denom == 0 {
		return 0
	}
	return float64(dist) / denom
}

func levenshtein(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		copy(prev, cur)
	}
	return prev[len(b)]
}

func trimForReport(v string) string {
	if len(v) <= 240 {
		return v
	}
	return v[:240] + "..."
}

func containsAny(s string, items []string) bool {
	for _, item := range items {
		if strings.Contains(s, strings.ToLower(item)) {
			return true
		}
	}
	return false
}

func allPassed(cases []core.CaseResult) bool {
	for _, c := range cases {
		if !c.Passed {
			return false
		}
	}
	return true
}

func percentile(samples []time.Duration, p int) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	cp := append([]time.Duration(nil), samples...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	idx := int(math.Ceil(float64(p)/100*float64(len(cp)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

func min3(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
