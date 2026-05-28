package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

type braintrustRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f braintrustRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func stubBraintrustTransport(t *testing.T, transport http.RoundTripper) func() {
	t.Helper()
	original := http.DefaultTransport
	http.DefaultTransport = transport
	return func() {
		http.DefaultTransport = original
	}
}

func jsonBraintrustResponse(t *testing.T, statusCode int, body map[string]any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

func decodeBraintrustRequestBody(t *testing.T, req *http.Request) map[string]any {
	t.Helper()
	defer req.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return body
}

func TestApplyBraintrustConfigPatchSetSupportsSelectorsAndAppendUnique(t *testing.T) {
	base := cleanr.ExampleConfig()
	base.Scenarios = []cleanr.Scenario{{
		Name:   "happy-path",
		System: "Original system",
		Tags:   []string{"existing"},
		Input:  "hello",
	}}
	base.Suites.TokenOptimization.MaxOutputTokens = 512
	base.Suites.Security.SecretExposureIndicators = []string{"sk-"}

	patched, err := cleanr.ApplyBraintrustInsightDataset(base, cleanr.BraintrustInsightDataset{
		ConfigPatch: &cleanr.BraintrustConfigPatchSet{
			Operations: []cleanr.BraintrustConfigPatchOperation{
				{
					Op:    "set",
					Path:  "suites.token_optimization.max_output_tokens",
					Value: 256,
				},
				{
					Op:    "set",
					Path:  "scenarios[name=happy-path].system",
					Value: "Use the verified password reset flow.",
				},
				{
					Op:    "append_unique",
					Path:  "scenarios[name=happy-path].tags",
					Value: []any{"regression", "existing"},
				},
				{
					Op:    "append_unique",
					Path:  "suites.security.secret_exposure_indicators",
					Value: []string{"AKIA", "sk-"},
				},
			},
		},
	}, false, true, true)
	if err != nil {
		t.Fatalf("apply patch: %v", err)
	}

	if patched.Suites.TokenOptimization.MaxOutputTokens != 256 {
		t.Fatalf("unexpected token threshold: %+v", patched.Suites.TokenOptimization)
	}
	if len(patched.Scenarios) != 1 || patched.Scenarios[0].System != "Use the verified password reset flow." {
		t.Fatalf("unexpected scenarios: %+v", patched.Scenarios)
	}
	if got := strings.Join(patched.Scenarios[0].Tags, ","); got != "existing,regression" {
		t.Fatalf("unexpected scenario tags: %s", got)
	}
	if got := strings.Join(patched.Suites.Security.SecretExposureIndicators, ","); got != "sk-,AKIA" {
		t.Fatalf("unexpected security indicators: %s", got)
	}
}

func TestApplyBraintrustConfigPatchSetRejectsInvalidSelectorPaths(t *testing.T) {
	base := cleanr.ExampleConfig()
	base.Scenarios = []cleanr.Scenario{{Name: "happy-path", Input: "hello"}}

	_, err := cleanr.ApplyBraintrustInsightDataset(base, cleanr.BraintrustInsightDataset{
		ConfigPatch: &cleanr.BraintrustConfigPatchSet{
			Operations: []cleanr.BraintrustConfigPatchOperation{{
				Op:    "set",
				Path:  "scenarios[name=missing].system",
				Value: "patched",
			}},
		},
	}, false, true, true)
	if err == nil || !strings.Contains(err.Error(), "no list item in scenarios matched name=missing") {
		t.Fatalf("expected selector failure, got %v", err)
	}

	_, err = cleanr.ApplyBraintrustInsightDataset(base, cleanr.BraintrustInsightDataset{
		ConfigPatch: &cleanr.BraintrustConfigPatchSet{
			Operations: []cleanr.BraintrustConfigPatchOperation{{
				Op:    "append_unique",
				Path:  "suites.token_optimization.max_output_tokens",
				Value: []string{"bad"},
			}},
		},
	}, false, true, true)
	if err == nil || !strings.Contains(err.Error(), "expected a string or string list") {
		t.Fatalf("expected type failure, got %v", err)
	}
}

func TestFetchBraintrustInsightDatasetMergesReplayAndRemotePatch(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{Name: "happy-path", Input: "base input"}}

	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	restore := stubBraintrustTransport(t, braintrustRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment":
			return jsonBraintrustResponse(t, 200, map[string]any{
				"objects": []map[string]any{{
					"id":         "exp-2",
					"project_id": "proj-1",
					"name":       "cleanr-ci/build-2",
					"created":    now.Format(time.RFC3339),
				}},
			}), nil
		case req.Method == http.MethodPost && req.URL.Path == "/btql":
			body := decodeBraintrustRequestBody(t, req)
			query := body["query"].(string)
			switch {
			case strings.Contains(query, "replay_artifact"):
				return jsonBraintrustResponse(t, 200, map[string]any{
					"data": []map[string]any{{
						"replay_artifact": map[string]any{
							"version":      "v1alpha1",
							"target":       cfg.Target.Name,
							"build_id":     "build-2",
							"generated_at": now.Format(time.RFC3339),
							"passed":       false,
							"failed_cases": 1,
							"failures": []map[string]any{{
								"suite":  "security",
								"name":   "happy-path",
								"failed": true,
								"findings": []map[string]any{{
									"severity": "high",
									"message":  "review me",
								}},
							}},
						},
					}},
				}), nil
			case strings.Contains(query, "cleanr_sync"):
				return jsonBraintrustResponse(t, 200, map[string]any{
					"data": []map[string]any{{
						"cleanr_sync": map[string]any{
							"version":         "v1alpha1",
							"review_required": true,
							"warnings":        []string{"remote warning"},
							"scenario_dataset": map[string]any{
								"review_required": true,
								"scenarios": []map[string]any{
									{"scenario": map[string]any{"name": "happy-path", "input": "updated input"}},
									{"scenario": map[string]any{"name": "new-regression", "input": "new"}},
								},
							},
							"config_patch": map[string]any{
								"review_required": true,
								"operations": []map[string]any{{
									"op":    "set",
									"path":  "suites.token_optimization.max_output_tokens",
									"value": 128,
								}},
							},
						},
					}},
				}), nil
			default:
				t.Fatalf("unexpected btql query: %s", query)
				return nil, nil
			}
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment/exp-2/summarize":
			return jsonBraintrustResponse(t, 200, map[string]any{
				"experiment_url": "https://braintrust.dev/app/cleanr-ci/build-2",
			}), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))
	defer restore()

	dataset, err := cleanr.FetchBraintrustInsightDataset(context.Background(), cleanr.TrendSourceConfig{
		Type:       "braintrust",
		Project:    "qa-gates",
		Experiment: "cleanr-ci",
	}, cfg)
	if err != nil {
		t.Fatalf("fetch braintrust insight dataset: %v", err)
	}
	if !dataset.ReviewRequired || dataset.BuildID != "build-2" || dataset.ExperimentURL == "" {
		t.Fatalf("unexpected merged dataset metadata: %+v", dataset)
	}
	if dataset.ScenarioDataset == nil || !dataset.ScenarioDataset.ReviewRequired || len(dataset.ScenarioDataset.Scenarios) != 2 {
		t.Fatalf("unexpected merged scenario dataset: %+v", dataset.ScenarioDataset)
	}
	if got := dataset.ScenarioDataset.Scenarios[0].Scenario.Input; got != "updated input" {
		t.Fatalf("expected remote scenario overwrite, got %q", got)
	}
	if dataset.ConfigPatch == nil || !dataset.ConfigPatch.ReviewRequired || len(dataset.ConfigPatch.Operations) != 1 {
		t.Fatalf("unexpected merged config patch: %+v", dataset.ConfigPatch)
	}
	if got := strings.Join(dataset.Warnings, ","); got != "remote warning" {
		t.Fatalf("unexpected warnings: %s", got)
	}
}

func TestFetchBraintrustInsightDatasetSkipsNewestExperimentWithoutReplayOrInsight(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	restore := stubBraintrustTransport(t, braintrustRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment":
			return jsonBraintrustResponse(t, 200, map[string]any{
				"objects": []map[string]any{
					{"id": "exp-new", "project_id": "proj-1", "name": "cleanr-ci/build-new", "created": "2026-05-28T13:00:00Z"},
					{"id": "exp-old", "project_id": "proj-1", "name": "cleanr-ci/build-old", "created": "2026-05-27T13:00:00Z"},
				},
			}), nil
		case req.Method == http.MethodPost && req.URL.Path == "/btql":
			body := decodeBraintrustRequestBody(t, req)
			query := body["query"].(string)
			switch {
			case strings.Contains(query, "experiment('exp-new')") && strings.Contains(query, "replay_artifact"):
				return jsonBraintrustResponse(t, 200, map[string]any{"data": []map[string]any{}}), nil
			case strings.Contains(query, "experiment('exp-new')") && strings.Contains(query, "cleanr_sync"):
				return jsonBraintrustResponse(t, 200, map[string]any{"data": []map[string]any{}}), nil
			case strings.Contains(query, "experiment('exp-old')") && strings.Contains(query, "replay_artifact"):
				return jsonBraintrustResponse(t, 200, map[string]any{
					"data": []map[string]any{{
						"replay_artifact": map[string]any{
							"version":      "v1alpha1",
							"target":       cfg.Target.Name,
							"build_id":     "build-old",
							"generated_at": "2026-05-27T13:00:00Z",
							"passed":       false,
							"failed_cases": 1,
						},
					}},
				}), nil
			case strings.Contains(query, "experiment('exp-old')") && strings.Contains(query, "cleanr_sync"):
				return jsonBraintrustResponse(t, 200, map[string]any{"data": []map[string]any{}}), nil
			default:
				t.Fatalf("unexpected btql query: %s", query)
				return nil, nil
			}
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment/exp-old/summarize":
			return jsonBraintrustResponse(t, 200, map[string]any{
				"experiment_url": "https://braintrust.dev/app/cleanr-ci/build-old",
			}), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))
	defer restore()

	dataset, err := cleanr.FetchBraintrustInsightDataset(context.Background(), cleanr.TrendSourceConfig{
		Type:       "braintrust",
		Project:    "qa-gates",
		Experiment: "cleanr-ci",
	}, cfg)
	if err != nil {
		t.Fatalf("fetch braintrust insight dataset: %v", err)
	}
	if dataset.BuildID != "build-old" || dataset.ExperimentID != "exp-old" {
		t.Fatalf("unexpected dataset selection: %+v", dataset)
	}
}

func TestFetchBraintrustInsightDatasetAllowsMissingExperimentURL(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	restore := stubBraintrustTransport(t, braintrustRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment":
			return jsonBraintrustResponse(t, 200, map[string]any{
				"objects": []map[string]any{{
					"id":         "exp-1",
					"project_id": "proj-1",
					"name":       "cleanr-ci/build-1",
					"created":    "2026-05-28T12:00:00Z",
				}},
			}), nil
		case req.Method == http.MethodPost && req.URL.Path == "/btql":
			body := decodeBraintrustRequestBody(t, req)
			query := body["query"].(string)
			if strings.Contains(query, "replay_artifact") {
				return jsonBraintrustResponse(t, 200, map[string]any{"data": []map[string]any{}}), nil
			}
			return jsonBraintrustResponse(t, 200, map[string]any{
				"data": []map[string]any{{
					"cleanr_sync": map[string]any{
						"version": "v1alpha1",
						"config_patch": map[string]any{
							"operations": []map[string]any{{
								"op":    "set",
								"path":  "suites.token_optimization.max_output_tokens",
								"value": 128,
							}},
						},
					},
				}},
			}), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment/exp-1/summarize":
			return jsonBraintrustResponse(t, 200, map[string]any{}), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))
	defer restore()

	dataset, err := cleanr.FetchBraintrustInsightDataset(context.Background(), cleanr.TrendSourceConfig{
		Type:       "braintrust",
		Project:    "qa-gates",
		Experiment: "cleanr-ci",
	}, cfg)
	if err != nil {
		t.Fatalf("fetch braintrust insight dataset: %v", err)
	}
	if dataset.ExperimentURL != "" || dataset.ConfigPatch == nil {
		t.Fatalf("unexpected dataset: %+v", dataset)
	}
}

func TestFetchBraintrustInsightDatasetRejectsMalformedCleanrSyncPayload(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	restore := stubBraintrustTransport(t, braintrustRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment":
			return jsonBraintrustResponse(t, 200, map[string]any{
				"objects": []map[string]any{{
					"id":         "exp-1",
					"project_id": "proj-1",
					"name":       "cleanr-ci/build-1",
					"created":    "2026-05-28T12:00:00Z",
				}},
			}), nil
		case req.Method == http.MethodPost && req.URL.Path == "/btql":
			body := decodeBraintrustRequestBody(t, req)
			query := body["query"].(string)
			if strings.Contains(query, "replay_artifact") {
				return jsonBraintrustResponse(t, 200, map[string]any{"data": []map[string]any{}}), nil
			}
			return jsonBraintrustResponse(t, 200, map[string]any{
				"data": []map[string]any{{
					"cleanr_sync": "bad-payload",
				}},
			}), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))
	defer restore()

	_, err := cleanr.FetchBraintrustInsightDataset(context.Background(), cleanr.TrendSourceConfig{
		Type:       "braintrust",
		Project:    "qa-gates",
		Experiment: "cleanr-ci",
	}, cfg)
	if err == nil || !strings.Contains(err.Error(), "load braintrust sync artifacts") {
		t.Fatalf("expected malformed payload error, got %v", err)
	}
}

func TestFetchBraintrustInsightDatasetErrorsWhenNoReplayOrInsightExists(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	restore := stubBraintrustTransport(t, braintrustRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/experiment":
			return jsonBraintrustResponse(t, 200, map[string]any{
				"objects": []map[string]any{{
					"id":         "exp-1",
					"project_id": "proj-1",
					"name":       "cleanr-ci/build-1",
					"created":    "2026-05-28T12:00:00Z",
				}},
			}), nil
		case req.Method == http.MethodPost && req.URL.Path == "/btql":
			return jsonBraintrustResponse(t, 200, map[string]any{"data": []map[string]any{}}), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))
	defer restore()

	_, err := cleanr.FetchBraintrustInsightDataset(context.Background(), cleanr.TrendSourceConfig{
		Type:       "braintrust",
		Project:    "qa-gates",
		Experiment: "cleanr-ci",
	}, cfg)
	if err == nil || !strings.Contains(err.Error(), "no replay artifact or sync insight found") {
		t.Fatalf("expected missing artifact error, got %v", err)
	}
}
