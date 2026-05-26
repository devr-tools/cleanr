package runtime

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
	trendspkg "github.com/devr-tools/cleanr/cleanr/trends"
)

const defaultBraintrustBaseURL = "https://api.braintrust.dev"

type braintrustClient struct {
	http *jsonAPIClient
}

type braintrustProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type braintrustExperiment struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"project_id"`
	Name      string         `json:"name"`
	Created   time.Time      `json:"created"`
	Metadata  map[string]any `json:"metadata"`
}

type braintrustExperimentList struct {
	Objects []braintrustExperiment `json:"objects"`
}

type braintrustExperimentSummary struct {
	ProjectURL    string `json:"project_url"`
	ExperimentURL string `json:"experiment_url"`
}

type braintrustBTQLResponse struct {
	Data []map[string]any `json:"data"`
}

func useNativeBraintrustSink(sink core.ResultSinkConfig) bool {
	if strings.TrimSpace(sink.Type) != "braintrust" {
		return false
	}
	if strings.TrimSpace(sink.Project) == "" {
		return false
	}
	return true
}

func newBraintrustClient(baseURL, endpoint, apiKeyEnv string, headers map[string]string, timeoutMS int) *braintrustClient {
	return &braintrustClient{
		http: newJSONAPIClient(
			normalizedBaseURL(baseURL, endpoint, defaultBraintrustBaseURL),
			headers,
			timeoutMS,
			func(h http.Header) { applyAuth(h, apiKeyEnv) },
		),
	}
}

func postBraintrustSinkPayload(ctx context.Context, sink core.ResultSinkConfig, payload SinkPayload) (string, error) {
	client := newBraintrustClient(sink.BaseURL, sink.Endpoint, sink.APIKeyEnv, sink.Headers, sink.TimeoutMS)
	projectName := strings.TrimSpace(sink.Project)
	if projectName == "" {
		projectName = payload.Target
	}
	family := integrationFamily(sink.Experiment)

	project, err := client.createProject(ctx, projectName)
	if err != nil {
		return "", fmt.Errorf("publish result sink %s: %w", displayName(sink.Name, sink.Type), err)
	}

	experimentName := family + "/" + runScopeSuffix(payload.BuildID, payload.GeneratedAt)
	experimentBody := map[string]any{
		"project_id": project.ID,
		"name":       experimentName,
		"ensure_new": true,
		"metadata": map[string]any{
			"cleanr": map[string]any{
				"source":             payload.Source,
				"family":             family,
				"target":             payload.Target,
				"build_id":           payload.BuildID,
				"generated_at":       payload.GeneratedAt,
				"local_blocking":     payload.LocalBlocking,
				"remote_best_effort": payload.RemoteBestEffort,
			},
		},
	}
	var experiment braintrustExperiment
	if err := client.http.postJSON(ctx, "/v1/experiment", experimentBody, &experiment); err != nil {
		return "", fmt.Errorf("publish result sink %s: %w", displayName(sink.Name, sink.Type), err)
	}

	historyRun := trendspkg.BuildRun(payload.Report, payload.BuildID)
	events := buildBraintrustEvents(payload, family, historyRun)
	if err := client.http.postJSON(ctx, path.Join("/v1/experiment", experiment.ID, "insert"), map[string]any{
		"events": events,
	}, nil); err != nil {
		return "", fmt.Errorf("publish result sink %s: %w", displayName(sink.Name, sink.Type), err)
	}

	var summary braintrustExperimentSummary
	if err := client.http.getJSON(ctx, path.Join("/v1/experiment", experiment.ID, "summarize"), nil, &summary); err == nil && strings.TrimSpace(summary.ExperimentURL) != "" {
		return strings.TrimSpace(summary.ExperimentURL), nil
	}
	return expandRunURLWithValues(sink.RunURLTemplate, payload, nil), nil
}

func loadBraintrustTrendSource(ctx context.Context, source core.TrendSourceConfig) (trendspkg.HistoryFile, error) {
	client := newBraintrustClient(source.BaseURL, source.URL, source.APIKeyEnv, source.Headers, source.TimeoutMS)
	family := strings.TrimSpace(source.Experiment)
	limit := source.HistoryLimit
	if limit <= 0 {
		limit = 10
	}
	experiments, err := client.listExperiments(ctx, source.APIKeyEnv, source.Project, family, limit)
	if err != nil {
		return trendspkg.HistoryFile{}, err
	}
	history := trendspkg.NewHistory(source.Project)
	if len(experiments) == 0 {
		return history, nil
	}
	for i, j := 0, len(experiments)-1; i < j; i, j = i+1, j-1 {
		experiments[i], experiments[j] = experiments[j], experiments[i]
	}
	for _, experiment := range experiments {
		run, err := client.fetchHistoryRun(ctx, source.APIKeyEnv, experiment.ID)
		if err != nil {
			return trendspkg.HistoryFile{}, err
		}
		if history.Target == "" && strings.TrimSpace(run.BuildID) != "" {
			history.Target = source.Project
		}
		history.Runs = append(history.Runs, run)
		if experiment.Created.After(history.UpdatedAt) {
			history.UpdatedAt = experiment.Created.UTC()
		}
	}
	return history, nil
}

func buildBraintrustEvents(payload SinkPayload, family string, historyRun trendspkg.HistoryRun) []map[string]any {
	events := []map[string]any{{
		"id": braintrustEventID("run", payload.Target, payload.BuildID, family),
		"input": map[string]any{
			"kind":         "cleanr-release-gate",
			"target":       payload.Target,
			"build_id":     payload.BuildID,
			"generated_at": payload.GeneratedAt,
		},
		"output": braintrustRunOutput(payload),
		"scores": map[string]any{
			"cleanr_passed": boolScore(payload.Report.Passed),
			"failed_suites": float64(payload.Report.FailedSuites),
			"failed_cases":  float64(payload.Report.FailedCases),
		},
		"metadata": map[string]any{
			"cleanr": map[string]any{
				"record_type": "run",
				"family":      family,
				"target":      payload.Target,
				"build_id":    payload.BuildID,
				"history_run": historyRun,
			},
		},
	}}

	for _, suite := range payload.Report.Suites {
		for _, c := range suite.Cases {
			if c.Passed && len(c.Findings) == 0 {
				continue
			}
			item := map[string]any{
				"id": braintrustEventID("case", payload.Target, payload.BuildID, suite.Name, c.Name),
				"input": map[string]any{
					"suite": suite.Name,
					"case":  c.Name,
				},
				"output": map[string]any{
					"passed":   c.Passed,
					"findings": c.Findings,
					"details":  c.Details,
				},
				"scores": map[string]any{
					"cleanr_passed": boolScore(c.Passed),
				},
				"metadata": map[string]any{
					"cleanr": map[string]any{
						"record_type": "case",
						"family":      family,
						"target":      payload.Target,
						"build_id":    payload.BuildID,
						"suite":       suite.Name,
						"case":        c.Name,
					},
				},
			}
			if c.Score > 0 {
				item["scores"].(map[string]any)["cleanr_score"] = c.Score
			}
			events = append(events, item)
		}
	}
	return events
}

func braintrustRunOutput(payload SinkPayload) map[string]any {
	out := map[string]any{
		"passed":          payload.Report.Passed,
		"failed_suites":   payload.Report.FailedSuites,
		"failed_cases":    payload.Report.FailedCases,
		"recommendations": payload.Report.Recommendations,
		"trend":           payload.Report.Trend,
		"trend_gate":      payload.Report.TrendGate,
	}
	if payload.ReplayArtifact != nil {
		out["replay_artifact"] = payload.ReplayArtifact
	}
	if payload.Attestation != nil {
		out["attestation"] = payload.Attestation
	}
	return out
}

func braintrustEventID(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	sum := sha1.Sum([]byte(strings.Join(filtered, "|")))
	return hex.EncodeToString(sum[:])
}

func boolScore(ok bool) float64 {
	if ok {
		return 1
	}
	return 0
}

func (c *braintrustClient) createProject(ctx context.Context, name string) (braintrustProject, error) {
	var project braintrustProject
	if err := c.http.postJSON(ctx, "/v1/project", map[string]any{"name": name}, &project); err != nil {
		return braintrustProject{}, err
	}
	return project, nil
}

func (c *braintrustClient) listExperiments(ctx context.Context, apiKeyEnv, project, family string, limit int) ([]braintrustExperiment, error) {
	query := url.Values{}
	query.Set("project_name", project)
	query.Set("limit", fmt.Sprintf("%d", limit))
	if family != "" {
		rawFilter, err := json.Marshal(map[string]any{
			"cleanr": map[string]any{
				"family": family,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("load trend source %s: %w", displayName(project, "braintrust"), err)
		}
		query.Set("metadata", string(rawFilter))
	}
	var resp braintrustExperimentList
	if err := c.http.getJSON(ctx, "/v1/experiment", query, &resp); err != nil {
		return nil, fmt.Errorf("load trend source %s: %w", displayName(project, "braintrust"), err)
	}
	return resp.Objects, nil
}

func (c *braintrustClient) fetchHistoryRun(ctx context.Context, apiKeyEnv, experimentID string) (trendspkg.HistoryRun, error) {
	query := fmt.Sprintf(
		"SELECT metadata.cleanr.history_run AS history_run FROM experiment('%s') WHERE metadata.cleanr.record_type = 'run' LIMIT 1",
		experimentID,
	)
	var resp braintrustBTQLResponse
	if err := c.http.postJSON(ctx, "/btql", map[string]any{
		"query": query,
		"fmt":   "json",
	}, &resp); err != nil {
		return trendspkg.HistoryRun{}, fmt.Errorf("load trend source braintrust: %w", err)
	}
	if len(resp.Data) == 0 {
		return trendspkg.HistoryRun{}, fmt.Errorf("load trend source braintrust: remote experiment %s did not include a cleanr history row", experimentID)
	}
	raw, err := json.Marshal(resp.Data[0]["history_run"])
	if err != nil {
		return trendspkg.HistoryRun{}, fmt.Errorf("load trend source braintrust: %w", err)
	}
	var run trendspkg.HistoryRun
	if err := json.Unmarshal(raw, &run); err != nil {
		return trendspkg.HistoryRun{}, fmt.Errorf("load trend source braintrust: %w", err)
	}
	return run, nil
}
