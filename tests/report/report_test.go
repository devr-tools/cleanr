package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

func TestWriteReportSupportsAllFormats(t *testing.T) {
	t.Parallel()

	report := cleanr.Report{
		Name:         "demo",
		Passed:       false,
		Duration:     1500 * time.Millisecond,
		TotalSuites:  1,
		FailedSuites: 1,
		TotalCases:   1,
		FailedCases:  1,
		Recommendations: []string{
			"tighten prompts",
		},
		Trend: &cleanr.TrendReport{
			HistoryLength:   2,
			CurrentBuildID:  "build-2",
			PreviousBuildID: "build-1",
			PreviousAt:      time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
			BuildDiff: &cleanr.BuildDiff{
				ModelBefore: "gpt-4.1-mini",
				ModelAfter:  "gpt-4.1",
				ScenarioChanges: []cleanr.ScenarioDiff{{
					Name:         "case-1",
					Status:       "changed",
					InputChanged: true,
				}},
			},
			Summary: cleanr.TrendSummary{
				FailedSuitesDelta: 1,
				FailedCasesDelta:  2,
				DurationDelta:     250 * time.Millisecond,
				RegressedSuites:   1,
			},
			Suites: []cleanr.SuiteTrend{{
				Name:             "drift",
				Status:           "regressed",
				FailedCasesDelta: 1,
				ScoreDelta:       -0.12,
				Drift: &cleanr.DriftTrend{
					SemanticDriftDelta: 0.18,
				},
			}},
		},
		TrendGate: &cleanr.TrendGateReport{
			Enabled:         true,
			Evaluated:       true,
			Passed:          false,
			RequiredWindow:  2,
			AvailableWindow: 2,
			Findings: []cleanr.Finding{{
				Severity: "high",
				Message:  "semantic drift delta 0.180 exceeded gate 0.050",
			}},
		},
		Integrations: &cleanr.IntegrationReport{
			LocalBlocking: true,
			RemoteMode:    "best_effort",
			TrendSources: []cleanr.ExternalTrendReport{{
				Name:       "approved-history",
				Status:     "compared",
				BestEffort: true,
				Summary: &cleanr.TrendSummary{
					FailedCasesDelta: 1,
				},
				ViewURL: "https://braintrust.dev/app/history/build-1",
			}},
			ResultSinks: []cleanr.ResultSinkReport{{
				Name:       "braintrust",
				Published:  true,
				BestEffort: true,
				RunURL:     "https://braintrust.dev/app/release-gate/build-2",
				Message:    "published",
			}},
			Summaries: []cleanr.SummaryArtifactReport{{
				Name:    "pr",
				Output:  "reports/summary.md",
				Written: true,
				Message: "written",
			}},
		},
		Suites: []cleanr.SuiteResult{{
			Name:     "security",
			Passed:   false,
			Duration: 2 * time.Second,
			Findings: []cleanr.Finding{{Severity: "high", Message: "suite issue"}},
			Cases: []cleanr.CaseResult{{
				Name:     "case-1",
				Passed:   false,
				Duration: 750 * time.Millisecond,
				Findings: []cleanr.Finding{{Severity: "critical", Message: "boom"}},
				Details: map[string]any{
					"first_unsupported_claim": "claimed tool execution with no matching invocation: lookup_policy",
					"claimed_tools":           []string{"lookup_policy"},
					"observed_state_actions":  []string{"none"},
				},
			}},
		}},
	}

	var text bytes.Buffer
	if err := cleanr.WriteReport(&text, report, ""); err != nil {
		t.Fatalf("write text report: %v", err)
	}
	textOut := text.String()
	for _, want := range []string{
		"Report Summary",
		"Status      FAIL",
		"Overview",
		"Details",
		"Trends",
		"Trend Gates",
		"semantic drift delta 0.180 exceeded gate 0.050",
		"Compared",
		"BuildDiff",
		"model=gpt-4.1-mini -> gpt-4.1",
		"Scenario",
		"case-1 | changed | input",
		"drift",
		"semantic_drift_delta=+0.180",
		"Integrations",
		"Contract",
		"approved-history",
		"braintrust",
		"reports/summary.md",
		"Recommendations",
		"Finding  HIGH: suite issue",
		"Finding  CRITICAL: boom",
		`claimed_tools`,
		`["lookup_policy"]`,
		`observed_state_actions`,
		`["none"]`,
	} {
		if !strings.Contains(textOut, want) {
			t.Fatalf("expected %q in text report:\n%s", want, textOut)
		}
	}
	if strings.Contains(textOut, "\x1b[") {
		t.Fatalf("unexpected text report: %s", textOut)
	}

	var jsonBuf bytes.Buffer
	if err := cleanr.WriteReport(&jsonBuf, report, "json"); err != nil {
		t.Fatalf("write json report: %v", err)
	}
	var decoded cleanr.Report
	if err := json.Unmarshal(jsonBuf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json report: %v", err)
	}
	if decoded.Name != "demo" || decoded.TotalCases != 1 {
		t.Fatalf("unexpected decoded report: %+v", decoded)
	}

	var html bytes.Buffer
	if err := cleanr.WriteReport(&html, report, "html"); err != nil {
		t.Fatalf("write html report: %v", err)
	}
	htmlOut := html.String()
	for _, want := range []string{
		"<!DOCTYPE html>",
		"Static cleanr report dashboard",
		"devr-tools / cleanr",
		`aria-label="cleanr ascii logo"`,
		`▄▄`,
		"Primary evaluation results",
		`status status-compact status-fail`,
		`class="detail-grid"`,
		`class="detail-key">claimed_tools</div>`,
		`class="detail-chip">lookup_policy</span>`,
		"semantic drift delta 0.180 exceeded gate 0.050",
		"https://braintrust.dev/app/release-gate/build-2",
		"claimed tool execution with no matching invocation",
	} {
		if !strings.Contains(htmlOut, want) {
			t.Fatalf("expected %q in html report:\n%s", want, htmlOut)
		}
	}
	var agent bytes.Buffer
	if err := cleanr.WriteReport(&agent, report, "agent"); err != nil {
		t.Fatalf("write agent report: %v", err)
	}
	var agentDecoded cleanr.AgentReport
	if err := json.Unmarshal(agent.Bytes(), &agentDecoded); err != nil {
		t.Fatalf("decode agent report: %v", err)
	}
	if agentDecoded.Contract.Kind != "cleanr.report.agent" || agentDecoded.Contract.Format != "agent" || agentDecoded.Contract.Version != "v1" {
		t.Fatalf("unexpected agent contract: %+v", agentDecoded.Contract)
	}
	if agentDecoded.Summary.Target != "demo" || agentDecoded.Summary.FindingCount != 3 || agentDecoded.Summary.RecommendationCount != 1 {
		t.Fatalf("unexpected agent summary: %+v", agentDecoded.Summary)
	}
	if len(agentDecoded.Findings) != 3 {
		t.Fatalf("expected 3 flattened findings, got %d", len(agentDecoded.Findings))
	}
	if agentDecoded.Findings[0].Scope != "suite" || agentDecoded.Findings[0].Suite != "security" || agentDecoded.Findings[0].Message != "suite issue" {
		t.Fatalf("unexpected suite finding: %+v", agentDecoded.Findings[0])
	}
	if agentDecoded.Findings[1].Scope != "case" || agentDecoded.Findings[1].Case != "case-1" || agentDecoded.Findings[1].Details["first_unsupported_claim"] != "claimed tool execution with no matching invocation: lookup_policy" {
		t.Fatalf("unexpected case finding: %+v", agentDecoded.Findings[1])
	}
	if agentDecoded.Findings[2].Scope != "trend_gate" || agentDecoded.Findings[2].Message != "semantic drift delta 0.180 exceeded gate 0.050" {
		t.Fatalf("unexpected trend gate finding: %+v", agentDecoded.Findings[2])
	}
	if len(agentDecoded.FixSuggestions) != 2 {
		t.Fatalf("expected 2 fix suggestions, got %+v", agentDecoded.FixSuggestions)
	}
	if agentDecoded.FixSuggestions[0].Kind != "trace_alignment" || agentDecoded.FixSuggestions[0].Case != "case-1" {
		t.Fatalf("unexpected trace alignment suggestion: %+v", agentDecoded.FixSuggestions[0])
	}
	if agentDecoded.FixSuggestions[1].Kind != "stability" || agentDecoded.FixSuggestions[1].Scope != "trend_gate" {
		t.Fatalf("unexpected trend stability suggestion: %+v", agentDecoded.FixSuggestions[1])
	}
	if agentDecoded.Report.Name != "demo" || agentDecoded.Report.TotalCases != 1 {
		t.Fatalf("unexpected embedded report: %+v", agentDecoded.Report)
	}

	var junit bytes.Buffer
	if err := cleanr.WriteReport(&junit, report, "junit"); err != nil {
		t.Fatalf("write junit report: %v", err)
	}
	junitOut := junit.String()
	if !strings.Contains(junitOut, `testsuite name="security"`) || !strings.Contains(junitOut, `failure message="cleanr assertion failed"`) {
		t.Fatalf("unexpected junit report: %s", junitOut)
	}
	if !strings.Contains(junitOut, `time="2.000"`) || !strings.Contains(junitOut, `CRITICAL: boom`) {
		t.Fatalf("unexpected junit timing/findings: %s", junitOut)
	}

	if err := cleanr.WriteReport(&bytes.Buffer{}, report, "markdown"); err == nil {
		t.Fatal("expected unsupported report format error")
	}

	var sarif bytes.Buffer
	if err := cleanr.WriteReport(&sarif, report, "sarif"); err != nil {
		t.Fatalf("write sarif report: %v", err)
	}
	sarifOut := sarif.String()
	for _, want := range []string{
		`"version": "2.1.0"`,
		`"ruleId": "cleanr.security.case-1.critical"`,
		`"level": "error"`,
		`"text": "boom"`,
	} {
		if !strings.Contains(sarifOut, want) {
			t.Fatalf("expected %q in sarif output:\n%s", want, sarifOut)
		}
	}
}
