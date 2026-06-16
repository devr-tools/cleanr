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
	parsedArgs, ok := toolCall.ParsedArgs.(map[string]any)
	if !ok || parsedArgs["policy_id"] != "refunds" {
		t.Fatalf("unexpected parsed tool arguments: %+v", toolCall.ParsedArgs)
	}
}

func TestOpenAITargetParsesChatCompletionSSE(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			body := decodeRequestBody(t, req)
			if body["stream"] != true {
				t.Fatalf("expected stream request body, got %#v", body)
			}
			return sseResponse(http.StatusOK, strings.Join([]string{
				`data: {"id":"chatcmpl_stream","object":"chat.completion.chunk","model":"gpt-4o-mini","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello "}}]}`,
				"",
				`data: {"id":"chatcmpl_stream","object":"chat.completion.chunk","model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"world","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup_policy","arguments":"{\"policy_id\":\"refund"}}]}}]}`,
				"",
				`data: {"id":"chatcmpl_stream","object":"chat.completion.chunk","model":"gpt-4o-mini","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"s\"}"}}]}}]}`,
				"",
				`data: {"id":"chatcmpl_stream","object":"chat.completion.chunk","model":"gpt-4o-mini","choices":[],"usage":{"prompt_tokens":18,"completion_tokens":4,"total_tokens":22}}`,
				"",
				`data: {"broken":`,
				"",
				`data: [DONE]`,
				"",
			}, "\n")), nil
		}),
	}

	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type:   "openai",
		Stream: true,
		OpenAI: cleanr.OpenAIConfig{
			APIMode:   "chat_completions",
			Model:     "gpt-4o-mini",
			APIKeyEnv: "OPENAI_API_KEY",
			BaseURL:   "https://openai.test/v1",
		},
	}, client)

	resp := target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: 2 * time.Second})
	if resp.Err != nil {
		t.Fatalf("unexpected response error: %v", resp.Err)
	}
	if resp.Text != "Hello world" {
		t.Fatalf("unexpected stream text: %q", resp.Text)
	}
	if resp.Stream.CompletionState != "completed" || resp.Stream.ErrorCount < 1 || !resp.Stream.Recovered {
		t.Fatalf("unexpected stream metrics: %+v", resp.Stream)
	}
	if resp.Usage.TotalTokens != 22 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
	if len(resp.Normalized.ToolCalls) != 1 || resp.Normalized.ToolCalls[0].Arguments != "{\"policy_id\":\"refunds\"}" {
		t.Fatalf("unexpected tool calls: %+v", resp.Normalized.ToolCalls)
	}
}

func TestOpenAITargetParsesResponsesSSE(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			body := decodeRequestBody(t, req)
			if body["stream"] != true {
				t.Fatalf("expected stream request body, got %#v", body)
			}
			return sseResponse(http.StatusOK, strings.Join([]string{
				"event: response.output_text.delta",
				`data: {"delta":"Require MFA "}`,
				"",
				"event: response.output_text.delta",
				`data: {"delta":"everywhere."}`,
				"",
				"event: response.completed",
				`data: {"id":"resp_stream","model":"gpt-4.1-mini","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Require MFA everywhere."}]}],"usage":{"input_tokens":21,"output_tokens":8,"total_tokens":29}}`,
				"",
			}, "\n")), nil
		}),
	}

	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type:   "openai",
		Stream: true,
		OpenAI: cleanr.OpenAIConfig{
			APIMode:   "responses",
			Model:     "gpt-4.1-mini",
			APIKeyEnv: "OPENAI_API_KEY",
			BaseURL:   "https://openai.test/v1",
		},
	}, client)

	resp := target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: 2 * time.Second})
	if resp.Err != nil {
		t.Fatalf("unexpected response error: %v", resp.Err)
	}
	if resp.Text != "Require MFA everywhere." {
		t.Fatalf("unexpected stream text: %q", resp.Text)
	}
	if resp.Stream.CompletionState != "completed" {
		t.Fatalf("unexpected stream metrics: %+v", resp.Stream)
	}
	if resp.Normalized.Status != "completed" || resp.Usage.TotalTokens != 29 {
		t.Fatalf("unexpected normalized response: normalized=%+v usage=%+v", resp.Normalized, resp.Usage)
	}
}

func TestOpenAITargetBuildsTranscriptMessages(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			body := decodeRequestBody(t, req)
			messages, ok := body["messages"].([]any)
			if !ok || len(messages) != 4 {
				t.Fatalf("unexpected transcript messages: %#v", body["messages"])
			}
			first := messages[0].(map[string]any)
			last := messages[3].(map[string]any)
			if first["role"] != "developer" || last["role"] != "tool" {
				t.Fatalf("unexpected transcript roles: %#v %#v", first, last)
			}
			if last["tool_call_id"] != "call_1" || last["name"] != "lookup_policy" {
				t.Fatalf("unexpected tool transcript payload: %#v", last)
			}
			return jsonResponse(t, http.StatusOK, map[string]any{
				"id":     "chatcmpl_turns",
				"object": "chat.completion",
				"model":  "gpt-4o-mini",
				"choices": []any{
					map[string]any{"message": map[string]any{"role": "assistant", "content": "done"}, "finish_reason": "stop"},
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

	resp := target.Invoke(context.Background(), cleanr.BuildScenarioRequest(cleanr.Scenario{
		Name: "transcript",
		Turns: []cleanr.ConversationTurn{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "First turn"},
			{Role: "assistant", Content: "First answer"},
			{Role: "tool", Name: "lookup_policy", ToolCallID: "call_1", Content: "{\"policy\":\"refunds\"}"},
		},
	}, 2*time.Second))
	if resp.Err != nil || resp.Text != "done" {
		t.Fatalf("unexpected transcript response: %+v", resp)
	}
}

func TestOpenAITargetExpandsMockToolResultFixtures(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			body := decodeRequestBody(t, req)
			messages, ok := body["messages"].([]any)
			if !ok || len(messages) != 3 {
				t.Fatalf("unexpected fixture transcript messages: %#v", body["messages"])
			}
			assistant := messages[1].(map[string]any)
			tool := messages[2].(map[string]any)
			if assistant["role"] != "assistant" || assistant["content"] != "[mock tool call] lookup_policy {\"policy_id\":\"refunds\"}" {
				t.Fatalf("unexpected mocked assistant turn: %#v", assistant)
			}
			if tool["role"] != "tool" || tool["name"] != "lookup_policy" || tool["tool_call_id"] != "call_fixture_1" {
				t.Fatalf("unexpected mocked tool turn: %#v", tool)
			}
			return jsonResponse(t, http.StatusOK, map[string]any{
				"id":     "chatcmpl_fixture",
				"object": "chat.completion",
				"model":  "gpt-4o-mini",
				"choices": []any{
					map[string]any{"message": map[string]any{"role": "assistant", "content": "done"}, "finish_reason": "stop"},
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

	resp := target.Invoke(context.Background(), cleanr.BuildScenarioRequest(cleanr.Scenario{
		Name: "fixture-transcript",
		Turns: []cleanr.ConversationTurn{{
			Role:    "user",
			Content: "Check the refund policy",
			MockToolResults: []cleanr.MockToolResult{{
				Name:       "lookup_policy",
				ToolCallID: "call_fixture_1",
				Arguments:  `{"policy_id":"refunds"}`,
				Content:    `{"policy":"Refunds take 30 days."}`,
			}},
		}},
	}, 2*time.Second))
	if resp.Err != nil || resp.Text != "done" {
		t.Fatalf("unexpected fixture transcript response: %+v", resp)
	}
}

func TestOpenAITargetBuildsMultimodalResponsesInput(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			body := decodeRequestBody(t, req)
			input, ok := body["input"].([]any)
			if !ok || len(input) != 1 {
				t.Fatalf("unexpected responses input: %#v", body["input"])
			}
			msg := input[0].(map[string]any)
			content, ok := msg["content"].([]any)
			if !ok || len(content) < 3 {
				t.Fatalf("unexpected multimodal content: %#v", msg["content"])
			}
			image := content[1].(map[string]any)
			file := content[2].(map[string]any)
			if image["type"] != "input_image" || image["image_url"] != "https://example.test/refund.png" {
				t.Fatalf("unexpected image payload: %#v", image)
			}
			if file["type"] != "input_file" || file["file_url"] != "https://example.test/policy.pdf" {
				t.Fatalf("unexpected file payload: %#v", file)
			}
			return jsonResponse(t, http.StatusOK, map[string]any{
				"id":     "resp_mm",
				"model":  "gpt-4.1-mini",
				"status": "completed",
				"output": []any{
					map[string]any{
						"type": "message",
						"content": []any{
							map[string]any{"type": "output_text", "text": "done"},
						},
					},
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

	resp := target.Invoke(context.Background(), cleanr.BuildScenarioRequest(cleanr.Scenario{
		Name:  "multimodal",
		Input: "Review these artifacts",
		Images: []cleanr.MediaInput{{
			URL:    "https://example.test/refund.png",
			Detail: "high",
		}},
		PDFs: []cleanr.MediaInput{{
			URL:       "https://example.test/policy.pdf",
			MediaType: "application/pdf",
		}},
	}, 2*time.Second))
	if resp.Err != nil || resp.Text != "done" {
		t.Fatalf("unexpected multimodal response: %+v", resp)
	}
}

func TestOpenAITargetBuildsChatImageContent(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			body := decodeRequestBody(t, req)
			messages := body["messages"].([]any)
			content := messages[0].(map[string]any)["content"].([]any)
			if len(content) != 2 {
				t.Fatalf("unexpected chat multimodal content: %#v", content)
			}
			image := content[1].(map[string]any)["image_url"].(map[string]any)
			if image["url"] != "https://example.test/refund.png" || image["detail"] != "low" {
				t.Fatalf("unexpected chat image payload: %#v", image)
			}
			return jsonResponse(t, http.StatusOK, map[string]any{
				"id":     "chat_mm",
				"object": "chat.completion",
				"model":  "gpt-4o-mini",
				"choices": []any{
					map[string]any{"message": map[string]any{"role": "assistant", "content": "done"}, "finish_reason": "stop"},
				},
			}), nil
		}),
	}

	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai_compatible",
		OpenAI: cleanr.OpenAIConfig{
			APIMode:   "chat_completions",
			Model:     "gpt-4o-mini",
			APIKeyEnv: "OPENAI_API_KEY",
			BaseURL:   "https://compat.test/v1",
		},
	}, client)

	resp := target.Invoke(context.Background(), cleanr.BuildScenarioRequest(cleanr.Scenario{
		Name:  "chat-image",
		Input: "What is in this image?",
		Images: []cleanr.MediaInput{{
			URL:    "https://example.test/refund.png",
			Detail: "low",
		}},
	}, 2*time.Second))
	if resp.Err != nil || resp.Text != "done" {
		t.Fatalf("unexpected chat multimodal response: %+v", resp)
	}
}

func TestOpenAICompatibleTargetSupportsCustomAuthAndProviderLabel(t *testing.T) {
	t.Setenv("OLLAMA_API_KEY", "test-key")

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://compat.test/v1/chat/completions" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}
			if req.Header.Get("api-key") != "test-key" {
				t.Fatalf("unexpected custom auth header: %q", req.Header.Get("api-key"))
			}
			if got := req.Header.Get("Authorization"); got != "" {
				t.Fatalf("expected bearer auth to stay unset, got %q", got)
			}
			return jsonResponse(t, http.StatusOK, map[string]any{
				"id":     "chatcmpl_tool",
				"object": "chat.completion",
				"model":  "llama3.1",
				"choices": []any{
					map[string]any{
						"index": 0,
						"message": map[string]any{
							"role":    "assistant",
							"content": nil,
							"tool_calls": []any{
								map[string]any{
									"id":   "call_compat",
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
			}), nil
		}),
	}

	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai_compatible",
		OpenAI: cleanr.OpenAIConfig{
			APIMode:    "chat_completions",
			Model:      "llama3.1",
			APIKeyEnv:  "OLLAMA_API_KEY",
			BaseURL:    "https://compat.test/v1",
			Provider:   "ollama",
			AuthHeader: "api-key",
			AuthScheme: "none",
		},
	}, client)

	resp := target.Invoke(context.Background(), cleanr.Request{
		Prompt:  "Use tools when needed.",
		Timeout: 2 * time.Second,
	})

	if resp.Err != nil || resp.ExtractError != nil {
		t.Fatalf("unexpected response errors: err=%v extract=%v", resp.Err, resp.ExtractError)
	}
	if resp.Normalized.Provider != "ollama" {
		t.Fatalf("unexpected provider label: %+v", resp.Normalized)
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

func TestNativeProviderTargetsApplyProviderSpecificAuthAndEndpoints(t *testing.T) {
	t.Setenv("AZURE_OPENAI_API_KEY", "azure-key")
	t.Setenv("GEMINI_API_KEY", "gemini-key")
	t.Setenv("VERTEX_AI_ACCESS_TOKEN", "vertex-token")
	t.Setenv("BEDROCK_API_KEY", "bedrock-key")
	t.Setenv("MISTRAL_API_KEY", "mistral-key")

	tests := []struct {
		name            string
		cfg             cleanr.TargetConfig
		wantURL         string
		wantHeader      string
		wantHeaderValue string
		wantProvider    string
		wantSystemRole  string
	}{
		{
			name: "azure openai",
			cfg: cleanr.TargetConfig{
				Type: "azure_openai",
				OpenAI: cleanr.OpenAIConfig{
					APIMode:    "chat_completions",
					Model:      "gpt-4o-mini",
					BaseURL:    "https://azure.test/openai/deployments/test-deployment",
					APIVersion: "2025-03-01-preview",
				},
			},
			wantURL:         "https://azure.test/openai/deployments/test-deployment/chat/completions?api-version=2025-03-01-preview",
			wantHeader:      "api-key",
			wantHeaderValue: "azure-key",
			wantProvider:    "azure_openai",
			wantSystemRole:  "system",
		},
		{
			name: "gemini",
			cfg: cleanr.TargetConfig{
				Type: "gemini",
				OpenAI: cleanr.OpenAIConfig{
					APIMode: "chat_completions",
					Model:   "gemini-2.5-flash",
				},
			},
			wantURL:         "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions",
			wantHeader:      "Authorization",
			wantHeaderValue: "Bearer gemini-key",
			wantProvider:    "gemini",
			wantSystemRole:  "system",
		},
		{
			name: "vertex",
			cfg: cleanr.TargetConfig{
				Type: "vertex",
				OpenAI: cleanr.OpenAIConfig{
					APIMode: "chat_completions",
					Model:   "gemini-2.5-pro",
					BaseURL: "https://vertex.test/v1/openapi",
				},
			},
			wantURL:         "https://vertex.test/v1/openapi/chat/completions",
			wantHeader:      "Authorization",
			wantHeaderValue: "Bearer vertex-token",
			wantProvider:    "vertex",
			wantSystemRole:  "system",
		},
		{
			name: "bedrock",
			cfg: cleanr.TargetConfig{
				Type: "bedrock",
				OpenAI: cleanr.OpenAIConfig{
					APIMode: "chat_completions",
					Model:   "anthropic.claude-3-5-sonnet-20240620-v1:0",
					BaseURL: "https://bedrock.test/openai/v1",
				},
			},
			wantURL:         "https://bedrock.test/openai/v1/chat/completions",
			wantHeader:      "Authorization",
			wantHeaderValue: "Bearer bedrock-key",
			wantProvider:    "bedrock",
			wantSystemRole:  "system",
		},
		{
			name: "mistral",
			cfg: cleanr.TargetConfig{
				Type: "mistral",
				OpenAI: cleanr.OpenAIConfig{
					APIMode: "chat_completions",
					Model:   "mistral-small-latest",
				},
			},
			wantURL:         "https://api.mistral.ai/v1/chat/completions",
			wantHeader:      "Authorization",
			wantHeaderValue: "Bearer mistral-key",
			wantProvider:    "mistral",
			wantSystemRole:  "system",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.String() != tt.wantURL {
						t.Fatalf("unexpected url: %s", req.URL.String())
					}
					if got := req.Header.Get(tt.wantHeader); got != tt.wantHeaderValue {
						t.Fatalf("unexpected auth header %s=%q", tt.wantHeader, got)
					}
					body := decodeRequestBody(t, req)
					messages, ok := body["messages"].([]any)
					if !ok || len(messages) == 0 {
						t.Fatalf("unexpected chat messages: %#v", body["messages"])
					}
					first, ok := messages[0].(map[string]any)
					if !ok || first["role"] != tt.wantSystemRole {
						t.Fatalf("unexpected first message: %#v", body["messages"])
					}
					return jsonResponse(t, http.StatusOK, map[string]any{
						"id":     "chatcmpl_native",
						"object": "chat.completion",
						"model":  tt.cfg.OpenAI.Model,
						"choices": []any{
							map[string]any{
								"index": 0,
								"message": map[string]any{
									"role":    "assistant",
									"content": "ok",
								},
								"finish_reason": "stop",
							},
						},
					}), nil
				}),
			}

			resp := cleanr.NewOpenAITarget(tt.cfg, client).Invoke(context.Background(), cleanr.Request{
				System:  "You are a concise assistant.",
				Prompt:  "Say ok.",
				Timeout: 2 * time.Second,
			})
			if resp.Err != nil || resp.Text != "ok" {
				t.Fatalf("unexpected response: %+v", resp)
			}
			if resp.Normalized.Provider != tt.wantProvider {
				t.Fatalf("unexpected provider label: %+v", resp.Normalized)
			}
		})
	}
}

func marshalOpenAIConfig(cfg map[string]any) string {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(data)
}
