package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"cleanr/cleanr"
	"cleanr/internal/cli"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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

	configPath := writeNamedConfigFile(t, "openai-chat.json", marshalOpenAIConfig(map[string]any{
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

	configPath := writeNamedConfigFile(t, "openai-responses.json", marshalOpenAIConfig(map[string]any{
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
	configPath := writeNamedConfigFile(t, "openai-validate.json", marshalOpenAIConfig(map[string]any{
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

func stubDefaultTransport(t *testing.T, transport http.RoundTripper) func() {
	t.Helper()

	original := http.DefaultTransport
	http.DefaultTransport = transport
	return func() {
		http.DefaultTransport = original
	}
}

func jsonResponse(t *testing.T, statusCode int, body map[string]any) *http.Response {
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

func marshalOpenAIConfig(cfg map[string]any) string {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(data)
}

func runConfigAsJSONReport(t *testing.T, configPath string) cleanr.Report {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"run", "-config", configPath, "-format", "json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected run to succeed, got exit code %d, stdout=%s, stderr=%s", exitCode, stdout.String(), stderr.String())
	}

	var report cleanr.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\nstdout=%s", err, stdout.String())
	}
	return report
}

func requirePassingSecurityReport(t *testing.T, report cleanr.Report, wantName string) {
	t.Helper()

	if report.Name != wantName {
		t.Fatalf("unexpected report name %q", report.Name)
	}
	if !report.Passed {
		t.Fatalf("expected passing report, got %+v", report)
	}
	if len(report.Suites) != 1 {
		t.Fatalf("expected 1 suite, got %+v", report.Suites)
	}

	suite := report.Suites[0]
	if suite.Name != "security" {
		t.Fatalf("expected security suite, got %+v", suite)
	}
	if !suite.Passed {
		t.Fatalf("expected passing suite, got %+v", suite)
	}
	if len(suite.Cases) != 1 || !suite.Cases[0].Passed {
		t.Fatalf("expected one passing case, got %+v", suite.Cases)
	}
}

func decodeRequestBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()

	defer r.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return body
}

func assertChatCompletionsRequest(body map[string]any, wantSystem, wantPrompt string) error {
	if strings.TrimSpace(stringValue(body["model"])) == "" {
		return fmt.Errorf("request missing model")
	}

	rawMessages, ok := body["messages"].([]any)
	if !ok || len(rawMessages) == 0 {
		return fmt.Errorf("request missing messages array")
	}

	var sawSystem bool
	var sawUser bool
	for _, raw := range rawMessages {
		msg, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		role := stringValue(msg["role"])
		switch role {
		case "system", "developer":
			if containsTextFragment(msg["content"], wantSystem) {
				sawSystem = true
			}
		case "user":
			if containsTextFragment(msg["content"], wantPrompt) {
				sawUser = true
			}
		}
	}

	if !sawSystem {
		return fmt.Errorf("request missing system/developer instruction %q", wantSystem)
	}
	if !sawUser {
		return fmt.Errorf("request missing user prompt %q", wantPrompt)
	}
	return nil
}

func assertResponsesRequest(body map[string]any, wantSystem, wantPrompt string) error {
	if strings.TrimSpace(stringValue(body["model"])) == "" {
		return fmt.Errorf("request missing model")
	}

	sawSystem := containsTextFragment(body["instructions"], wantSystem) ||
		rolePayloadContainsText(body["input"], []string{"system", "developer"}, wantSystem)
	sawPrompt := containsTextFragment(body["input"], wantPrompt) ||
		rolePayloadContainsText(body["input"], []string{"user"}, wantPrompt)

	if !sawSystem {
		return fmt.Errorf("request missing system/developer instruction %q", wantSystem)
	}
	if !sawPrompt {
		return fmt.Errorf("request missing user input %q", wantPrompt)
	}
	return nil
}

func rolePayloadContainsText(v any, roles []string, want string) bool {
	switch typed := v.(type) {
	case []any:
		for _, item := range typed {
			if rolePayloadContainsText(item, roles, want) {
				return true
			}
		}
	case map[string]any:
		role := stringValue(typed["role"])
		for _, allowed := range roles {
			if role == allowed && containsTextFragment(typed["content"], want) {
				return true
			}
		}
		for _, item := range typed {
			if rolePayloadContainsText(item, roles, want) {
				return true
			}
		}
	}
	return false
}

func containsTextFragment(v any, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	for _, text := range collectTextFragments(v) {
		if strings.Contains(text, want) {
			return true
		}
	}
	return false
}

func collectTextFragments(v any) []string {
	switch typed := v.(type) {
	case string:
		return []string{typed}
	case []any:
		var out []string
		for _, item := range typed {
			out = append(out, collectTextFragments(item)...)
		}
		return out
	case map[string]any:
		var out []string
		for _, value := range typed {
			out = append(out, collectTextFragments(value)...)
		}
		return out
	default:
		return nil
	}
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}
