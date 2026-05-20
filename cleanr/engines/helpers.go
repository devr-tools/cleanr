package engines

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"cleanr/cleanr/core"
)

func responseFindings(resp core.Response, forbidden []string) []core.Finding {
	findings := make([]core.Finding, 0)
	if resp.Err != nil {
		return append(findings, core.Finding{Severity: "critical", Message: resp.Err.Error()})
	}
	if resp.ExtractError != nil {
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

func filterStableScenarios(scenarios []core.Scenario, tags []string) []core.Scenario {
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
