package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cleanr/cleanr"
	"cleanr/internal/devtools"
)

func TestReportUsesBuiltInFailPreset(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := devtools.NewRunner(t.TempDir(), &stdout, &stderr)

	if err := runner.Report(devtools.ReportOptions{}); err != nil {
		t.Fatalf("report: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{
		"rendering text report from built-in fail preset",
		"cleanr FAIL",
		"token-optimization",
		"estimated_savings=214",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestReportRendersInputJSONFile(t *testing.T) {
	reportJSON, err := json.Marshal(cleanr.Report{
		Name:         "fixture",
		Passed:       true,
		GeneratedAt:  time.Date(2026, time.May, 20, 15, 0, 0, 0, time.UTC),
		Duration:     950 * time.Millisecond,
		TotalSuites:  1,
		FailedSuites: 0,
		TotalCases:   1,
		FailedCases:  0,
		Suites: []cleanr.SuiteResult{{
			Name:     "security",
			Passed:   true,
			Duration: 950 * time.Millisecond,
			Cases: []cleanr.CaseResult{{
				Name:     "case-1",
				Passed:   true,
				Duration: 200 * time.Millisecond,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}

	workDir := t.TempDir()
	inputPath := filepath.Join(workDir, "report.json")
	if err := os.WriteFile(inputPath, reportJSON, 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := devtools.NewRunner(workDir, &stdout, &stderr)

	if err := runner.Report(devtools.ReportOptions{Input: "report.json"}); err != nil {
		t.Fatalf("report: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{
		"rendering text report from",
		"cleanr PASS",
		"target: fixture",
		"security",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
}
