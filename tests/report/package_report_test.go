package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	reportpkg "github.com/devr-tools/cleanr/cleanr/report"
)

func TestReportPackageSupportsPlainAndColorText(t *testing.T) {
	report := cleanr.Report{
		Name:         "demo",
		Passed:       true,
		GeneratedAt:  time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
		Duration:     1500 * time.Millisecond,
		TotalSuites:  1,
		FailedSuites: 0,
		TotalCases:   1,
		FailedCases:  0,
		Suites: []cleanr.SuiteResult{{
			Name:     "security",
			Passed:   true,
			Duration: time.Second,
			Cases: []cleanr.CaseResult{{
				Name:       "case-1",
				Passed:     true,
				Duration:   500 * time.Millisecond,
				Score:      0.99,
				LatencyP95: 200 * time.Millisecond,
				Details: map[string]any{
					"alpha": "x",
					"beta":  3.14159,
					"skip":  []string{"not-scalar"},
				},
			}},
			Meta: map[string]any{
				"passed": true,
				"count":  4,
			},
		}},
		Trend: &cleanr.TrendReport{
			PreviousAt: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
			Summary: cleanr.TrendSummary{
				FailedSuitesDelta: -1,
				FailedCasesDelta:  -2,
				DurationDelta:     -300 * time.Millisecond,
				ImprovedSuites:    1,
			},
			Suites: []cleanr.SuiteTrend{{
				Name:   "security",
				Status: "improved",
				Drift: &cleanr.DriftTrend{
					NormalizedDriftDelta:          -0.10,
					SemanticConsistencyScoreDelta: 0.05,
				},
			}},
		},
		TrendGate: &cleanr.TrendGateReport{
			Enabled:   true,
			Evaluated: false,
			Passed:    false,
		},
	}

	plain := reportpkg.Text(report)
	for _, want := range []string{
		"Report Summary",
		"Overview",
		"Details",
		"Trends",
		"Trend Gates",
		"score 0.99",
		"p95 200ms",
		"alpha=x",
		"beta=3.14",
		"count=4",
		"improved_suites=1",
		"normalized_drift_delta=-0.100",
		"semantic_consistency_score_delta=+0.050",
		"Status",
		"SKIPPED",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected %q in plain report:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("unexpected ANSI codes in plain report:\n%s", plain)
	}

	t.Setenv("FORCE_COLOR", "1")
	t.Setenv("TERM", "xterm-256color")
	var color bytes.Buffer
	if err := reportpkg.Write(&color, report, "text"); err != nil {
		t.Fatalf("write color report: %v", err)
	}
	if !strings.Contains(color.String(), "\x1b[") {
		t.Fatalf("expected ANSI color codes in report:\n%s", color.String())
	}

	var agent bytes.Buffer
	if err := reportpkg.Write(&agent, report, "agent"); err != nil {
		t.Fatalf("write agent report: %v", err)
	}
	var decoded cleanr.AgentReport
	if err := json.Unmarshal(agent.Bytes(), &decoded); err != nil {
		t.Fatalf("decode agent report: %v", err)
	}
	if decoded.Contract.Kind != "cleanr.report.agent" || decoded.Summary.Target != "demo" {
		t.Fatalf("unexpected agent report: %+v", decoded)
	}

	var html bytes.Buffer
	if err := reportpkg.Write(&html, report, "html"); err != nil {
		t.Fatalf("write html report: %v", err)
	}
	if out := html.String(); !strings.Contains(out, "<!DOCTYPE html>") || !strings.Contains(out, "Static cleanr report dashboard") || !strings.Contains(out, "devr-tools / cleanr") || !strings.Contains(out, `aria-label="cleanr ascii logo"`) || !strings.Contains(out, `▄▄`) {
		t.Fatalf("unexpected html report:\n%s", out)
	}
}
