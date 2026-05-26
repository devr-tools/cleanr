package tests

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	profilepkg "github.com/devr-tools/cleanr/cleanr/profile"
)

func TestOpenAITargetFallsBackToStoredCredentials(t *testing.T) {
	t.Setenv("CLEANR_HOME", t.TempDir())
	if err := profilepkg.UpsertProvider(profilepkg.Provider{
		Name:      "openai",
		Model:     "gpt-4.1-mini",
		APIMode:   "responses",
		APIKeyEnv: "OPENAI_API_KEY",
		APIKey:    "stored-openai-key",
	}); err != nil {
		t.Fatalf("store profile: %v", err)
	}

	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != "Bearer stored-openai-key" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		return jsonResponse(t, http.StatusOK, map[string]any{
			"id":     "resp_123",
			"model":  "gpt-4.1-mini",
			"status": "completed",
			"output": []any{
				map[string]any{
					"type": "message",
					"content": []any{
						map[string]any{"type": "output_text", "text": "ok"},
					},
				},
			},
		}), nil
	})}
	target := cleanr.NewOpenAITarget(cleanr.TargetConfig{
		Type: "openai",
		OpenAI: cleanr.OpenAIConfig{
			Model:     "gpt-4.1-mini",
			APIMode:   "responses",
			APIKeyEnv: "OPENAI_API_KEY",
		},
	}, client)
	resp := target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
	if resp.Err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response: err=%v status=%d", resp.Err, resp.StatusCode)
	}
}

func TestAnthropicTargetFallsBackToStoredCredentials(t *testing.T) {
	t.Setenv("CLEANR_HOME", t.TempDir())
	if err := profilepkg.UpsertProvider(profilepkg.Provider{
		Name:      "anthropic",
		Model:     "claude-sonnet-4-20250514",
		APIKeyEnv: "ANTHROPIC_API_KEY",
		APIKey:    "stored-anthropic-key",
		MaxTokens: 1024,
	}); err != nil {
		t.Fatalf("store profile: %v", err)
	}

	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("x-api-key"); got != "stored-anthropic-key" {
			t.Fatalf("unexpected api key header: %q", got)
		}
		return jsonResponse(t, http.StatusOK, map[string]any{
			"id":    "msg_123",
			"model": "claude-sonnet-4-20250514",
			"role":  "assistant",
			"content": []any{
				map[string]any{"type": "text", "text": "ok"},
			},
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 1,
			},
		}), nil
	})}
	target := cleanr.NewAnthropicTarget(cleanr.TargetConfig{
		Type: "anthropic",
		Anthropic: cleanr.AnthropicConfig{
			Model:     "claude-sonnet-4-20250514",
			APIKeyEnv: "ANTHROPIC_API_KEY",
		},
	}, client)
	resp := target.Invoke(context.Background(), cleanr.Request{Prompt: "hello", Timeout: time.Second})
	if resp.Err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response: err=%v status=%d", resp.Err, resp.StatusCode)
	}
}
