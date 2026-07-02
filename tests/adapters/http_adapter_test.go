package tests

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	adapterspkg "github.com/devr-tools/cleanr/cleanr/adapters"
)

type invalidJSONValue struct{}

func (invalidJSONValue) MarshalJSON() ([]byte, error) {
	return []byte("{"), nil
}

func TestHTTPTargetInvokeRendersTemplateAndExtractsResponse(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("X-Base"); got != "override" {
			t.Fatalf("unexpected merged header: %q", got)
		}
		if got := r.Header.Get("X-Req"); got != "present" {
			t.Fatalf("unexpected request header: %q", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["input"] != "Prompt text" || body["system"] != "System text" {
			t.Fatalf("unexpected prompt/system payload: %+v", body)
		}
		nested, ok := body["nested"].([]any)
		if !ok || len(nested) != 2 {
			t.Fatalf("unexpected nested payload: %#v", body["nested"])
		}
		if nested[0] != "Prompt text" {
			t.Fatalf("unexpected prompt interpolation: %#v", nested[0])
		}
		meta, ok := body["metadata"].(map[string]any)
		if !ok || meta["trace_id"] != "abc-123" {
			t.Fatalf("unexpected metadata payload: %#v", body["metadata"])
		}
		return jsonResponse(t, http.StatusOK, map[string]any{
			"output": map[string]any{"text": "hello from http"},
			"trace": map[string]any{
				"provider":      "sample-agent",
				"model":         "workflow-v1",
				"finish_reason": "stop",
				"tool_calls": []any{
					map[string]any{"name": "lookup_customer", "arguments": `{"customer_id":"cust_123"}`},
				},
				"approvals": []any{
					map[string]any{"id": "appr_1", "status": "approved", "artifact": "ticket://appr_1"},
				},
				"state_changes": []any{
					map[string]any{"kind": "ticket", "action": "update", "target": "case-123", "status": "applied"},
				},
			},
			"usage": map[string]any{
				"input_tokens":  11,
				"output_tokens": 7,
				"total_tokens":  18,
			},
			"stream": map[string]any{
				"ttft_ms":     42,
				"duration_ms": 240,
				"chunk_count": 6,
				"error_count": 1,
				"recovered":   true,
			},
		}), nil
	})}

	target := cleanr.NewHTTPTarget(cleanr.TargetConfig{
		URL:           "https://example.test/http",
		Method:        http.MethodPost,
		PromptField:   "input",
		SystemField:   "system",
		ResponseField: "output.text",
		Headers:       map[string]string{"X-Base": "base"},
		RequestTemplate: map[string]any{
			"nested": []any{
				"{{prompt}}",
				map[string]any{"kind": "{{scenario.name}}", "system": "{{system}}"},
			},
		},
	}, client)

	resp := target.Invoke(context.Background(), cleanr.Request{
		Scenario: cleanr.Scenario{
			Name:     "happy-http",
			Metadata: map[string]string{"trace_id": "abc-123"},
		},
		System:  "System text",
		Prompt:  "Prompt text",
		Headers: map[string]string{"X-Base": "override", "X-Req": "present"},
		Timeout: time.Second,
	})

	if resp.Err != nil || resp.ExtractError != nil {
		t.Fatalf("unexpected response errors: err=%v extract=%v", resp.Err, resp.ExtractError)
	}
	if resp.Text != "hello from http" {
		t.Fatalf("unexpected response text: %q", resp.Text)
	}
	if resp.Normalized.Provider != "sample-agent" || resp.Normalized.Model != "workflow-v1" || resp.Normalized.FinishReason != "stop" {
		t.Fatalf("unexpected normalized payload: %+v", resp.Normalized)
	}
	if len(resp.Normalized.ToolCalls) != 1 || len(resp.Normalized.Approvals) != 1 || len(resp.Normalized.StateChanges) != 1 {
		t.Fatalf("expected normalized workflow evidence, got %+v", resp.Normalized)
	}
	if resp.Usage.TotalTokens != 18 {
		t.Fatalf("expected parsed usage, got %+v", resp.Usage)
	}
	if resp.Stream.TTFTMS != 42 || resp.Stream.DurationMS != 240 || resp.Stream.ChunkCount != 6 || !resp.Stream.Recovered {
		t.Fatalf("expected parsed stream metrics, got %+v", resp.Stream)
	}
}

func TestHTTPTargetInvokeParsesSSEStreamMetrics(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("unexpected accept header: %q", got)
		}
		return sseResponse(http.StatusOK, strings.Join([]string{
			"event: message",
			`data: {"delta":"hello "}`,
			"",
			"event: message",
			`data: {"delta":"world"}`,
			"",
			"event: message",
			`data: {"delta":`,
			"",
			"data: [DONE]",
			"",
		}, "\n")), nil
	})}

	target := cleanr.NewHTTPTarget(cleanr.TargetConfig{
		URL:           "https://example.test/http",
		Method:        http.MethodPost,
		ResponseField: "output.text",
		Stream:        true,
	}, client)

	resp := target.Invoke(context.Background(), cleanr.Request{Timeout: time.Second})
	if resp.Err != nil {
		t.Fatalf("unexpected response error: %v", resp.Err)
	}
	if resp.Text != "hello world" {
		t.Fatalf("unexpected stream text: %q", resp.Text)
	}
	if resp.Stream.ChunkCount != 4 || resp.Stream.ErrorCount != 1 || !resp.Stream.Recovered {
		t.Fatalf("unexpected stream metrics: %+v", resp.Stream)
	}
	if resp.Stream.CompletionState != "completed" {
		t.Fatalf("unexpected completion state: %+v", resp.Stream)
	}
}

func TestHTTPTargetInjectsTranscriptMessages(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["transcript"] == "" {
			t.Fatalf("expected transcript text in body: %#v", body)
		}
		messages, ok := body["messages"].([]any)
		if !ok || len(messages) != 3 {
			t.Fatalf("unexpected transcript messages: %#v", body["messages"])
		}
		return jsonResponse(t, http.StatusOK, map[string]any{
			"output": map[string]any{"text": "done"},
		}), nil
	})}

	target := cleanr.NewHTTPTarget(cleanr.TargetConfig{
		URL:           "https://example.test/http",
		Method:        http.MethodPost,
		PromptField:   "input",
		SystemField:   "system",
		ResponseField: "output.text",
	}, client)

	resp := target.Invoke(context.Background(), cleanr.BuildScenarioRequest(cleanr.Scenario{
		Name: "transcript",
		Turns: []cleanr.ConversationTurn{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi"},
		},
	}, time.Second))
	if resp.Err != nil || resp.Text != "done" {
		t.Fatalf("unexpected transcript response: %+v", resp)
	}
}

func TestHTTPTargetInvokeSupportsOpenAPIOverrides(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPatch {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.URL.String(); got != "https://example.test/api/tickets/t_123?verbose=true" {
			t.Fatalf("unexpected request URL: %s", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected content type: %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if string(body) != `{"status":"closed"}` {
			t.Fatalf("unexpected request body: %s", string(body))
		}
		return jsonResponse(t, http.StatusOK, map[string]any{
			"output": map[string]any{"text": "patched"},
		}), nil
	})}

	target := cleanr.NewHTTPTarget(cleanr.TargetConfig{
		Type:          "http",
		URL:           "https://example.test/api",
		Method:        http.MethodPost,
		ResponseField: "output.text",
		OpenAPI:       cleanr.OpenAPITargetConfig{Enabled: true},
	}, client)

	resp := target.Invoke(context.Background(), cleanr.Request{
		Scenario: cleanr.Scenario{
			Metadata: map[string]string{
				"openapi.method":       "PATCH",
				"openapi.path":         "/tickets/t_123",
				"openapi.query":        "verbose=true",
				"openapi.content_type": "application/json",
			},
		},
		Prompt: `{"status":"closed"}`,
	})
	if resp.Err != nil || resp.Text != "patched" {
		t.Fatalf("unexpected openapi override response: %+v", resp)
	}
}

func TestHTTPTargetInvokeHandlesTemplateCloneAndMarshalFailures(t *testing.T) {
	t.Parallel()

	target := cleanr.NewHTTPTarget(cleanr.TargetConfig{
		URL:           "http://example.invalid",
		Method:        http.MethodPost,
		PromptField:   "input",
		ResponseField: "output.text",
	}, &http.Client{})

	resp := target.Invoke(context.Background(), cleanr.Request{
		Template: map[string]any{
			"broken": func() {},
		},
	})
	if resp.Err == nil {
		t.Fatal("expected marshal failure for unsupported template value")
	}

	resp = target.Invoke(context.Background(), cleanr.Request{
		Template: map[string]any{
			"broken": invalidJSONValue{},
		},
	})
	if resp.Err == nil {
		t.Fatal("expected marshal failure after clone fallback")
	}
}

func TestHTTPTargetInvokeHandlesResponseFallbacks(t *testing.T) {
	t.Parallel()

	t.Run("plain text body falls back to raw body", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader("plain body")),
			}, nil
		})}

		target := cleanr.NewHTTPTarget(cleanr.TargetConfig{
			URL:           "https://example.test/plain",
			Method:        http.MethodPost,
			PromptField:   "input",
			ResponseField: "output.text",
		}, client)

		resp := target.Invoke(context.Background(), cleanr.Request{})
		if resp.Err != nil || resp.ExtractError != nil {
			t.Fatalf("unexpected errors: err=%v extract=%v", resp.Err, resp.ExtractError)
		}
		if resp.Text != "plain body" {
			t.Fatalf("unexpected fallback body: %q", resp.Text)
		}
	})

	t.Run("missing field reports extract error", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(t, http.StatusOK, map[string]any{
				"output": map[string]any{"other": "value"},
			}), nil
		})}

		target := cleanr.NewHTTPTarget(cleanr.TargetConfig{
			URL:           "https://example.test/missing",
			Method:        http.MethodPost,
			PromptField:   "input",
			ResponseField: "output.text",
		}, client)

		resp := target.Invoke(context.Background(), cleanr.Request{})
		if resp.ExtractError == nil {
			t.Fatalf("expected extract error for missing field, got nil")
		}
		if !strings.Contains(resp.ExtractError.Error(), "output.text") {
			t.Fatalf("expected extract error to name the missing field, got %v", resp.ExtractError)
		}
	})

	t.Run("object field is marshaled back to json", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(t, http.StatusOK, map[string]any{
				"output": map[string]any{
					"text": map[string]any{"nested": true},
				},
			}), nil
		})}

		target := cleanr.NewHTTPTarget(cleanr.TargetConfig{
			URL:           "https://example.test/object",
			Method:        http.MethodPost,
			PromptField:   "input",
			ResponseField: "output.text",
		}, client)

		resp := target.Invoke(context.Background(), cleanr.Request{})
		if resp.Text != `{"nested":true}` {
			t.Fatalf("unexpected object extraction: %q", resp.Text)
		}
	})
}

func TestTargetFactoryReturnsConcreteTargetsAndInvalidErrors(t *testing.T) {
	t.Parallel()

	client := &http.Client{}
	if _, ok := adapterspkg.NewTargetFromConfig(cleanr.TargetConfig{Type: "http"}, client).(*adapterspkg.HTTP); !ok {
		t.Fatal("expected http target")
	}
	if _, ok := adapterspkg.NewTargetFromConfig(cleanr.TargetConfig{Type: "openai"}, client).(*adapterspkg.OpenAI); !ok {
		t.Fatal("expected openai target")
	}
	if _, ok := adapterspkg.NewTargetFromConfig(cleanr.TargetConfig{Type: "openai_compatible"}, client).(*adapterspkg.OpenAI); !ok {
		t.Fatal("expected openai-compatible target")
	}
	if _, ok := adapterspkg.NewTargetFromConfig(cleanr.TargetConfig{Type: "azure_openai"}, client).(*adapterspkg.OpenAI); !ok {
		t.Fatal("expected azure openai target")
	}
	if _, ok := adapterspkg.NewTargetFromConfig(cleanr.TargetConfig{Type: "gemini"}, client).(*adapterspkg.OpenAI); !ok {
		t.Fatal("expected gemini target")
	}
	if _, ok := adapterspkg.NewTargetFromConfig(cleanr.TargetConfig{Type: "bedrock"}, client).(*adapterspkg.OpenAI); !ok {
		t.Fatal("expected bedrock target")
	}
	if _, ok := adapterspkg.NewTargetFromConfig(cleanr.TargetConfig{Type: "vertex"}, client).(*adapterspkg.OpenAI); !ok {
		t.Fatal("expected vertex target")
	}
	if _, ok := adapterspkg.NewTargetFromConfig(cleanr.TargetConfig{Type: "mistral"}, client).(*adapterspkg.OpenAI); !ok {
		t.Fatal("expected mistral target")
	}
	if _, ok := adapterspkg.NewTargetFromConfig(cleanr.TargetConfig{Type: "anthropic"}, client).(*adapterspkg.Anthropic); !ok {
		t.Fatal("expected anthropic target")
	}
	if _, ok := adapterspkg.NewTargetFromConfig(cleanr.TargetConfig{Type: "mcp"}, client).(*adapterspkg.MCP); !ok {
		t.Fatal("expected mcp target")
	}

	resp := adapterspkg.NewTargetFromConfig(cleanr.TargetConfig{Type: "bogus"}, client).Invoke(context.Background(), cleanr.Request{})
	if resp.Err == nil || !strings.Contains(resp.Err.Error(), `unsupported target type "bogus"`) {
		t.Fatalf("unexpected invalid target error: %v", resp.Err)
	}
}
