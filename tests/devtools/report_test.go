package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cleanr/cleanr"
	"cleanr/internal/devtools"
)

func TestDevtoolsReportSupportsPresetsAndInputFiles(t *testing.T) {
	var stdout bytes.Buffer
	runner := devtools.NewRunner(t.TempDir(), &stdout, &stdout)
	if err := runner.Report(devtools.ReportOptions{Preset: "pass", Format: "json"}); err != nil {
		t.Fatalf("report pass preset: %v", err)
	}
	if !strings.Contains(stdout.String(), `"passed": true`) {
		t.Fatalf("unexpected pass preset output: %s", stdout.String())
	}

	stdout.Reset()
	if err := runner.Report(devtools.ReportOptions{Preset: "", Format: "text"}); err != nil {
		t.Fatalf("report fail preset: %v", err)
	}
	if !strings.Contains(stdout.String(), "rendering text report from built-in fail preset") {
		t.Fatalf("unexpected fail preset output: %s", stdout.String())
	}

	if err := runner.Report(devtools.ReportOptions{Preset: "unknown"}); err == nil {
		t.Fatal("expected unsupported preset error")
	}

	report := cleanr.Report{Name: "from-file", Passed: true}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	workDir := t.TempDir()
	path := filepath.Join(workDir, "report.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write report file: %v", err)
	}
	stdout.Reset()
	runner = devtools.NewRunner(workDir, &stdout, &stdout)
	if err := runner.Report(devtools.ReportOptions{Input: "report.json", Format: "json"}); err != nil {
		t.Fatalf("report file input: %v", err)
	}
	if !strings.Contains(stdout.String(), `"name": "from-file"`) {
		t.Fatalf("unexpected file report output: %s", stdout.String())
	}

	if err := runner.Report(devtools.ReportOptions{Input: "missing.json"}); err == nil {
		t.Fatal("expected missing file error")
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("write broken report file: %v", err)
	}
	if err := runner.Report(devtools.ReportOptions{Input: "report.json"}); err == nil {
		t.Fatal("expected invalid report input error")
	}
}
