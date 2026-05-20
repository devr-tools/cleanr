package tests

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"cleanr/cleanr"
)

func TestOpenAITargetCoversMissingEnvAndAPIErrorBranches(t *testing.T) {
	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			Model: "gpt-4.1-mini",
		},
	}, &http.Client{})
	if resp := target.Invoke(context.Background(), cleanr.Request{}); resp.Err == nil || !strings.Contains(resp.Err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("expected missing api key error, got %v", resp.Err)
	}

	t.Setenv("OPENAI_API_KEY", "test-key")
	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.openai.com/v1/responses" {
			t.Fatalf("unexpected default endpoint: %s", req.URL.String())
		}
		return jsonResponse(t, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "bad request"},
		}), nil
	})}
	target = cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			Model: "gpt-4.1-mini",
		},
	}, client)
	resp := target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
	if resp.Err == nil || !strings.Contains(resp.Err.Error(), "openai api error (400): bad request") {
		t.Fatalf("expected structured api error, got %v", resp.Err)
	}

	client = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Status:     "500 Internal Server Error",
			Body:       io.NopCloser(strings.NewReader("not-json")),
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
		}, nil
	})}
	target = cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			Model:   "gpt-4.1-mini",
			BaseURL: "https://openai.test/v1",
		},
	}, client)
	resp = target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
	if resp.Err == nil || !strings.Contains(resp.Err.Error(), "openai api error (500)") {
		t.Fatalf("expected fallback api error, got %v", resp.Err)
	}
}

func TestOpenAITargetCoversRequestBodyAndParsingBranches(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://openai.test/v1/chat/completions" {
			t.Fatalf("unexpected chat endpoint: %s", req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("expected default auth header, got %q", req.Header.Get("Authorization"))
		}
		return jsonResponse(t, http.StatusOK, map[string]any{
			"id":     "chat_empty",
			"object": "chat.completion",
			"model":  "gpt-4o-mini",
			"choices": []any{
				map[string]any{
					"finish_reason": "tool_calls",
					"message": map[string]any{
						"role":    "assistant",
						"content": 123,
						"tool_calls": []any{
							map[string]any{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "lookup_policy",
									"arguments": "{\"id\":\"refunds\"}",
								},
							},
						},
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 3,
			},
		}), nil
	})}
	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			Model:     "gpt-4o-mini",
			APIMode:   "chat_completions",
			APIKeyEnv: " OPENAI_API_KEY ",
			BaseURL:   "https://openai.test/v1",
		},
	}, client)
	resp := target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
	if resp.ExtractError != nil {
		t.Fatalf("expected tool-call response to suppress content parse error, got %v", resp.ExtractError)
	}
	if len(resp.Normalized.ToolCalls) != 1 || resp.Usage.TotalTokens != 7 {
		t.Fatalf("unexpected chat tool-call response: %+v %+v", resp.Normalized, resp.Usage)
	}

	client = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(t, http.StatusOK, map[string]any{
			"id":      "chat_none",
			"object":  "chat.completion",
			"model":   "gpt-4o-mini",
			"choices": []any{},
		}), nil
	})}
	target = cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			Model:   "gpt-4o-mini",
			APIMode: "chat_completions",
			BaseURL: "https://openai.test/v1",
		},
	}, client)
	resp = target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
	if resp.ExtractError == nil || resp.Normalized.Raw["object"] != "chat.completion" {
		t.Fatalf("expected empty-choice parse result, got %+v %+v", resp.ExtractError, resp.Normalized)
	}

	client = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := decodeRequestBody(t, req)
		if _, ok := body["metadata"]; !ok {
			t.Fatalf("expected metadata in responses request: %#v", body)
		}
		return jsonResponse(t, http.StatusOK, map[string]any{
			"id":     "resp_tool",
			"model":  "gpt-4.1-mini",
			"status": "completed",
			"output": []any{
				map[string]any{"type": "reasoning"},
				map[string]any{
					"id":        "call_2",
					"type":      "function_call",
					"call_id":   "call_2",
					"name":      "search_docs",
					"arguments": "{\"q\":\"refunds\"}",
					"status":    "completed",
					"role":      "assistant",
				},
			},
			"usage": map[string]any{
				"input_tokens":  10,
				"output_tokens": 2,
				"total_tokens":  12,
			},
		}), nil
	})}
	target = cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			Model:   "gpt-4.1-mini",
			BaseURL: "https://openai.test/v1",
		},
	}, client)
	resp = target.Invoke(context.Background(), cleanr.Request{
		Prompt: "hello",
		Scenario: cleanr.Scenario{
			Metadata: map[string]string{"trace_id": "123"},
		},
		Timeout: time.Second,
	})
	if resp.ExtractError != nil || len(resp.Normalized.ToolCalls) != 1 {
		t.Fatalf("unexpected responses tool-call parse: err=%v normalized=%+v", resp.ExtractError, resp.Normalized)
	}

	target = cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			Model:   "gpt-4.1-mini",
			APIMode: "weird",
		},
	}, &http.Client{})
	resp = target.Invoke(context.Background(), cleanr.Request{Prompt: "hello"})
	if resp.Err == nil || !strings.Contains(resp.Err.Error(), `unsupported openai api_mode "weird"`) {
		t.Fatalf("expected unsupported api mode error, got %v", resp.Err)
	}
}

func TestAnthropicTargetCoversMissingEnvAndAPIErrorBranches(t *testing.T) {
	target := cleanr.NewAnthropicTarget(cleanr.TargetConfig{
		Type: "anthropic",
		Anthropic: cleanr.AnthropicConfig{
			Model: "claude-sonnet-4-20250514",
		},
	}, &http.Client{})
	if resp := target.Invoke(context.Background(), cleanr.Request{}); resp.Err == nil || !strings.Contains(resp.Err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("expected missing anthropic api key error, got %v", resp.Err)
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.anthropic.com/v1/messages" {
			t.Fatalf("unexpected default anthropic endpoint: %s", req.URL.String())
		}
		if req.Header.Get("anthropic-version") != "2023-06-01" {
			t.Fatalf("unexpected default version: %s", req.Header.Get("anthropic-version"))
		}
		body := decodeRequestBody(t, req)
		if body["max_tokens"] != float64(1024) {
			t.Fatalf("unexpected default max_tokens: %#v", body["max_tokens"])
		}
		return jsonResponse(t, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "bad anthropic request"},
		}), nil
	})}
	target = cleanr.NewAnthropicTarget(cleanr.TargetConfig{
		Type: "anthropic",
		Anthropic: cleanr.AnthropicConfig{
			Model: "claude-sonnet-4-20250514",
		},
	}, client)
	resp := target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
	if resp.Err == nil || !strings.Contains(resp.Err.Error(), "anthropic api error (400): bad anthropic request") {
		t.Fatalf("expected structured anthropic api error, got %v", resp.Err)
	}

	client = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Status:     "500 Internal Server Error",
			Body:       io.NopCloser(strings.NewReader(`{"oops":true}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	target = cleanr.NewAnthropicTarget(cleanr.TargetConfig{
		Type: "anthropic",
		Anthropic: cleanr.AnthropicConfig{
			Model:     "claude-sonnet-4-20250514",
			APIKeyEnv: " ANTHROPIC_API_KEY ",
			BaseURL:   "https://anthropic.test/v1",
			Version:   " 2025-01-01 ",
			MaxTokens: 2048,
		},
	}, client)
	resp = target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
	if resp.Err == nil || !strings.Contains(resp.Err.Error(), "anthropic api error (500)") {
		t.Fatalf("expected fallback anthropic api error, got %v", resp.Err)
	}

	client = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(t, http.StatusOK, map[string]any{
			"id":      "msg_empty",
			"model":   "claude-sonnet-4-20250514",
			"role":    "assistant",
			"content": []any{},
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 2,
			},
		}), nil
	})}
	target = cleanr.NewAnthropicTarget(cleanr.TargetConfig{
		Type: "anthropic",
		Anthropic: cleanr.AnthropicConfig{
			Model: "claude-sonnet-4-20250514",
		},
	}, client)
	resp = target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
	if resp.ExtractError == nil {
		t.Fatalf("expected empty anthropic content extract error")
	}
}
