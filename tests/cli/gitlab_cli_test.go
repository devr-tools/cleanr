package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/cli"
)

func TestRunCommandWritesGitLabDotenvAndAnnotations(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = true
	cfg.Suites.Security.MaxPIIMatches = 0
	cfg.Suites.Security.DangerousToolIndicators = []string{}
	cfg.Suites.Security.SecretExposureIndicators = []string{}
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Scenarios = []cleanr.Scenario{{
		Name:             "missing-phrase",
		Input:            "hello",
		ExpectedContains: []string{"missing"},
	}}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	dotenvPath := filepath.Join(dir, "gl-cleanr.env")
	annotationsPath := filepath.Join(dir, "gl-cleanr-annotations.json")
	reportPath := filepath.Join(dir, "reports", "cleanr-report.json")
	t.Setenv("CI_PROJECT_URL", "https://gitlab.example.com/group/project")
	t.Setenv("CI_JOB_ID", "321")
	t.Setenv("CI_JOB_URL", "https://gitlab.example.com/group/project/-/jobs/321")

	restoreTransport := stubCLITransport(t, cliRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"output":{"text":"hello there"}}`
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}))
	defer restoreTransport()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{
		"run",
		"-config", configPath,
		"-format", "json",
		"-output", reportPath,
		"-gitlab-dotenv", dotenvPath,
		"-gitlab-annotations", annotationsPath,
	}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected failing run exit code 1, got %d stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}

	dotenvBody, err := os.ReadFile(dotenvPath)
	if err != nil {
		t.Fatalf("read dotenv output: %v", err)
	}
	dotenvText := string(dotenvBody)
	for _, want := range []string{
		"CLEANR_RUN_GATE_PASSED=false",
		"CLEANR_RUN_FAILED_SUITES=1",
		"CLEANR_RUN_FAILED_CASES=1",
		"CLEANR_RUN_NEW_FAILURES=0",
		"CLEANR_RUN_WORSENED_DRIFT=0",
		"CLEANR_RUN_GATE_SUMMARY=local gate fail, 1 failed suites, 1 failed cases",
		"CLEANR_RUN_REPORT_PATH=" + reportPath,
	} {
		if !strings.Contains(dotenvText, want) {
			t.Fatalf("expected %q in GitLab dotenv output:\n%s", want, dotenvText)
		}
	}

	var annotations map[string][]map[string]map[string]string
	annotationsBody, err := os.ReadFile(annotationsPath)
	if err != nil {
		t.Fatalf("read annotations output: %v", err)
	}
	if err := json.Unmarshal(annotationsBody, &annotations); err != nil {
		t.Fatalf("decode annotations output: %v\n%s", err, string(annotationsBody))
	}
	items := annotations["cleanr_run"]
	if len(items) < 2 {
		t.Fatalf("expected at least two GitLab annotations, got %d: %s", len(items), string(annotationsBody))
	}
	var foundReport, foundJob bool
	for _, item := range items {
		link := item["external_link"]
		switch link["label"] {
		case "cleanr report artifact":
			foundReport = strings.Contains(link["url"], "/-/jobs/321/artifacts/file/")
		case "GitLab job":
			foundJob = link["url"] == "https://gitlab.example.com/group/project/-/jobs/321"
		}
	}
	if !foundReport {
		t.Fatalf("expected cleanr report artifact link in annotations: %s", string(annotationsBody))
	}
	if !foundJob {
		t.Fatalf("expected GitLab job link in annotations: %s", string(annotationsBody))
	}
}

func TestDatasetReviewCommandWritesGitLabDotenvAndAnnotations(t *testing.T) {
	baseCfg := cleanr.ExampleConfig()
	baseCfg.Scenarios = []cleanr.Scenario{
		{Name: "existing", System: "You are helpful.", Input: "Reset my password."},
	}

	dir := t.TempDir()
	basePath := filepath.Join(dir, "cleanr.yaml")
	if err := cleanr.WriteConfigFile(basePath, baseCfg); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	dataset := cleanr.ScenarioDataset{
		Version:     "v1alpha1",
		Source:      "cleanr-generation",
		Target:      baseCfg.Target.Name,
		GeneratedAt: time.Now().UTC(),
		Scenarios: []cleanr.ScenarioDatasetEntry{
			{Scenario: cleanr.Scenario{Name: "needs-review", System: "You are helpful.", Input: "What is the refund policy?", Tags: []string{"generated"}}},
			{Scenario: cleanr.Scenario{Name: "dup", System: "You are helpful.", Input: "Reset my password.", Tags: []string{"generated"}}},
		},
	}
	datasetPath := filepath.Join(dir, "dataset.yaml")
	if err := cleanr.WriteScenarioDatasetFile(datasetPath, dataset); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	dotenvPath := filepath.Join(dir, "gl-review.env")
	annotationsPath := filepath.Join(dir, "gl-review-annotations.json")
	reviewedPath := filepath.Join(dir, "reviewed.yaml")
	mergedPath := filepath.Join(dir, "merged.yaml")
	t.Setenv("CI_PROJECT_URL", "https://gitlab.example.com/group/project")
	t.Setenv("CI_JOB_ID", "654")
	t.Setenv("CI_JOB_URL", "https://gitlab.example.com/group/project/-/jobs/654")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run([]string{
		"dataset", "review",
		"-input", datasetPath,
		"-base-config", basePath,
		"-output", reviewedPath,
		"-merge-output", mergedPath,
		"-approve", "dup",
		"-fail-on-pending",
		"-max-duplicates", "0",
		"-gitlab-dotenv", dotenvPath,
		"-gitlab-annotations", annotationsPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected gate failure exit code 1, got %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	dotenvBody, err := os.ReadFile(dotenvPath)
	if err != nil {
		t.Fatalf("read dotenv output: %v", err)
	}
	dotenvText := string(dotenvBody)
	for _, want := range []string{
		"CLEANR_REVIEW_GATE_PASSED=false",
		"CLEANR_REVIEW_PENDING=1",
		"CLEANR_REVIEW_DUPLICATES=1",
		"CLEANR_REVIEW_ARTIFACT=" + reviewedPath,
		"CLEANR_REVIEW_MERGE_OUTPUT=" + mergedPath,
	} {
		if !strings.Contains(dotenvText, want) {
			t.Fatalf("expected %q in GitLab dotenv output:\n%s", want, dotenvText)
		}
	}

	var annotations map[string][]map[string]map[string]string
	annotationsBody, err := os.ReadFile(annotationsPath)
	if err != nil {
		t.Fatalf("read annotations output: %v", err)
	}
	if err := json.Unmarshal(annotationsBody, &annotations); err != nil {
		t.Fatalf("decode annotations output: %v\n%s", err, string(annotationsBody))
	}
	items := annotations["cleanr_review"]
	if len(items) < 2 {
		t.Fatalf("expected review annotations, got %d: %s", len(items), string(annotationsBody))
	}
	var foundReviewed, foundMerged bool
	for _, item := range items {
		link := item["external_link"]
		switch link["label"] {
		case "cleanr reviewed dataset":
			foundReviewed = strings.Contains(link["url"], "/-/jobs/654/artifacts/file/")
		case "cleanr merged config":
			foundMerged = strings.Contains(link["url"], "/-/jobs/654/artifacts/file/")
		}
	}
	if !foundReviewed {
		t.Fatalf("expected reviewed dataset link in annotations: %s", string(annotationsBody))
	}
	if !foundMerged {
		t.Fatalf("expected merged config link in annotations: %s", string(annotationsBody))
	}
}
