package integrations

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"cleanr/cleanr/core"
)

const defaultPostHogBaseURL = "https://us.i.posthog.com"

type postHogClient struct {
	token string
	http  *jsonAPIClient
}

func useNativePostHogSink(sink core.ResultSinkConfig) bool {
	return strings.TrimSpace(sink.Type) == "posthog"
}

func newPostHogClient(sink core.ResultSinkConfig) (*postHogClient, error) {
	token := strings.TrimSpace(envValue(sink.ProjectTokenEnv))
	if token == "" {
		return nil, fmt.Errorf("missing PostHog project token in %s", emptyValue(sink.ProjectTokenEnv))
	}
	return &postHogClient{
		token: token,
		http: newJSONAPIClient(
			strings.TrimRight(postHogBaseURL(sink.BaseURL, sink.Endpoint), "/"),
			sink.Headers,
			sink.TimeoutMS,
			nil,
		),
	}, nil
}

func postPostHogSinkPayload(ctx context.Context, sink core.ResultSinkConfig, payload SinkPayload) (string, error) {
	client, err := newPostHogClient(sink)
	if err != nil {
		return "", fmt.Errorf("publish result sink %s: %w", displayName(sink.Name, sink.Type), err)
	}

	family := postHogFamilyName(sink.Experiment)
	distinctID := postHogDistinctID(payload.Target, family, payload.BuildID, payload.GeneratedAt)
	body := map[string]any{
		"api_key": client.token,
		"batch":   buildPostHogEvents(payload, family, distinctID),
	}
	if err := client.http.postJSON(ctx, "/batch/", body, nil); err != nil {
		return "", fmt.Errorf("publish result sink %s: %w", displayName(sink.Name, sink.Type), err)
	}
	return expandPostHogRunURL(sink.RunURLTemplate, payload, distinctID), nil
}

func buildPostHogEvents(payload SinkPayload, family, distinctID string) []map[string]any {
	events := []map[string]any{
		{
			"event":       "cleanr_run",
			"distinct_id": distinctID,
			"timestamp":   payload.GeneratedAt.UTC().Format(time.RFC3339Nano),
			"properties":  buildPostHogRunProperties(payload, family, distinctID),
		},
	}
	for _, suite := range payload.Report.Suites {
		for _, c := range suite.Cases {
			if c.Passed && len(c.Findings) == 0 {
				continue
			}
			events = append(events, map[string]any{
				"event":       "cleanr_case_result",
				"distinct_id": distinctID,
				"timestamp":   payload.GeneratedAt.UTC().Format(time.RFC3339Nano),
				"properties": map[string]any{
					"distinct_id":     distinctID,
					"cleanr_target":   payload.Target,
					"cleanr_family":   family,
					"cleanr_build_id": payload.BuildID,
					"cleanr_suite":    suite.Name,
					"cleanr_case":     c.Name,
					"cleanr_passed":   c.Passed,
					"cleanr_score":    c.Score,
					"cleanr_findings": c.Findings,
					"cleanr_details":  c.Details,
				},
			})
		}
	}
	return events
}

func buildPostHogRunProperties(payload SinkPayload, family, distinctID string) map[string]any {
	properties := map[string]any{
		"distinct_id":               distinctID,
		"cleanr_source":             payload.Source,
		"cleanr_target":             payload.Target,
		"cleanr_family":             family,
		"cleanr_build_id":           payload.BuildID,
		"cleanr_passed":             payload.Report.Passed,
		"cleanr_failed_suites":      payload.Report.FailedSuites,
		"cleanr_failed_cases":       payload.Report.FailedCases,
		"cleanr_generated_at":       payload.GeneratedAt,
		"cleanr_local_blocking":     payload.LocalBlocking,
		"cleanr_remote_best_effort": payload.RemoteBestEffort,
		"cleanr_recommendations":    payload.Report.Recommendations,
		"cleanr_report":             payload.Report,
	}
	if payload.ReplayArtifact != nil {
		properties["cleanr_replay_artifact"] = payload.ReplayArtifact
	}
	if payload.Attestation != nil {
		properties["cleanr_attestation"] = payload.Attestation
	}
	return properties
}

func postHogFamilyName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "cleanr-release-gate"
	}
	return name
}

func postHogDistinctID(target, family, buildID string, generatedAt time.Time) string {
	buildID = strings.TrimSpace(buildID)
	if buildID != "" {
		return strings.Join([]string{"cleanr", target, family, buildID}, ":")
	}
	return strings.Join([]string{"cleanr", target, family, generatedAt.UTC().Format("20060102T150405Z")}, ":")
}

func expandPostHogRunURL(tmpl string, payload SinkPayload, distinctID string) string {
	tmpl = strings.TrimSpace(tmpl)
	if tmpl == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"{{project}}", payload.Project,
		"{{experiment}}", payload.Experiment,
		"{{build_id}}", payload.BuildID,
		"{{target}}", payload.Target,
		"{{distinct_id}}", distinctID,
	)
	return replacer.Replace(tmpl)
}

func postHogBaseURL(baseURL, endpoint string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL != "" {
		return baseURL
	}
	if raw := strings.TrimSpace(endpoint); raw != "" {
		if parsed, err := url.Parse(raw); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			return parsed.Scheme + "://" + parsed.Host
		}
	}
	return defaultPostHogBaseURL
}
