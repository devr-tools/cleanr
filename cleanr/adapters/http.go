package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"cleanr/cleanr/core"
)

type HTTP struct {
	cfg    core.TargetConfig
	client *http.Client
}

func NewHTTP(cfg core.TargetConfig, client *http.Client) *HTTP {
	return &HTTP{cfg: cfg, client: client}
}

func (t *HTTP) Invoke(ctx context.Context, req core.Request) core.Response {
	body := buildRequestBody(req, t.cfg)
	data, err := json.Marshal(body)
	if err != nil {
		return core.Response{Err: err}
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = t.cfg.Timeout()
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, t.cfg.Method, t.cfg.URL, bytes.NewReader(data))
	if err != nil {
		return core.Response{Err: err}
	}
	for k, v := range t.cfg.Headers {
		httpReq.Header.Set(k, v)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	start := time.Now()
	httpResp, err := t.client.Do(httpReq)
	latency := time.Since(start)
	if err != nil {
		return core.Response{Err: err, Latency: latency}
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return core.Response{StatusCode: httpResp.StatusCode, Latency: latency, Err: err}
	}

	text, extractErr := extractResponseField(respBody, t.cfg.ResponseField)
	return core.Response{
		StatusCode:   httpResp.StatusCode,
		Body:         respBody,
		Text:         text,
		Latency:      latency,
		ExtractError: extractErr,
		Normalized: core.ProviderResponse{
			Provider: "http",
			Status:   httpResp.Status,
		},
	}
}

func buildRequestBody(req core.Request, cfg core.TargetConfig) any {
	template := req.Template
	if template == nil {
		template = cfg.RequestTemplate
	}
	if template == nil {
		template = map[string]any{}
	}
	rendered := deepClone(template)
	replacements := map[string]string{
		"prompt":        req.Prompt,
		"system":        req.System,
		"scenario.name": req.Scenario.Name,
	}
	return interpolateValue(rendered, replacements, req.Scenario.Metadata, cfg, req)
}

func deepClone(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return v
	}
	return out
}

func interpolateValue(v any, replacements map[string]string, metadata map[string]string, cfg core.TargetConfig, req core.Request) any {
	switch typed := v.(type) {
	case map[string]any:
		for k, item := range typed {
			typed[k] = interpolateValue(item, replacements, metadata, cfg, req)
		}
		typed[cfg.PromptField] = req.Prompt
		if cfg.SystemField != "" {
			typed[cfg.SystemField] = req.System
		}
		if len(metadata) > 0 {
			if _, ok := typed["metadata"]; !ok {
				typed["metadata"] = map[string]any{}
			}
			if metaMap, ok := typed["metadata"].(map[string]any); ok {
				for k, v := range metadata {
					metaMap[k] = v
				}
			}
		}
		return typed
	case []any:
		for i, item := range typed {
			typed[i] = interpolateValue(item, replacements, metadata, cfg, req)
		}
		return typed
	case string:
		out := typed
		for key, value := range replacements {
			out = strings.ReplaceAll(out, "{{"+key+"}}", value)
		}
		return out
	default:
		return typed
	}
}

func extractResponseField(body []byte, path string) (string, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return string(body), nil
	}

	cur := payload
	for _, part := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return "", io.EOF
		}
		cur, ok = obj[part]
		if !ok {
			return "", io.EOF
		}
	}

	switch typed := cur.(type) {
	case string:
		return typed, nil
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}
