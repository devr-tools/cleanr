package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"cleanr/cleanr"
	"cleanr/internal/cli"
)

func TestAnthropicTargetParsesMessagesAPIUsage(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://anthropic.test/v1/messages" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}
			if req.Header.Get("x-api-key") != "test-key" {
				t.Fatalf("unexpected api key header: %s", req.Header.Get("x-api-key"))
			}
			if req.Header.Get("anthropic-version") != "2023-06-01" {
				t.Fatalf("unexpected anthropic version: %s", req.Header.Get("anthropic-version"))
			}

			body := decodeRequestBody(t, req)
			if err := assertAnthropicMessagesRequest(body, "You are a security reviewer.", "Give one short password-hardening recommendation.", 512); err != nil {
				t.Fatalf("unexpected request: %v", err)
			}

			return jsonResponse(t, http.StatusOK, map[string]any{
				"id":    "msg_test",
				"type":  "message",
				"role":  "assistant",
				"model": "claude-sonnet-4-20250514",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": "Require MFA on every admin account.",
					},
				},
				"usage": map[string]any{
					"input_tokens":  21,
					"output_tokens": 8,
				},
			}), nil
		}),
	}

	target := cleanr.NewAnthropicTarget(cleanr.TargetConfig{
		Type: "anthropic",
		Anthropic: cleanr.AnthropicConfig{
			Model:     "claude-sonnet-4-20250514",
			APIKeyEnv: "ANTHROPIC_API_KEY",
			BaseURL:   "https://anthropic.test/v1",
			Version:   "2023-06-01",
			MaxTokens: 512,
		},
	}, client)

	resp := target.Invoke(context.Background(), cleanr.Request{
		Scenario: cleanr.Scenario{
			Name:   "anthropic-messages",
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
}

func TestRunCommandSupportsAnthropicTarget(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	restoreTransport := stubDefaultTransport(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.anthropic.com/v1/messages" {
			t.Fatalf("unexpected url: %s", req.URL.String())
		}
		if req.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("unexpected api key header: %s", req.Header.Get("x-api-key"))
		}
		if req.Header.Get("anthropic-version") != "2023-06-01" {
			t.Fatalf("unexpected anthropic version: %s", req.Header.Get("anthropic-version"))
		}

		body := decodeRequestBody(t, req)
		if err := assertAnthropicMessagesRequest(body, "You are a concise support assistant.", "Summarize the refund policy in one sentence.", 1024); err != nil {
			t.Fatalf("unexpected request: %v", err)
		}

		return jsonResponse(t, http.StatusOK, map[string]any{
			"id":    "msg_test",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-sonnet-4-20250514",
			"content": []any{
				map[string]any{
					"type": "text",
					"text": "Refunds are available within 30 days of purchase.",
				},
			},
			"usage": map[string]any{
				"input_tokens":  24,
				"output_tokens": 11,
			},
		}), nil
	}))
	defer restoreTransport()

	configPath := writeNamedConfigFile(t, "anthropic-messages.json", marshalProviderConfig(map[string]any{
		"version": "v1alpha1",
		"target": map[string]any{
			"type": "anthropic",
			"name": "anthropic-messages",
			"anthropic": map[string]any{
				"model": "claude-sonnet-4-20250514",
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
	requirePassingSecurityReport(t, report, "anthropic-messages")
}

func TestValidateCommandAcceptsAnthropicConfig(t *testing.T) {
	configPath := writeNamedConfigFile(t, "anthropic-validate.json", marshalProviderConfig(map[string]any{
		"version": "v1alpha1",
		"target": map[string]any{
			"type": "anthropic",
			"name": "anthropic-validate",
			"anthropic": map[string]any{
				"model": "claude-sonnet-4-20250514",
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
	if !strings.Contains(stdout.String(), "valid config for anthropic-validate with 1 scenarios") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func marshalProviderConfig(cfg map[string]any) string {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(data)
}

func assertAnthropicMessagesRequest(body map[string]any, wantSystem, wantPrompt string, wantMaxTokens int) error {
	if strings.TrimSpace(stringValue(body["model"])) == "" {
		return fmt.Errorf("request missing model")
	}
	if intValue(body["max_tokens"]) != wantMaxTokens {
		return fmt.Errorf("request max_tokens=%d, want %d", intValue(body["max_tokens"]), wantMaxTokens)
	}
	if !containsTextFragment(body["system"], wantSystem) {
		return fmt.Errorf("request missing system prompt %q", wantSystem)
	}

	rawMessages, ok := body["messages"].([]any)
	if !ok || len(rawMessages) != 1 {
		return fmt.Errorf("request missing user messages array")
	}
	msg, ok := rawMessages[0].(map[string]any)
	if !ok {
		return fmt.Errorf("request message has unexpected shape")
	}
	if stringValue(msg["role"]) != "user" {
		return fmt.Errorf("request role=%q, want user", stringValue(msg["role"]))
	}
	if !containsTextFragment(msg["content"], wantPrompt) {
		return fmt.Errorf("request missing user prompt %q", wantPrompt)
	}
	return nil
}

func intValue(v any) int {
	switch typed := v.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return 0
	}
}
