package tests

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"cleanr/cleanr"
	adapterspkg "cleanr/cleanr/adapters"
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
	if resp.Normalized.Provider != "http" {
		t.Fatalf("unexpected normalized payload: %+v", resp.Normalized)
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
		if !errors.Is(resp.ExtractError, io.EOF) {
			t.Fatalf("expected EOF extract error, got %v", resp.ExtractError)
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
	if _, ok := adapterspkg.NewTargetFromConfig(cleanr.TargetConfig{Type: "anthropic"}, client).(*adapterspkg.Anthropic); !ok {
		t.Fatal("expected anthropic target")
	}

	resp := adapterspkg.NewTargetFromConfig(cleanr.TargetConfig{Type: "bogus"}, client).Invoke(context.Background(), cleanr.Request{})
	if resp.Err == nil || !strings.Contains(resp.Err.Error(), `unsupported target type "bogus"`) {
		t.Fatalf("unexpected invalid target error: %v", resp.Err)
	}
}
