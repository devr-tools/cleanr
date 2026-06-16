package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type HTTP struct {
	cfg    core.TargetConfig
	client *http.Client
}

func NewHTTP(cfg core.TargetConfig, client *http.Client) *HTTP {
	return &HTTP{cfg: cfg, client: client}
}

func (t *HTTP) Invoke(ctx context.Context, req core.Request) core.Response {
	method := t.cfg.Method
	requestURL := t.cfg.URL
	data, headers, err := buildHTTPRequestPayload(req, t.cfg)
	if err != nil {
		return core.Response{Err: err}
	}
	if cfgMethod, cfgURL, ok := openAPIRequestOverride(req, t.cfg); ok {
		method = cfgMethod
		requestURL = cfgURL
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = t.cfg.Timeout()
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, method, requestURL, bytes.NewReader(data))
	if err != nil {
		return core.Response{Err: err}
	}
	for k, v := range t.cfg.Headers {
		httpReq.Header.Set(k, v)
	}
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if t.cfg.Stream && httpReq.Header.Get("Accept") == "" {
		httpReq.Header.Set("Accept", "text/event-stream")
	}

	start := time.Now()
	httpResp, err := t.client.Do(httpReq)
	latency := time.Since(start)
	if err != nil {
		return core.Response{Err: err, Latency: latency}
	}
	defer httpResp.Body.Close()

	if isSSEContentType(httpResp.Header.Get("Content-Type")) {
		return t.invokeStream(httpResp, latency)
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return core.Response{StatusCode: httpResp.StatusCode, Latency: latency, Err: err}
	}

	text, extractErr := extractResponseField(respBody, t.cfg.ResponseField)
	normalized, usage, stream := extractHTTPNormalized(respBody, httpResp.Status)
	return core.Response{
		StatusCode:   httpResp.StatusCode,
		Body:         respBody,
		Text:         text,
		Latency:      latency,
		Stream:       stream,
		ExtractError: extractErr,
		Usage:        usage,
		Normalized:   normalized,
	}
}

func (t *HTTP) invokeStream(httpResp *http.Response, latency time.Duration) core.Response {
	events, stream, err := parseSSEStream(httpResp.Body)
	if err != nil {
		return core.Response{StatusCode: httpResp.StatusCode, Latency: latency, Stream: stream, Err: err}
	}
	text := collectHTTPStreamText(events, t.cfg.ResponseField, &stream)
	stream.CompletionState = httpStreamCompletionState(events)
	body := marshalSSEEvents(events)
	normalized := core.ProviderResponse{
		Provider: "http",
		Status:   httpResp.Status,
		Raw: map[string]any{
			"stream_events": events,
		},
	}
	if stream.CompletionState == "completed" {
		markStreamParseRecovery(&stream)
	}
	return core.Response{
		StatusCode: httpResp.StatusCode,
		Body:       body,
		Text:       text,
		Latency:    latency,
		Stream:     stream,
		Normalized: normalized,
	}
}

func buildHTTPRequestPayload(req core.Request, cfg core.TargetConfig) ([]byte, map[string]string, error) {
	if cfg.TargetType() == "http" && cfg.OpenAPI.Enabled {
		if body, headers, ok, err := buildOpenAPIHTTPRequestPayload(req); ok || err != nil {
			return body, headers, err
		}
	}
	body := buildRequestBody(req, cfg)
	data, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}
	return data, nil, nil
}

func buildOpenAPIHTTPRequestPayload(req core.Request) ([]byte, map[string]string, bool, error) {
	if req.Scenario.Metadata == nil {
		return nil, nil, false, nil
	}
	if strings.TrimSpace(req.Scenario.Metadata["openapi.path"]) == "" && strings.TrimSpace(req.Scenario.Metadata["openapi.method"]) == "" {
		return nil, nil, false, nil
	}
	headers := map[string]string{}
	if contentType := strings.TrimSpace(req.Scenario.Metadata["openapi.content_type"]); contentType != "" {
		headers["Content-Type"] = contentType
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, headers, true, nil
	}
	if contentType := strings.ToLower(strings.TrimSpace(headers["Content-Type"])); contentType == "" || strings.Contains(contentType, "json") {
		var payload any
		if err := json.Unmarshal([]byte(req.Prompt), &payload); err != nil {
			return nil, nil, true, err
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, true, err
		}
		return data, headers, true, nil
	}
	return []byte(req.Prompt), headers, true, nil
}

func openAPIRequestOverride(req core.Request, cfg core.TargetConfig) (string, string, bool) {
	if !cfg.OpenAPI.Enabled || req.Scenario.Metadata == nil {
		return "", "", false
	}
	method := strings.ToUpper(strings.TrimSpace(req.Scenario.Metadata["openapi.method"]))
	pathValue := strings.TrimSpace(req.Scenario.Metadata["openapi.path"])
	if method == "" && pathValue == "" {
		return "", "", false
	}
	if method == "" {
		method = cfg.Method
	}
	requestURL := joinOpenAPIURL(cfg.URL, pathValue, strings.TrimSpace(req.Scenario.Metadata["openapi.query"]))
	return method, requestURL, true
}

func joinOpenAPIURL(baseURL, pathValue, rawQuery string) string {
	base, err := neturl.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	if absolute, err := neturl.Parse(pathValue); err == nil && absolute.Scheme != "" && absolute.Host != "" {
		if rawQuery != "" {
			absolute.RawQuery = rawQuery
		}
		return absolute.String()
	}
	joined := *base
	if pathValue != "" {
		if strings.HasSuffix(joined.Path, "/") {
			joined.Path = strings.TrimSuffix(joined.Path, "/")
		}
		if !strings.HasPrefix(pathValue, "/") {
			pathValue = "/" + pathValue
		}
		joined.Path += pathValue
	}
	if rawQuery != "" {
		joined.RawQuery = rawQuery
	}
	return joined.String()
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
		"transcript":    req.Scenario.TranscriptText(),
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
		return interpolateMapValue(typed, replacements, metadata, cfg, req)
	case []any:
		return interpolateSliceValue(typed, replacements, metadata, cfg, req)
	case string:
		return interpolateStringValue(typed, replacements)
	default:
		return typed
	}
}

func interpolateMapValue(typed map[string]any, replacements map[string]string, metadata map[string]string, cfg core.TargetConfig, req core.Request) any {
	for k, item := range typed {
		typed[k] = interpolateValue(item, replacements, metadata, cfg, req)
	}
	applyPromptFields(typed, cfg, req)
	applyTranscriptFields(typed, req)
	applyMetadataFields(typed, metadata)
	return typed
}

func interpolateSliceValue(typed []any, replacements map[string]string, metadata map[string]string, cfg core.TargetConfig, req core.Request) any {
	for i, item := range typed {
		typed[i] = interpolateValue(item, replacements, metadata, cfg, req)
	}
	return typed
}

func interpolateStringValue(value string, replacements map[string]string) string {
	out := value
	for key, replacement := range replacements {
		out = strings.ReplaceAll(out, "{{"+key+"}}", replacement)
	}
	return out
}

func applyPromptFields(payload map[string]any, cfg core.TargetConfig, req core.Request) {
	if cfg.PromptField != "" {
		payload[cfg.PromptField] = req.Prompt
	}
	if cfg.SystemField != "" {
		payload[cfg.SystemField] = req.System
	}
}

func applyTranscriptFields(payload map[string]any, req core.Request) {
	if len(req.Messages) == 0 {
		return
	}
	payload["messages"] = deepClone(req.Messages)
	payload["transcript"] = req.Scenario.TranscriptText()
}

func applyMetadataFields(payload map[string]any, metadata map[string]string) {
	if len(metadata) == 0 {
		return
	}
	metaMap, ok := payload["metadata"].(map[string]any)
	if !ok {
		metaMap = map[string]any{}
		payload["metadata"] = metaMap
	}
	for key, value := range metadata {
		metaMap[key] = value
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

func extractHTTPNormalized(body []byte, status string) (core.ProviderResponse, core.TokenUsage, core.StreamMetrics) {
	normalized := core.ProviderResponse{
		Provider: "http",
		Status:   status,
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return normalized, core.TokenUsage{}, core.StreamMetrics{}
	}

	trace := payload
	if nested, ok := payload["trace"].(map[string]any); ok {
		trace = nested
	}

	normalized.ID = stringValue(trace["id"])
	normalized.Model = stringValue(trace["model"])
	normalized.Role = stringValue(trace["role"])
	if provider := stringValue(trace["provider"]); provider != "" {
		normalized.Provider = provider
	}
	if providerStatus := stringValue(trace["status"]); providerStatus != "" {
		normalized.Status = providerStatus
	}
	normalized.FinishReason = stringValue(trace["finish_reason"])
	normalized.StopSequence = stringValue(trace["stop_sequence"])
	normalized.ToolCalls = decodeStructuredSlice[core.ToolCall](trace["tool_calls"])
	normalized.SourceUses = decodeStructuredSlice[core.SourceUse](trace["source_uses"])
	normalized.Approvals = decodeStructuredSlice[core.ApprovalArtifact](trace["approvals"])
	normalized.StateChanges = decodeStructuredSlice[core.StateChange](trace["state_changes"])
	normalized.MemoryOperations = decodeStructuredSlice[core.MemoryOperation](trace["memory_operations"])
	normalized.Raw = trace

	usagePayload := payload
	if nested, ok := payload["usage"].(map[string]any); ok {
		usagePayload = nested
	}
	usage := core.TokenUsage{
		InputTokens:  intValue(usagePayload["input_tokens"]),
		OutputTokens: intValue(usagePayload["output_tokens"]),
		TotalTokens:  intValue(usagePayload["total_tokens"]),
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	streamPayload := payload
	if nested, ok := payload["stream"].(map[string]any); ok {
		streamPayload = nested
	} else if nested, ok := trace["stream"].(map[string]any); ok {
		streamPayload = nested
	}
	stream := core.StreamMetrics{
		TTFTMS:          int64(intValue(streamPayload["ttft_ms"])),
		DurationMS:      int64(intValue(streamPayload["duration_ms"])),
		ChunkCount:      intValue(streamPayload["chunk_count"]),
		ErrorCount:      intValue(streamPayload["error_count"]),
		Recovered:       boolValue(streamPayload["recovered"]),
		CompletionState: stringValue(streamPayload["completion_state"]),
	}
	return normalized, usage, stream
}

func isSSEContentType(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/event-stream")
}

func collectHTTPStreamText(events []sseEvent, responseField string, metrics *core.StreamMetrics) string {
	parts := make([]string, 0, len(events))
	for _, event := range events {
		text, ok := extractHTTPStreamEventText(event.Data, responseField)
		if ok {
			parts = append(parts, text)
			continue
		}
		if shouldCountHTTPStreamParseError(event.Data, responseField) {
			metrics.ErrorCount++
		}
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}

func extractHTTPStreamEventText(data, responseField string) (string, bool) {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" || trimmed == "[DONE]" {
		return "", false
	}
	if responseField == "" {
		if text, ok := extractHTTPStreamJSONText(trimmed, "text", "delta", "content"); ok {
			return text, true
		}
		return trimmed, true
	}
	var payload any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return "", false
	}
	if text, err := extractResponseField([]byte(trimmed), responseField); err == nil && strings.TrimSpace(text) != "" {
		return text, true
	}
	if obj, ok := payload.(map[string]any); ok {
		if text, ok := extractHTTPStreamJSONTextFromObject(obj, "text", "delta", "content"); ok {
			return text, true
		}
	}
	return "", false
}

func shouldCountHTTPStreamParseError(data, responseField string) bool {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" || trimmed == "[DONE]" || !strings.HasPrefix(trimmed, "{") {
		return false
	}
	if responseField == "" {
		return false
	}
	return true
}

func extractHTTPStreamJSONText(data string, keys ...string) (string, bool) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return "", false
	}
	return extractHTTPStreamJSONTextFromObject(payload, keys...)
}

func extractHTTPStreamJSONTextFromObject(payload map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if value := stringValue(payload[key]); strings.TrimSpace(value) != "" {
			return value, true
		}
	}
	return "", false
}

func httpStreamCompletionState(events []sseEvent) string {
	if len(events) == 0 {
		return ""
	}
	for i := len(events) - 1; i >= 0; i-- {
		data := strings.TrimSpace(events[i].Data)
		name := strings.TrimSpace(events[i].Name)
		switch {
		case data == "[DONE]":
			return "completed"
		case strings.EqualFold(name, "error"):
			return "error"
		}
	}
	return "eof"
}

func marshalSSEEvents(events []sseEvent) []byte {
	data, err := json.Marshal(map[string]any{"events": events})
	if err != nil {
		return nil
	}
	return data
}

func decodeStructuredSlice[T any](value any) []T {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out []T
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func stringValue(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}

func intValue(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func boolValue(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}
