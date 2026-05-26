package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/cli"
	"github.com/devr-tools/cleanr/internal/testutil"
)

func TestOpenAITargetParsesResponsesAPIUsage(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://openai.test/v1/responses" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}
			if req.Header.Get("Authorization") != "Bearer test-key" {
				t.Fatalf("unexpected auth header: %s", req.Header.Get("Authorization"))
			}

			body := decodeRequestBody(t, req)
			if err := assertResponsesRequest(body, "You are a security reviewer.", "Give one short password-hardening recommendation."); err != nil {
				t.Fatalf("unexpected request: %v", err)
			}

			return jsonResponse(t, http.StatusOK, map[string]any{
				"id":     "resp_test",
				"object": "response",
				"model":  "gpt-4.1-mini",
				"status": "completed",
				"output": []any{
					map[string]any{
						"type": "message",
						"role": "assistant",
						"content": []any{
							map[string]any{
								"type": "output_text",
								"text": "Require MFA on every admin account.",
							},
						},
					},
				},
				"usage": map[string]any{
					"input_tokens":  21,
					"output_tokens": 8,
					"total_tokens":  29,
				},
			}), nil
		}),
	}

	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			APIMode:   "responses",
			Model:     "gpt-4.1-mini",
			APIKeyEnv: "OPENAI_API_KEY",
			BaseURL:   "https://openai.test/v1",
		},
	}, client)

	resp := target.Invoke(context.Background(), cleanr.Request{
		Scenario: cleanr.Scenario{
			Name:   "openai-responses",
			System: "You are a security reviewer.",
			Input:  "Give one short password-hardening recommendation.",
		},
		System:  "You are a security reviewer.",
		Prompt:  "Give one short password-hardening recommendation.",
		Timeout: 2 * time.Second,
	})

	if resp.Err != nil || resp.ExtractError != nil {
		t.Fatalf("unexpected response errors: err=%v extract=%v", resp.Err, resp.ExtractError)
	}
	if resp.Text != "Require MFA on every admin account." {
		t.Fatalf("unexpected text: %q", resp.Text)
	}
	if resp.Usage.InputTokens != 21 || resp.Usage.OutputTokens != 8 || resp.Usage.TotalTokens != 29 || resp.Usage.Heuristic {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
	if resp.Normalized.Provider != "openai" || resp.Normalized.ID != "resp_test" || resp.Normalized.Model != "gpt-4.1-mini" || resp.Normalized.Status != "completed" {
		t.Fatalf("unexpected normalized response: %+v", resp.Normalized)
	}
}

func TestOpenAITargetNormalizesChatCompletionToolCalls(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(t, http.StatusOK, map[string]any{
				"id":     "chatcmpl_tool",
				"object": "chat.completion",
				"model":  "gpt-4o-mini",
				"choices": []any{
					map[string]any{
						"index": 0,
						"message": map[string]any{
							"role":    "assistant",
							"content": nil,
							"tool_calls": []any{
								map[string]any{
									"id":   "call_123",
									"type": "function",
									"function": map[string]any{
										"name":      "lookup_policy",
										"arguments": "{\"policy_id\":\"refunds\"}",
									},
								},
							},
						},
						"finish_reason": "tool_calls",
					},
				},
				"usage": map[string]any{
					"prompt_tokens":     18,
					"completion_tokens": 4,
					"total_tokens":      22,
				},
			}), nil
		}),
	}

	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			APIMode:   "chat_completions",
			Model:     "gpt-4o-mini",
			APIKeyEnv: "OPENAI_API_KEY",
			BaseURL:   "https://openai.test/v1",
		},
	}, client)

	resp := target.Invoke(context.Background(), cleanr.Request{
		Prompt:  "Use tools when needed.",
		Timeout: 2 * time.Second,
	})

	if resp.Err != nil || resp.ExtractError != nil {
		t.Fatalf("unexpected response errors: err=%v extract=%v", resp.Err, resp.ExtractError)
	}
	if len(resp.Normalized.ToolCalls) != 1 {
		t.Fatalf("unexpected normalized tool calls: %+v", resp.Normalized.ToolCalls)
	}
	toolCall := resp.Normalized.ToolCalls[0]
	if resp.Normalized.FinishReason != "tool_calls" || toolCall.Name != "lookup_policy" || toolCall.Arguments != "{\"policy_id\":\"refunds\"}" {
		t.Fatalf("unexpected normalized tool call payload: normalized=%+v tool=%+v", resp.Normalized, toolCall)
	}
}

func TestRunCommandSupportsOpenAIChatCompletionsTarget(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	restoreTransport := stubDefaultTransport(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://openai.test/v1/chat/completions" {
			t.Fatalf("unexpected url: %s", req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", req.Header.Get("Authorization"))
		}

		body := decodeRequestBody(t, req)
		if err := assertChatCompletionsRequest(body, "You are a concise support assistant.", "Summarize the refund policy in one sentence."); err != nil {
			t.Fatalf("unexpected request: %v", err)
		}

		return jsonResponse(t, http.StatusOK, map[string]any{
			"id":     "chatcmpl_test",
			"object": "chat.completion",
			"model":  "gpt-4o-mini",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Refunds are available within 30 days of purchase.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     24,
				"completion_tokens": 11,
				"total_tokens":      35,
			},
		}), nil
	}))
	defer restoreTransport()

	configPath := testutil.WriteNamedConfigFile(t, "openai-chat.json", marshalOpenAIConfig(map[string]any{
		"version": "v1alpha1",
		"target": map[string]any{
			"type": "openai",
			"name": "openai-chat",
			"openai": map[string]any{
				"api_mode":    "chat_completions",
				"model":       "gpt-4o-mini",
				"api_key_env": "OPENAI_API_KEY",
				"base_url":    "https://openai.test/v1",
			},
		},
		"scenarios": []any{
			map[string]any{
				"name":              "happy-path",
				"system":            "You are a concise support assistant.",
				"input":             "Summarize the refund policy in one sentence.",
				"expected_contains": []string{"30 days"},
			},
		},
		"suites": map[string]any{
			"security": map[string]any{
				"enabled":                    true,
				"max_pii_matches":            0,
				"dangerous_tool_indicators":  []string{},
				"secret_exposure_indicators": []string{},
			},
		},
		"reporting": map[string]any{"format": "json"},
	}))

	report := runConfigAsJSONReport(t, configPath)
	requirePassingSecurityReport(t, report, "openai-chat")
	details := report.Suites[0].Cases[0].Details
	if details["provider"] != "openai" || details["provider_model"] != "gpt-4o-mini" || details["finish_reason"] != "stop" {
		t.Fatalf("unexpected normalized details: %+v", details)
	}
}

func TestRunCommandSupportsOpenAIResponsesTarget(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	restoreTransport := stubDefaultTransport(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://openai.test/v1/responses" {
			t.Fatalf("unexpected url: %s", req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", req.Header.Get("Authorization"))
		}

		body := decodeRequestBody(t, req)
		if err := assertResponsesRequest(body, "You are a security reviewer.", "Give one short password-hardening recommendation."); err != nil {
			t.Fatalf("unexpected request: %v", err)
		}

		return jsonResponse(t, http.StatusOK, map[string]any{
			"id":     "resp_test",
			"object": "response",
			"model":  "gpt-4.1-mini",
			"status": "completed",
			"output": []any{
				map[string]any{
					"type": "message",
					"role": "assistant",
					"content": []any{
						map[string]any{
							"type": "output_text",
							"text": "Require MFA on every admin account.",
						},
					},
				},
			},
			"usage": map[string]any{
				"input_tokens":  21,
				"output_tokens": 8,
				"total_tokens":  29,
			},
		}), nil
	}))
	defer restoreTransport()

	configPath := testutil.WriteNamedConfigFile(t, "openai-responses.json", marshalOpenAIConfig(map[string]any{
		"version": "v1alpha1",
		"target": map[string]any{
			"type": "openai",
			"name": "openai-responses",
			"openai": map[string]any{
				"api_mode":    "responses",
				"model":       "gpt-4.1-mini",
				"api_key_env": "OPENAI_API_KEY",
				"base_url":    "https://openai.test/v1",
			},
		},
		"scenarios": []any{
			map[string]any{
				"name":              "happy-path",
				"system":            "You are a security reviewer.",
				"input":             "Give one short password-hardening recommendation.",
				"expected_contains": []string{"MFA"},
			},
		},
		"suites": map[string]any{
			"security": map[string]any{
				"enabled":                    true,
				"max_pii_matches":            0,
				"dangerous_tool_indicators":  []string{},
				"secret_exposure_indicators": []string{},
			},
		},
		"reporting": map[string]any{"format": "json"},
	}))

	report := runConfigAsJSONReport(t, configPath)
	requirePassingSecurityReport(t, report, "openai-responses")
}

func TestValidateCommandAcceptsOpenAIConfig(t *testing.T) {
	configPath := testutil.WriteNamedConfigFile(t, "openai-validate.json", marshalOpenAIConfig(map[string]any{
		"version": "v1alpha1",
		"target": map[string]any{
			"type": "openai",
			"name": "openai-validate",
			"openai": map[string]any{
				"api_mode": "responses",
				"model":    "gpt-4.1-mini",
			},
		},
		"scenarios": []any{
			map[string]any{
				"name":  "happy-path",
				"input": "hello",
			},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"validate", "-config", configPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected validate to pass, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "valid config for openai-validate with 1 scenarios") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func marshalOpenAIConfig(cfg map[string]any) string {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(data)
}
