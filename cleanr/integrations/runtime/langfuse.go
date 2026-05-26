package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

const defaultLangfuseBaseURL = "https://cloud.langfuse.com"

type langfuseClient struct {
	http *jsonAPIClient
}

type langfuseScore struct {
	Name     string         `json:"name"`
	Value    float64        `json:"value"`
	DataType string         `json:"dataType,omitempty"`
	Comment  string         `json:"comment,omitempty"`
	TraceID  string         `json:"traceId"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func useNativeLangfuseSink(sink core.ResultSinkConfig) bool {
	return strings.TrimSpace(sink.Type) == "langfuse"
}

func newLangfuseClient(sink core.ResultSinkConfig) (*langfuseClient, error) {
	publicKey := strings.TrimSpace(envValue(sink.PublicKeyEnv))
	secretKey := strings.TrimSpace(envValue(sink.SecretKeyEnv))
	if publicKey == "" || secretKey == "" {
		return nil, fmt.Errorf("missing Langfuse credentials in %s or %s", emptyValue(sink.PublicKeyEnv), emptyValue(sink.SecretKeyEnv))
	}
	return &langfuseClient{
		http: newJSONAPIClient(
			normalizedBaseURL(sink.BaseURL, sink.Endpoint, defaultLangfuseBaseURL),
			sink.Headers,
			sink.TimeoutMS,
			func(h http.Header) {
				h.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(publicKey+":"+secretKey)))
			},
		),
	}, nil
}

func postLangfuseSinkPayload(ctx context.Context, sink core.ResultSinkConfig, payload SinkPayload) (string, error) {
	client, err := newLangfuseClient(sink)
	if err != nil {
		return "", fmt.Errorf("publish result sink %s: %w", displayName(sink.Name, sink.Type), err)
	}

	family := integrationFamily(sink.Experiment)
	traceID := langfuseTraceID(payload.Target, family, payload.BuildID, payload.GeneratedAt)
	traceEvent := map[string]any{
		"batch": []map[string]any{{
			"id":        langfuseEventID("trace", traceID),
			"type":      "trace-create",
			"timestamp": payload.GeneratedAt.UTC().Format(time.RFC3339Nano),
			"body": map[string]any{
				"id":        traceID,
				"name":      family,
				"sessionId": strings.TrimSpace(payload.BuildID),
				"input": map[string]any{
					"kind":         "cleanr-release-gate",
					"target":       payload.Target,
					"build_id":     payload.BuildID,
					"generated_at": payload.GeneratedAt,
				},
				"output": map[string]any{
					"passed":          payload.Report.Passed,
					"failed_suites":   payload.Report.FailedSuites,
					"failed_cases":    payload.Report.FailedCases,
					"trend":           payload.Report.Trend,
					"trend_gate":      payload.Report.TrendGate,
					"recommendations": payload.Report.Recommendations,
					"failures":        failureSummary(payload.Report),
				},
				"metadata": buildLangfuseTraceMetadata(payload, family),
				"tags":     buildLangfuseTags(payload.Target, family),
			},
		}},
	}
	if err := client.http.postJSON(ctx, "/api/public/ingestion", traceEvent, nil); err != nil {
		return "", fmt.Errorf("publish result sink %s: %w", displayName(sink.Name, sink.Type), err)
	}

	for _, score := range buildLangfuseScores(traceID, payload) {
		if err := client.http.postJSON(ctx, "/api/public/scores", score, nil); err != nil {
			return "", fmt.Errorf("publish result sink %s: %w", displayName(sink.Name, sink.Type), err)
		}
	}

	return expandRunURLWithValues(sink.RunURLTemplate, payload, map[string]string{"trace_id": traceID}), nil
}

func buildLangfuseTraceMetadata(payload SinkPayload, family string) map[string]any {
	metadata := map[string]any{
		"source":             payload.Source,
		"family":             family,
		"target":             payload.Target,
		"build_id":           payload.BuildID,
		"generated_at":       payload.GeneratedAt,
		"local_blocking":     payload.LocalBlocking,
		"remote_best_effort": payload.RemoteBestEffort,
		"report":             payload.Report,
	}
	if payload.ReplayArtifact != nil {
		metadata["replay_artifact"] = payload.ReplayArtifact
	}
	if payload.Attestation != nil {
		metadata["attestation"] = payload.Attestation
	}
	return map[string]any{
		"cleanr": metadata,
	}
}

func buildLangfuseTags(target, family string) []string {
	tags := []string{"cleanr", target}
	if family != "" {
		tags = append(tags, family)
	}
	return tags
}

func buildLangfuseScores(traceID string, payload SinkPayload) []langfuseScore {
	scores := []langfuseScore{
		{
			Name:     "cleanr_passed",
			Value:    boolScore(payload.Report.Passed),
			DataType: "NUMERIC",
			Comment:  "1 means the local cleanr gate passed, 0 means it failed.",
			TraceID:  traceID,
		},
		{
			Name:     "failed_suites",
			Value:    float64(payload.Report.FailedSuites),
			DataType: "NUMERIC",
			TraceID:  traceID,
		},
		{
			Name:     "failed_cases",
			Value:    float64(payload.Report.FailedCases),
			DataType: "NUMERIC",
			TraceID:  traceID,
		},
	}
	if payload.Report.Trend != nil {
		scores = append(scores, langfuseScore{
			Name:     "regressed_suites",
			Value:    float64(payload.Report.Trend.Summary.RegressedSuites),
			DataType: "NUMERIC",
			TraceID:  traceID,
		})
	}
	if payload.Report.TrendGate != nil && payload.Report.TrendGate.Evaluated {
		scores = append(scores, langfuseScore{
			Name:     "trend_gate_passed",
			Value:    boolScore(payload.Report.TrendGate.Passed),
			DataType: "NUMERIC",
			TraceID:  traceID,
		})
	}
	return scores
}

func langfuseTraceID(target, family, buildID string, generatedAt time.Time) string {
	seed := strings.Join([]string{
		strings.TrimSpace(target),
		strings.TrimSpace(family),
		runScopeSuffix(buildID, generatedAt),
	}, "|")
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:16])
}

func langfuseEventID(parts ...string) string {
	seed := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:16])
}
