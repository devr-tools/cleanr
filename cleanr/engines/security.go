package engines

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type SecurityEngine struct {
	cache *responseCache
}

func (SecurityEngine) Name() string { return "security" }

var securityPIIPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	regexp.MustCompile(`\b(?:\d[ -]*?){13,16}\b`),
	regexp.MustCompile(`[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}`),
}

func (e SecurityEngine) Run(ctx context.Context, runCtx *core.RunContext) core.SuiteResult {
	cfg := runCtx.Config.Suites.Security
	piiPatterns := securityPIIPatterns
	var extraPatterns []*regexp.Regexp
	for _, pattern := range cfg.LeakPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			extraPatterns = append(extraPatterns, re)
		}
	}

	scenarios := runCtx.Config.Scenarios
	cases := make([]core.CaseResult, len(scenarios))
	runBoundedByIndex(ctx, len(scenarios), runCtx.Config.CaseConcurrency(), func(i int) {
		cases[i] = e.evaluateScenario(ctx, runCtx, scenarios[i], cfg, piiPatterns, extraPatterns)
	})
	return core.SuiteResult{Name: "security", Passed: allPassed(cases), Cases: cases}
}

func (e SecurityEngine) evaluateScenario(ctx context.Context, runCtx *core.RunContext, scenario core.Scenario, cfg core.SecurityConfig, piiPatterns, extraPatterns []*regexp.Regexp) core.CaseResult {
	start := time.Now()
	resp := e.cache.invoke(ctx, runCtx.Target, scenarioRequest(scenario, runCtx.Config.Target.Timeout()))
	findings := responseFindings(resp, nil)
	findings = append(findings, evaluateScenarioAssertions(scenario, resp)...)
	text := resp.Text
	piiMatches := 0
	for _, re := range piiPatterns {
		piiMatches += len(re.FindAllString(text, -1))
	}
	for _, re := range extraPatterns {
		if match := re.FindString(text); match != "" {
			findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("matched custom leak pattern: %s", match)})
		}
	}
	if piiMatches > cfg.MaxPIIMatches {
		findings = append(findings, core.Finding{Severity: "critical", Message: fmt.Sprintf("detected %d PII-like matches", piiMatches)})
	}
	for _, indicator := range cfg.DangerousToolIndicators {
		if strings.Contains(strings.ToLower(text), strings.ToLower(indicator)) {
			findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("dangerous tool indicator present: %s", indicator)})
		}
	}
	for _, indicator := range cfg.SecretExposureIndicators {
		if strings.Contains(text, indicator) {
			findings = append(findings, core.Finding{Severity: "critical", Message: fmt.Sprintf("secret exposure indicator present: %s", indicator)})
		}
	}
	return core.CaseResult{
		Name:     scenario.Name,
		Passed:   len(findings) == 0,
		Duration: time.Since(start),
		Findings: findings,
		Details: responseDetails(resp, map[string]any{
			"assertion_count": len(scenarioAssertions(scenario)),
			"pii_matches":     piiMatches,
			"status_code":     resp.StatusCode,
			"response_len":    len(resp.Text),
		}),
	}
}
