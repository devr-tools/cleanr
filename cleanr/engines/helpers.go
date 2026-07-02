package engines

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func scenarioRequest(scenario core.Scenario, timeout time.Duration) core.Request {
	return core.BuildScenarioRequest(scenario, timeout)
}

// runBoundedByIndex invokes fn(i) for i in [0,count) using a bounded worker
// pool of at most limit goroutines. fn must write only to its own index i;
// callers pre-size result slices so output stays deterministically ordered
// regardless of completion order. Iteration stops early if ctx is cancelled.
// It returns the number of indices actually invoked — always a prefix of
// [0,count) — so callers can truncate pre-sized result slices instead of
// reporting never-run entries as zero-value failures.
func runBoundedByIndex(ctx context.Context, count, limit int, fn func(i int)) int {
	if limit < 1 {
		limit = 1
	}
	if limit == 1 || count <= 1 {
		for i := 0; i < count; i++ {
			if ctx.Err() != nil {
				return i
			}
			fn(i)
		}
		return count
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	launched := 0
	for i := 0; i < count; i++ {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		launched++
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(i)
		}(i)
	}
	wg.Wait()
	return launched
}

// responseCache memoizes target responses for identical plain scenario requests
// within a single run so read-only engines that replay the same unmodified
// request share one live Invoke instead of each issuing their own.
type responseCache struct {
	mu sync.Mutex
	m  map[string]core.Response
}

func newResponseCache() *responseCache {
	return &responseCache{m: make(map[string]core.Response)}
}

func responseCacheKey(req core.Request) string {
	var b strings.Builder
	b.WriteString(req.Scenario.Name)
	b.WriteByte(0)
	b.WriteString(req.System)
	b.WriteByte(0)
	b.WriteString(req.Prompt)
	b.WriteByte(0)
	b.WriteString(req.Scenario.TranscriptText())
	return b.String()
}

// invoke returns a cached response for req when available, otherwise invokes the
// target and stores the result. A nil cache invokes the target directly. Only
// plain (unmutated) scenario requests should flow through here; mutating engines
// must invoke the target directly to preserve their distinct inputs.
func (c *responseCache) invoke(ctx context.Context, target core.Target, req core.Request) core.Response {
	if c == nil {
		return target.Invoke(ctx, req)
	}
	key := responseCacheKey(req)
	c.mu.Lock()
	if resp, ok := c.m[key]; ok {
		c.mu.Unlock()
		return resp
	}
	c.mu.Unlock()

	resp := target.Invoke(ctx, req)
	c.mu.Lock()
	c.m[key] = resp
	c.mu.Unlock()
	return resp
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
	applyUsageDetails(base, resp.Usage)
	applyStreamDetails(base, resp.Stream)
	applyEvidenceDetails(base, normalized, resp.Usage)
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

func applyUsageDetails(base map[string]any, usage core.TokenUsage) {
	if usage.TotalTokens == 0 && usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return
	}
	base["usage"] = usage
}

func applyEvidenceDetails(base map[string]any, normalized core.ProviderResponse, usage core.TokenUsage) {
	maybeSetCountedDetail(base, "tool_call", normalized.ToolCalls)
	maybeSetCountedDetail(base, "source_use", normalized.SourceUses)
	maybeSetCountedDetail(base, "approval", normalized.Approvals)
	maybeSetCountedDetail(base, "state_change", normalized.StateChanges)
	maybeSetCountedDetail(base, "memory_operation", normalized.MemoryOperations)
	if summary := trajectorySummary(normalized, usage); summary["steps"].(int) > 0 || summary["token_cost_signal"].(int) > 0 {
		base["trajectory"] = summary
	}
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

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func trajectorySummary(normalized core.ProviderResponse, usage core.TokenUsage) map[string]any {
	steps := len(normalized.ToolCalls) + len(normalized.Approvals) + len(normalized.StateChanges) + len(normalized.MemoryOperations)
	deadEnds := countDeadEnds(normalized)
	tokenCost := usage.TotalTokens
	if tokenCost == 0 {
		tokenCost = usage.InputTokens + usage.OutputTokens
	}
	coverageBonus := math.Min(0.4, float64(steps)*0.1)
	deadEndPenalty := 0.0
	if steps > 0 {
		deadEndPenalty = math.Min(0.6, float64(deadEnds)/float64(steps)*0.6)
	}
	costPenalty := math.Min(0.3, float64(tokenCost)/5000*0.3)
	score := clamp01(0.5 + coverageBonus - deadEndPenalty - costPenalty)
	return map[string]any{
		"steps":             steps,
		"dead_ends":         deadEnds,
		"token_cost_signal": tokenCost,
		"score":             round3(score),
	}
}

func countDeadEnds(normalized core.ProviderResponse) int {
	count := 0
	for _, tool := range normalized.ToolCalls {
		if evidenceDeadEnd(tool.Status) {
			count++
		}
	}
	for _, approval := range normalized.Approvals {
		if evidenceDeadEnd(approval.Status) {
			count++
		}
	}
	for _, change := range normalized.StateChanges {
		if evidenceDeadEnd(change.Status) {
			count++
		}
	}
	for _, op := range normalized.MemoryOperations {
		if evidenceDeadEnd(op.Status) {
			count++
		}
	}
	return count
}

func evidenceDeadEnd(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "error", "failed", "failure", "denied", "rejected", "cancelled", "canceled", "timeout", "timed_out":
		return true
	default:
		return false
	}
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

type passRateInterval struct {
	rate       float64
	lowerBound float64
	upperBound float64
}

func passRateCI(passes, total int, confidence float64) passRateInterval {
	if total <= 0 {
		return passRateInterval{}
	}
	rate := float64(passes) / float64(total)
	z := zForConfidence(confidence)
	n := float64(total)
	denom := 1 + (z*z)/n
	center := (rate + (z*z)/(2*n)) / denom
	margin := (z / denom) * math.Sqrt((rate*(1-rate)+(z*z)/(4*n))/n)
	return passRateInterval{
		rate:       round3(rate),
		lowerBound: round3(math.Max(0, center-margin)),
		upperBound: round3(math.Min(1, center+margin)),
	}
}

func zForConfidence(confidence float64) float64 {
	switch {
	case confidence >= 0.99:
		return 2.576
	case confidence >= 0.975:
		return 2.241
	case confidence >= 0.95:
		return 1.96
	case confidence >= 0.90:
		return 1.645
	case confidence >= 0.80:
		return 1.282
	default:
		return 1.0
	}
}

func flakeRate(passes, total int) float64 {
	if total <= 0 {
		return 0
	}
	fails := total - passes
	majority := passes
	if fails > majority {
		majority = fails
	}
	return round3(1 - (float64(majority) / float64(total)))
}

func majorityTextFlakeRate(samples []string) float64 {
	if len(samples) <= 1 {
		return 0
	}
	counts := map[string]int{}
	maxCount := 0
	for _, sample := range samples {
		counts[sample]++
		if counts[sample] > maxCount {
			maxCount = counts[sample]
		}
	}
	return round3(1 - (float64(maxCount) / float64(len(samples))))
}
