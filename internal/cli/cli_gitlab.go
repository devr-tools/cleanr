package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/devr-tools/cleanr/cleanr"
)

type gitlabAnnotationReport map[string][]map[string]gitlabExternalLink

type gitlabExternalLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

func maybeWriteGitLabRunOutputs(opts gitlabOptions, cfg cleanr.Config, report cleanr.Report, resolvedConfigPath string) error {
	if strings.TrimSpace(opts.DotenvPath) == "" && strings.TrimSpace(opts.AnnotationsPath) == "" {
		return nil
	}
	reportPath := resolveConfigRelativePath(resolvedConfigPath, cfg.Reporting.Output)
	if strings.TrimSpace(opts.DotenvPath) != "" {
		values := map[string]string{
			"CLEANR_RUN_GATE_PASSED":      boolString(report.Passed),
			"CLEANR_RUN_FAILED_SUITES":    fmt.Sprintf("%d", report.FailedSuites),
			"CLEANR_RUN_FAILED_CASES":     fmt.Sprintf("%d", report.FailedCases),
			"CLEANR_RUN_NEW_FAILURES":     fmt.Sprintf("%d", len(runCaseRegressions(report))),
			"CLEANR_RUN_WORSENED_DRIFT":   fmt.Sprintf("%d", len(runWorsenedDrift(report))),
			"CLEANR_RUN_REVIEW_SCENARIOS": strings.Join(runRecommendedScenarioNames(report), ","),
			"CLEANR_RUN_GATE_SUMMARY":     sanitizeGitLabDotenvValue(runGateSummary(report)),
			"CLEANR_RUN_REPORT_PATH":      reportPath,
			"CLEANR_RUN_TARGET":           strings.TrimSpace(report.Name),
		}
		if report.TrendGate != nil {
			values["CLEANR_RUN_TREND_GATE_PASSED"] = boolString(report.TrendGate.Passed)
		}
		if err := writeGitLabDotenvFile(opts.DotenvPath, values); err != nil {
			return err
		}
	}
	if strings.TrimSpace(opts.AnnotationsPath) != "" {
		links := collectRunGitLabLinks(cfg, report, resolvedConfigPath, reportPath)
		if err := writeGitLabAnnotationsFile(opts.AnnotationsPath, "cleanr_run", links); err != nil {
			return err
		}
	}
	return nil
}

func maybeWriteGitLabReviewOutputs(opts gitlabOptions, ctx datasetReviewCommandContext, gate datasetReviewGateResult) error {
	if strings.TrimSpace(opts.DotenvPath) == "" && strings.TrimSpace(opts.AnnotationsPath) == "" {
		return nil
	}
	if strings.TrimSpace(opts.DotenvPath) != "" {
		values := map[string]string{
			"CLEANR_REVIEW_GATE_PASSED":   boolString(gate.Passed),
			"CLEANR_REVIEW_TOTAL":         fmt.Sprintf("%d", ctx.Reviewed.Summary.TotalCandidates),
			"CLEANR_REVIEW_APPROVED":      fmt.Sprintf("%d", ctx.Reviewed.ApprovedScenarios),
			"CLEANR_REVIEW_REJECTED":      fmt.Sprintf("%d", ctx.Reviewed.RejectedScenarios),
			"CLEANR_REVIEW_PENDING":       fmt.Sprintf("%d", ctx.Reviewed.PendingScenarios),
			"CLEANR_REVIEW_NEW":           fmt.Sprintf("%d", ctx.Reviewed.Summary.NewCandidates),
			"CLEANR_REVIEW_MODIFIED":      fmt.Sprintf("%d", ctx.Reviewed.Summary.Modified),
			"CLEANR_REVIEW_DUPLICATES":    fmt.Sprintf("%d", ctx.Reviewed.Summary.Duplicates),
			"CLEANR_REVIEW_UNCHANGED":     fmt.Sprintf("%d", ctx.Reviewed.Summary.Unchanged),
			"CLEANR_REVIEW_ARTIFACT":      ctx.OutputPath,
			"CLEANR_REVIEW_POLICY_PATH":   ctx.Reviewed.PolicyPath,
			"CLEANR_REVIEW_MERGE_OUTPUT":  ctx.MergePath,
			"CLEANR_REVIEW_TOP_CANDIDATE": topReviewedScenarioName(ctx.Reviewed),
			"CLEANR_REVIEW_TOP_SCORE":     fmt.Sprintf("%d", topReviewedScenarioScore(ctx.Reviewed)),
		}
		if err := writeGitLabDotenvFile(opts.DotenvPath, values); err != nil {
			return err
		}
	}
	if strings.TrimSpace(opts.AnnotationsPath) != "" {
		links := collectReviewGitLabLinks(ctx)
		if err := writeGitLabAnnotationsFile(opts.AnnotationsPath, "cleanr_review", links); err != nil {
			return err
		}
	}
	return nil
}

func writeGitLabDotenvFile(path string, values map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("prepare GitLab dotenv output: %w", err)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		value := sanitizeGitLabDotenvValue(values[key])
		if strings.TrimSpace(key) == "" {
			continue
		}
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(value)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write GitLab dotenv output: %w", err)
	}
	return nil
}

func sanitizeGitLabDotenvValue(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func writeGitLabAnnotationsFile(path, section string, links []gitlabExternalLink) error {
	report := gitlabAnnotationReport{}
	annotations := make([]map[string]gitlabExternalLink, 0, len(links))
	for _, link := range links {
		if strings.TrimSpace(link.Label) == "" || strings.TrimSpace(link.URL) == "" {
			continue
		}
		annotations = append(annotations, map[string]gitlabExternalLink{
			"external_link": link,
		})
	}
	report[section] = annotations
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal GitLab annotations output: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("prepare GitLab annotations output: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write GitLab annotations output: %w", err)
	}
	return nil
}

func collectRunGitLabLinks(cfg cleanr.Config, report cleanr.Report, resolvedConfigPath, reportPath string) []gitlabExternalLink {
	links := make([]gitlabExternalLink, 0, 4)
	if artifactURL := gitLabArtifactFileURL(reportPath); artifactURL != "" {
		links = append(links, gitlabExternalLink{Label: "cleanr report artifact", URL: artifactURL})
	}
	replayPath := resolveConfigRelativePath(resolvedConfigPath, cfg.Reporting.ReplayArtifactFile)
	if artifactURL := gitLabArtifactFileURL(replayPath); artifactURL != "" {
		links = append(links, gitlabExternalLink{Label: "cleanr replay artifact", URL: artifactURL})
	}
	attestationPath := resolveConfigRelativePath(resolvedConfigPath, cfg.Governance.Attestation.Output)
	if artifactURL := gitLabArtifactFileURL(attestationPath); artifactURL != "" {
		links = append(links, gitlabExternalLink{Label: "cleanr attestation artifact", URL: artifactURL})
	}
	for _, sink := range reportIntegrationLinks(report) {
		links = append(links, sink)
	}
	if jobURL := strings.TrimSpace(os.Getenv("CI_JOB_URL")); jobURL != "" {
		links = append(links, gitlabExternalLink{Label: "GitLab job", URL: jobURL})
	}
	return uniqueGitLabLinks(links)
}

func collectReviewGitLabLinks(ctx datasetReviewCommandContext) []gitlabExternalLink {
	links := make([]gitlabExternalLink, 0, 3)
	if artifactURL := gitLabArtifactFileURL(ctx.OutputPath); artifactURL != "" {
		links = append(links, gitlabExternalLink{Label: "cleanr reviewed dataset", URL: artifactURL})
	}
	if artifactURL := gitLabArtifactFileURL(ctx.MergePath); artifactURL != "" {
		links = append(links, gitlabExternalLink{Label: "cleanr merged config", URL: artifactURL})
	}
	if jobURL := strings.TrimSpace(os.Getenv("CI_JOB_URL")); jobURL != "" {
		links = append(links, gitlabExternalLink{Label: "GitLab job", URL: jobURL})
	}
	return uniqueGitLabLinks(links)
}

func reportIntegrationLinks(report cleanr.Report) []gitlabExternalLink {
	if report.Integrations == nil {
		return nil
	}
	links := make([]gitlabExternalLink, 0, len(report.Integrations.ResultSinks))
	for _, sink := range report.Integrations.ResultSinks {
		if strings.TrimSpace(sink.RunURL) == "" {
			continue
		}
		label := "cleanr run URL"
		if strings.TrimSpace(sink.Name) != "" {
			label = "cleanr " + strings.TrimSpace(sink.Name)
		}
		links = append(links, gitlabExternalLink{Label: label, URL: strings.TrimSpace(sink.RunURL)})
	}
	return links
}

func uniqueGitLabLinks(links []gitlabExternalLink) []gitlabExternalLink {
	seen := map[string]struct{}{}
	out := make([]gitlabExternalLink, 0, len(links))
	for _, link := range links {
		key := strings.TrimSpace(link.Label) + "\x00" + strings.TrimSpace(link.URL)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, link)
	}
	return out
}

func gitLabArtifactFileURL(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	projectURL := strings.TrimSpace(os.Getenv("CI_PROJECT_URL"))
	jobID := strings.TrimSpace(os.Getenv("CI_JOB_ID"))
	if projectURL == "" || jobID == "" {
		return ""
	}
	normalized := filepath.ToSlash(strings.TrimPrefix(path, "./"))
	return strings.TrimRight(projectURL, "/") + "/-/jobs/" + jobID + "/artifacts/file/" + url.PathEscape(normalized)
}
