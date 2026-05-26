package tests

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	profilepkg "github.com/devr-tools/cleanr/cleanr/profile"
)

func TestOpenAIUsesStoredCredentialFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CLEANR_HOME", home)
	t.Setenv("OPENAI_API_KEY", "")
	if err := profilepkg.UpsertProvider(profilepkg.Provider{
		Name:      "openai",
		APIKeyEnv: "OPENAI_API_KEY",
		APIKey:    "stored-key",
		Model:     "gpt-4.1-mini",
	}); err != nil {
		t.Fatalf("store openai provider: %v", err)
	}

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Authorization"); got != "Bearer stored-key" {
				t.Fatalf("expected stored credential auth header, got %q", got)
			}
			return jsonResponse(t, http.StatusOK, map[string]any{
				"id":     "resp_stored",
				"model":  "gpt-4.1-mini",
				"status": "completed",
				"output": []any{
					map[string]any{
						"type": "message",
						"content": []any{
							map[string]any{"type": "output_text", "text": "hello"},
						},
					},
				},
				"usage": map[string]any{
					"input_tokens":  1,
					"output_tokens": 1,
					"total_tokens":  2,
				},
			}), nil
		}),
	}

	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			Model:   "gpt-4.1-mini",
			BaseURL: "https://openai.test/v1",
		},
	}, client)
	resp := target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
	if resp.Err != nil || resp.Text != "hello" {
		t.Fatalf("unexpected stored-credential response: %+v", resp)
	}
}

func TestOpenAIReturnsStoredCredentialLoadErrors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CLEANR_HOME", home)
	t.Setenv("OPENAI_API_KEY", "")
	path, err := profilepkg.Path()
	if err != nil {
		t.Fatalf("profile path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir profile dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write broken profile: %v", err)
	}

	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			Model: "gpt-4.1-mini",
		},
	}, &http.Client{})
	resp := target.Invoke(context.Background(), cleanr.Request{})
	if resp.Err == nil || !strings.Contains(resp.Err.Error(), "load stored openai api key") {
		t.Fatalf("expected stored credential load error, got %v", resp.Err)
	}
}

func TestOpenAIHandlesChatParseAndAPIErrorFallbacks(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	t.Run("chat tool calls suppress unsupported content parse errors", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(t, http.StatusOK, map[string]any{
					"id":     "chat_tool",
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
											"arguments": "{}",
										},
									},
								},
							},
						},
					},
					"usage": map[string]any{
						"prompt_tokens":     2,
						"completion_tokens": 1,
					},
				}), nil
			}),
		}

		target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
			Type: "openai",
			OpenAI: cleanr.OpenAIConfig{
				Model:   "gpt-4o-mini",
				APIMode: "chat_completions",
				BaseURL: "https://openai.test/v1",
			},
		}, client)
		resp := target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
		if resp.ExtractError != nil || len(resp.Normalized.ToolCalls) != 1 || resp.Usage.TotalTokens != 3 {
			t.Fatalf("unexpected chat tool response: %+v", resp)
		}
	})

	t.Run("chat response with no choices becomes extract error", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(t, http.StatusOK, map[string]any{
					"id":      "chat_empty",
					"object":  "chat.completion",
					"model":   "gpt-4o-mini",
					"choices": []any{},
				}), nil
			}),
		}
		target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
			Type: "openai",
			OpenAI: cleanr.OpenAIConfig{
				Model:   "gpt-4o-mini",
				APIMode: "chat_completions",
				BaseURL: "https://openai.test/v1",
			},
		}, client)
		resp := target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
		if resp.ExtractError == nil || resp.Normalized.Raw["object"] != "chat.completion" {
			t.Fatalf("unexpected empty chat response: %+v", resp)
		}
	})

	t.Run("api error fallback for non-json body", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Status:     "502 Bad Gateway",
					Body:       io.NopCloser(strings.NewReader("nope")),
					Header:     http.Header{"Content-Type": []string{"text/plain"}},
				}, nil
			}),
		}
		target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
			Type: "openai",
			OpenAI: cleanr.OpenAIConfig{
				Model:   "gpt-4.1-mini",
				BaseURL: "https://openai.test/v1",
			},
		}, client)
		resp := target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
		if resp.Err == nil || !strings.Contains(resp.Err.Error(), "openai api error (502)") {
			t.Fatalf("expected fallback api error, got %v", resp.Err)
		}
	})

	t.Run("response parse with incomplete reason still succeeds", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				body := decodeRequestBody(t, req)
				if _, ok := body["metadata"]; !ok {
					t.Fatalf("expected metadata to be sent: %#v", body)
				}
				return jsonResponse(t, http.StatusOK, map[string]any{
					"id":     "resp_1",
					"model":  "gpt-4.1-mini",
					"status": "incomplete",
					"incomplete_details": map[string]any{
						"reason": "max_output_tokens",
					},
					"output": []any{
						map[string]any{
							"type": "message",
							"content": []any{
								map[string]any{"type": "output_text", "text": "short answer"},
							},
						},
					},
					"usage": map[string]any{
						"input_tokens":  2,
						"output_tokens": 2,
						"total_tokens":  4,
					},
				}), nil
			}),
		}
		target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
			Type: "openai",
			OpenAI: cleanr.OpenAIConfig{
				Model:   "gpt-4.1-mini",
				BaseURL: "https://openai.test/v1",
			},
		}, client)
		resp := target.Invoke(context.Background(), cleanr.Request{
			Prompt: "hello",
			Scenario: cleanr.Scenario{
				Metadata: map[string]string{"trace_id": "123"},
			},
			Timeout: time.Second,
		})
		if resp.Err != nil || resp.ExtractError != nil {
			t.Fatalf("unexpected incomplete response parse: %+v", resp)
		}
		if resp.Normalized.Raw["incomplete_reason"] != "max_output_tokens" {
			t.Fatalf("expected incomplete reason in normalized raw: %+v", resp.Normalized)
		}
	})
}

func TestOpenAIChatStringContentParsesNormally(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(t, http.StatusOK, map[string]any{
				"id":     "chat_text",
				"object": "chat.completion",
				"model":  "gpt-4o-mini",
				"choices": []any{
					map[string]any{
						"finish_reason": "stop",
						"message": map[string]any{
							"role":    "assistant",
							"content": "plain string content",
						},
					},
				},
			}), nil
		}),
	}
	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			Model:   "gpt-4o-mini",
			APIMode: "chat_completions",
			BaseURL: "https://openai.test/v1",
		},
	}, client)
	resp := target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
	if resp.Err != nil || resp.Text != "plain string content" {
		t.Fatalf("unexpected plain string chat response: %+v", resp)
	}
}

func TestOpenAIStoredProfileKeyWithMismatchedEnvNameDoesNotApply(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CLEANR_HOME", home)
	t.Setenv("OPENAI_API_KEY", "")
	if err := profilepkg.UpsertProvider(profilepkg.Provider{
		Name:      "openai",
		APIKeyEnv: "OTHER_ENV",
		APIKey:    "stored-key",
		Model:     "gpt-4.1-mini",
	}); err != nil {
		t.Fatalf("store profile: %v", err)
	}
	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			Model: "gpt-4.1-mini",
		},
	}, &http.Client{})
	resp := target.Invoke(context.Background(), cleanr.Request{})
	if resp.Err == nil || !strings.Contains(resp.Err.Error(), `openai api key env "OPENAI_API_KEY" is not set`) {
		t.Fatalf("expected env mismatch to ignore stored key, got %v", resp.Err)
	}
}

func TestOpenAIResponseUsageFallbackFromPromptCompletionTokens(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(t, http.StatusOK, map[string]any{
				"id":     "chat_usage",
				"object": "chat.completion",
				"model":  "gpt-4o-mini",
				"choices": []any{
					map[string]any{
						"finish_reason": "stop",
						"message": map[string]any{
							"role": "assistant",
							"content": []any{
								map[string]any{"text": "ok"},
							},
						},
					},
				},
				"usage": map[string]any{
					"prompt_tokens":     3,
					"completion_tokens": 4,
				},
			}), nil
		}),
	}
	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			Model:   "gpt-4o-mini",
			APIMode: "chat_completions",
			BaseURL: "https://openai.test/v1",
		},
	}, client)
	resp := target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
	if resp.Usage.InputTokens != 3 || resp.Usage.OutputTokens != 4 || resp.Usage.TotalTokens != 7 {
		t.Fatalf("unexpected usage fallback: %+v", resp.Usage)
	}
}
