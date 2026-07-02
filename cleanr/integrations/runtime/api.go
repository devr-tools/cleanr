package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
	reportpkg "github.com/devr-tools/cleanr/cleanr/report"
	trendspkg "github.com/devr-tools/cleanr/cleanr/trends"
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
		applyAuth(req.Header, source.APIKeyEnv, source.URL)
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
	case "langsmith", "openllmetry", "provider_logs":
		return loadExternalTrendSource(ctx, source, baseDir)
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
	applyAuth(req.Header, sink.APIKeyEnv, sink.Endpoint)
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
	case "html":
		var buf bytes.Buffer
		if err := reportpkg.Write(&buf, report, "html"); err != nil {
			return nil, fmt.Errorf("render summary: %w", err)
		}
		return buf.Bytes(), nil
	default:
		return nil, fmt.Errorf("render summary: unsupported format %s", cfg.Format)
	}
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

// integrationEgressAllowlist holds host suffixes trusted to receive integration
// credentials. A provider-secret-looking env var is only ever sent to a host
// that matches one of these suffixes (or a loopback host used for testing).
var integrationEgressAllowlist = []string{
	"braintrust.dev",
	"langfuse.com",
	"posthog.com",
	"i.posthog.com",
	"smith.langchain.com",
	"api.smith.langchain.com",
}

// providerSecretTokens are substrings that mark an env var name as a
// third-party provider credential which must not be exfiltrated to an
// arbitrary, config-specified host.
var providerSecretTokens = []string{
	"OPENAI", "ANTHROPIC", "CLAUDE", "AWS", "AZURE", "GCP", "GOOGLE",
	"GEMINI", "VERTEX", "COHERE", "MISTRAL", "HUGGINGFACE", "HUGGING_FACE",
	"HF_TOKEN", "GITHUB", "GITLAB", "SLACK", "STRIPE", "TWILIO",
	"SENDGRID", "DATABASE_URL", "PRIVATE_KEY", "SESSION_TOKEN",
}

// applyAuth reads the credential named by apiKeyEnv and attaches it as a Bearer
// token for the request to destURL. To prevent a config from exfiltrating a
// well-known provider secret (e.g. OPENAI_API_KEY) to an arbitrary host, an env
// var whose name matches a provider-secret pattern is only sent to hosts on the
// egress allowlist (or loopback). Every credential send is logged with the env
// var name and destination host.
func applyAuth(headers http.Header, apiKeyEnv, destURL string) {
	apiKeyEnv = strings.TrimSpace(apiKeyEnv)
	if apiKeyEnv == "" {
		return
	}
	value := strings.TrimSpace(os.Getenv(apiKeyEnv))
	if value == "" {
		return
	}

	host := destinationHost(destURL)
	if looksLikeProviderSecret(apiKeyEnv) && !hostAllowedForSecret(host) {
		log.Printf("cleanr integrations: refusing to send credential %q to untrusted host %q", apiKeyEnv, host)
		return
	}
	log.Printf("cleanr integrations: sending credential %q to host %q", apiKeyEnv, host)
	headers.Set("Authorization", "Bearer "+value)
}

func destinationHost(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

func looksLikeProviderSecret(name string) bool {
	upper := strings.ToUpper(strings.TrimSpace(name))
	for _, token := range providerSecretTokens {
		if strings.Contains(upper, token) {
			return true
		}
	}
	return false
}

func hostAllowedForSecret(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	for _, suffix := range integrationEgressAllowlist {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
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
