package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"cleanr/cleanr"
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
		Suites: []cleanr.SuiteResult{{
			Name:     "security",
			Passed:   false,
			Duration: 2 * time.Second,
			Findings: []cleanr.Finding{{Severity: "high", Message: "suite issue"}},
			Cases:    []cleanr.CaseResult{{Name: "case-1", Passed: false, Duration: 750 * time.Millisecond, Findings: []cleanr.Finding{{Severity: "critical", Message: "boom"}}}},
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
		"Recommendations",
		"Finding  HIGH: suite issue",
		"Finding  CRITICAL: boom",
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
}
