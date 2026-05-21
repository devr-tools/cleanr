package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cleanr/cleanr/core"
	trendspkg "cleanr/cleanr/trends"
)

type SinkPayload struct {
	Version          string                       `json:"version"`
	Source           string                       `json:"source"`
	SinkType         string                       `json:"sink_type,omitempty"`
	Project          string                       `json:"project,omitempty"`
	Experiment       string                       `json:"experiment,omitempty"`
	Target           string                       `json:"target"`
	BuildID          string                       `json:"build_id,omitempty"`
	GeneratedAt      time.Time                    `json:"generated_at"`
	LocalBlocking    bool                         `json:"local_blocking"`
	RemoteBestEffort bool                         `json:"remote_best_effort"`
	Report           core.Report                  `json:"report"`
	ReplayArtifact   *core.ReplayArtifact         `json:"replay_artifact,omitempty"`
	Attestation      *core.ReleaseGateAttestation `json:"attestation,omitempty"`
}

func EnsureReport(report *core.Report) *core.IntegrationReport {
	if report == nil {
		return nil
	}
	if report.Integrations == nil {
		report.Integrations = &core.IntegrationReport{
			LocalBlocking: true,
			RemoteMode:    "best_effort",
		}
	}
	return report.Integrations
}

func CompareTrendSources(ctx context.Context, cfg core.IntegrationsConfig, report core.Report, configPath string) []core.ExternalTrendReport {
	if len(cfg.TrendSources) == 0 {
		return nil
	}
	baseDir := filepath.Dir(configPath)
	current := trendspkg.BuildRun(report, buildID(report.Metadata))
	results := make([]core.ExternalTrendReport, 0, len(cfg.TrendSources))
	for _, source := range cfg.TrendSources {
		item := core.ExternalTrendReport{
			Name:       displayName(source.Name, source.Type),
			SourceType: strings.TrimSpace(source.Type),
			Blocking:   false,
			BestEffort: true,
			ViewURL:    strings.TrimSpace(source.ViewURL),
		}
		history, err := loadTrendSource(ctx, source, baseDir)
		if err != nil {
			item.Status = "error"
			item.Message = err.Error()
			results = append(results, item)
			continue
		}
		item.HistoryLength = len(history.Runs)
		if len(history.Runs) == 0 {
			item.Status = "empty"
			item.Message = "source contained no retained runs"
			results = append(results, item)
			continue
		}
		previous := trendspkg.LatestRun(history)
		item.Status = "compared"
		item.ComparedBuildID = current.BuildID
		item.LatestBuildID = previous.BuildID
		item.LatestAt = previous.GeneratedAt
		item.PreviousAt = previous.GeneratedAt
		comparison := trendspkg.Compare(current, previous, len(history.Runs)+1)
		item.BuildDiff = comparison.BuildDiff
		if !comparison.Baseline {
			summary := comparison.Summary
			item.Summary = &summary
		}
		results = append(results, item)
	}
	return results
}

func PublishResultSinks(ctx context.Context, cfg core.IntegrationsConfig, report core.Report, replay *core.ReplayArtifact, attestation *core.ReleaseGateAttestation) []core.ResultSinkReport {
	if len(cfg.ResultSinks) == 0 {
		return nil
	}
	results := make([]core.ResultSinkReport, 0, len(cfg.ResultSinks))
	for _, sink := range cfg.ResultSinks {
		item := core.ResultSinkReport{
			Name:       displayName(sink.Name, sink.Type),
			SinkType:   strings.TrimSpace(sink.Type),
			Blocking:   false,
			BestEffort: true,
			Published:  false,
		}
		payload := buildSinkPayload(sink, report, replay, attestation)
		runURL, err := postSinkPayload(ctx, sink, payload)
		if err != nil {
			item.Message = err.Error()
			results = append(results, item)
			continue
		}
		item.Published = true
		item.Message = "published"
		item.RunURL = runURL
		results = append(results, item)
	}
	return results
}

func WriteSummaries(cfg core.IntegrationsConfig, report core.Report, configPath string) []core.SummaryArtifactReport {
	if len(cfg.Summaries) == 0 {
		return nil
	}
	baseDir := filepath.Dir(configPath)
	results := make([]core.SummaryArtifactReport, 0, len(cfg.Summaries))
	for _, summary := range cfg.Summaries {
		item := core.SummaryArtifactReport{
			Name:   displayName(summary.Name, "summary"),
			Format: strings.TrimSpace(summary.Format),
			Output: resolveRelativePath(baseDir, summary.Output),
		}
		data, err := renderSummary(report, summary)
		if err != nil {
			item.Message = err.Error()
			results = append(results, item)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(item.Output), 0o755); err != nil {
			item.Message = err.Error()
			results = append(results, item)
			continue
		}
		if err := os.WriteFile(item.Output, data, 0o644); err != nil {
			item.Message = err.Error()
			results = append(results, item)
			continue
		}
		item.Written = true
		item.Message = "written"
		results = append(results, item)
	}
	return results
}

func loadTrendSource(ctx context.Context, source core.TrendSourceConfig, baseDir string) (trendspkg.HistoryFile, error) {
	switch strings.TrimSpace(source.Type) {
	case "file":
		return trendspkg.LoadFile(resolveRelativePath(baseDir, source.Path))
	case "http":
		client := &http.Client{Timeout: time.Duration(source.TimeoutMS) * time.Millisecond}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(source.URL), nil)
		if err != nil {
			return trendspkg.HistoryFile{}, fmt.Errorf("load trend source %s: %w", displayName(source.Name, source.Type), err)
		}
		applyAuth(req.Header, source.APIKeyEnv)
		applyHeaders(req.Header, source.Headers)
		resp, err := client.Do(req)
		if err != nil {
			return trendspkg.HistoryFile{}, fmt.Errorf("load trend source %s: %w", displayName(source.Name, source.Type), err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		if err != nil {
			return trendspkg.HistoryFile{}, fmt.Errorf("load trend source %s: %w", displayName(source.Name, source.Type), err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return trendspkg.HistoryFile{}, fmt.Errorf("load trend source %s: %s", displayName(source.Name, source.Type), compactHTTPError(resp.StatusCode, data))
		}
		return trendspkg.LoadData(data, source.URL)
	case "braintrust":
		return loadBraintrustTrendSource(ctx, source)
	default:
		return trendspkg.HistoryFile{}, fmt.Errorf("unsupported trend source type %q", source.Type)
	}
}

func postSinkPayload(ctx context.Context, sink core.ResultSinkConfig, payload SinkPayload) (string, error) {
	if runURL, handled, err := publishNativeSink(ctx, sink, payload); handled {
		return runURL, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("publish result sink %s: %w", displayName(sink.Name, sink.Type), err)
	}
	client := &http.Client{Timeout: time.Duration(sink.TimeoutMS) * time.Millisecond}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(sink.Endpoint), bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("publish result sink %s: %w", displayName(sink.Name, sink.Type), err)
	}
	req.Header.Set("Content-Type", "application/json")
	applyAuth(req.Header, sink.APIKeyEnv)
	applyHeaders(req.Header, sink.Headers)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("publish result sink %s: %w", displayName(sink.Name, sink.Type), err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("publish result sink %s: %w", displayName(sink.Name, sink.Type), err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("publish result sink %s: %s", displayName(sink.Name, sink.Type), compactHTTPError(resp.StatusCode, body))
	}
	runURL := discoverRunURL(body)
	if runURL == "" {
		runURL = expandRunURLTemplate(sink.RunURLTemplate, payload)
	}
	return runURL, nil
}

func renderSummary(report core.Report, cfg core.SummaryConfig) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "", "markdown":
		return []byte(renderMarkdownSummary(report)), nil
	case "json":
		payload := map[string]any{
			"target":          report.Name,
			"passed":          report.Passed,
			"generated_at":    report.GeneratedAt,
			"failed_suites":   report.FailedSuites,
			"failed_cases":    report.FailedCases,
			"build_id":        buildID(report.Metadata),
			"trend":           report.Trend,
			"trend_gate":      report.TrendGate,
			"integrations":    report.Integrations,
			"recommendations": report.Recommendations,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("render summary: %w", err)
		}
		return append(data, '\n'), nil
	default:
		return nil, fmt.Errorf("render summary: unsupported format %s", cfg.Format)
	}
}

func renderMarkdownSummary(report core.Report) string {
	var b strings.Builder
	status := "PASS"
	if !report.Passed {
		status = "FAIL"
	}
	fmt.Fprintf(&b, "# cleanr Release Summary\n\n")
	fmt.Fprintf(&b, "- Local gate: `%s` (blocking)\n", status)
	fmt.Fprintf(&b, "- Target: `%s`\n", report.Name)
	if build := buildID(report.Metadata); build != "" {
		fmt.Fprintf(&b, "- Build: `%s`\n", build)
	}
	if !report.GeneratedAt.IsZero() {
		fmt.Fprintf(&b, "- Generated: `%s`\n", report.GeneratedAt.Format(time.RFC3339))
	}
	fmt.Fprintf(&b, "- Failed suites: `%d`\n", report.FailedSuites)
	fmt.Fprintf(&b, "- Failed cases: `%d`\n", report.FailedCases)
	if report.Trend != nil && !report.Trend.Baseline {
		fmt.Fprintf(&b, "- Local trend: `%+d suites`, `%+d cases`, `%s duration`\n", report.Trend.Summary.FailedSuitesDelta, report.Trend.Summary.FailedCasesDelta, report.Trend.Summary.DurationDelta.Round(time.Millisecond))
	}
	if report.TrendGate != nil {
		gateStatus := "SKIPPED"
		if report.TrendGate.Evaluated {
			if report.TrendGate.Passed {
				gateStatus = "PASS"
			} else {
				gateStatus = "FAIL"
			}
		}
		fmt.Fprintf(&b, "- Trend gates: `%s`\n", gateStatus)
	}

	failures := failureSummary(report)
	if len(failures) > 0 {
		fmt.Fprintf(&b, "\n## Local Failures\n\n")
		for _, line := range failures {
			fmt.Fprintf(&b, "- %s\n", line)
		}
	}

	if report.Integrations != nil && len(report.Integrations.TrendSources) > 0 {
		fmt.Fprintf(&b, "\n## Remote Comparisons\n\n")
		for _, source := range report.Integrations.TrendSources {
			line := fmt.Sprintf("`%s`: `%s`", source.Name, strings.ToUpper(emptyStatus(source.Status)))
			if source.Summary != nil {
				line += fmt.Sprintf(" against `%s` with `%+d` failed-case delta", emptyValue(source.LatestBuildID), source.Summary.FailedCasesDelta)
			}
			if source.ViewURL != "" {
				line += fmt.Sprintf(" ([view](%s))", source.ViewURL)
			}
			if source.Message != "" && source.Status != "compared" {
				line += ": " + source.Message
			}
			fmt.Fprintf(&b, "- %s\n", line)
		}
	}

	if report.Integrations != nil {
		links := remoteLinks(report.Integrations.ResultSinks)
		if len(links) > 0 {
			fmt.Fprintf(&b, "\n## Remote Views\n\n")
			for _, line := range links {
				fmt.Fprintf(&b, "- %s\n", line)
			}
		}
	}

	if len(report.Recommendations) > 0 {
		fmt.Fprintf(&b, "\n## Recommendations\n\n")
		for _, rec := range report.Recommendations {
			fmt.Fprintf(&b, "- %s\n", rec)
		}
	}
	return b.String()
}

func failureSummary(report core.Report) []string {
	lines := make([]string, 0)
	for _, suite := range report.Suites {
		for _, finding := range suite.Findings {
			lines = append(lines, fmt.Sprintf("%s: %s", suite.Name, finding.Message))
			if len(lines) == 8 {
				return lines
			}
		}
		for _, c := range suite.Cases {
			if c.Passed && len(c.Findings) == 0 {
				continue
			}
			line := fmt.Sprintf("%s/%s", suite.Name, c.Name)
			if len(c.Findings) > 0 {
				line += ": " + c.Findings[0].Message
			}
			lines = append(lines, line)
			if len(lines) == 8 {
				return lines
			}
		}
	}
	return lines
}

func remoteLinks(items []core.ResultSinkReport) []string {
	out := make([]string, 0)
	for _, item := range items {
		if strings.TrimSpace(item.RunURL) == "" {
			continue
		}
		out = append(out, fmt.Sprintf("`%s`: %s", item.Name, item.RunURL))
	}
	return out
}

func compactHTTPError(statusCode int, body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return fmt.Sprintf("remote endpoint returned HTTP %d", statusCode)
	}
	if len(text) > 240 {
		text = text[:240]
	}
	return fmt.Sprintf("remote endpoint returned HTTP %d: %s", statusCode, text)
}

func discoverRunURL(body []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	for _, key := range []string{"run_url", "view_url", "url"} {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func expandRunURLTemplate(tmpl string, payload SinkPayload) string {
	tmpl = strings.TrimSpace(tmpl)
	if tmpl == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"{{project}}", payload.Project,
		"{{experiment}}", payload.Experiment,
		"{{build_id}}", payload.BuildID,
		"{{target}}", payload.Target,
	)
	return replacer.Replace(tmpl)
}

func resolveRelativePath(baseDir, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}

func applyAuth(headers http.Header, apiKeyEnv string) {
	apiKeyEnv = strings.TrimSpace(apiKeyEnv)
	if apiKeyEnv == "" {
		return
	}
	if value := strings.TrimSpace(os.Getenv(apiKeyEnv)); value != "" {
		headers.Set("Authorization", "Bearer "+value)
	}
}

func applyHeaders(headers http.Header, values map[string]string) {
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		headers.Set(key, value)
	}
}

func buildID(metadata *core.RunMetadata) string {
	if metadata == nil {
		return ""
	}
	return strings.TrimSpace(metadata.BuildID)
}

func displayName(name, fallback string) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		return "integration"
	}
	return fallback
}

func emptyValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "<unset>"
	}
	return value
}

func emptyStatus(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
