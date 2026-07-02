package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
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
	baseURL := normalizedBaseURL(sink.BaseURL, sink.Endpoint, defaultPostHogBaseURL)
	// The token travels in the request body, so it bypasses applyAuth; enforce
	// the same egress policy here so a config cannot point a provider secret
	// (e.g. project_token_env: OPENAI_API_KEY) at an arbitrary host.
	if !CredentialEgressAllowed(sink.ProjectTokenEnv, baseURL) {
		return nil, fmt.Errorf("refusing to send credential %q to untrusted host %q", strings.TrimSpace(sink.ProjectTokenEnv), destinationHost(baseURL))
	}
	return &postHogClient{
		token: token,
		http: newJSONAPIClient(
			baseURL,
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

	family := integrationFamily(sink.Experiment)
	distinctID := postHogDistinctID(payload.Target, family, payload.BuildID, payload.GeneratedAt)
	body := map[string]any{
		"api_key": client.token,
		"batch":   buildPostHogEvents(payload, family, distinctID),
	}
	if err := client.http.postJSON(ctx, "/batch/", body, nil); err != nil {
		return "", fmt.Errorf("publish result sink %s: %w", displayName(sink.Name, sink.Type), err)
	}
	return expandRunURLWithValues(sink.RunURLTemplate, payload, map[string]string{"distinct_id": distinctID}), nil
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

func postHogDistinctID(target, family, buildID string, generatedAt time.Time) string {
	return strings.Join([]string{"cleanr", target, family, runScopeSuffix(buildID, generatedAt)}, ":")
}
