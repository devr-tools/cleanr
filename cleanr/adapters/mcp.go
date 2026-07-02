package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type MCP struct {
	cfg          core.TargetConfig
	client       *http.Client
	initMu       sync.Mutex
	initialized  bool
	requestIDSeq int64
	mu           sync.Mutex
}

func NewMCP(cfg core.TargetConfig, client *http.Client) *MCP {
	if !cfg.MCP.Initialize {
		cfg.MCP.Initialize = true
	}
	return &MCP{cfg: cfg, client: client}
}

func (t *MCP) Invoke(ctx context.Context, req core.Request) core.Response {
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = t.cfg.Timeout()
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	if t.cfg.MCP.Initialize {
		if err := t.initialize(reqCtx); err != nil {
			return core.Response{Err: err, Latency: time.Since(start)}
		}
	}

	args := buildMCPArguments(req, t.cfg)
	rpcReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      t.nextRequestID(),
		"method":  "tools/call",
		"params": map[string]any{
			"name":      t.cfg.MCP.Tool,
			"arguments": args,
		},
	}
	respBody, statusCode, err := t.postJSONRPC(reqCtx, rpcReq)
	latency := time.Since(start)
	if err != nil {
		return core.Response{Err: err, Latency: latency, StatusCode: statusCode}
	}

	text, normalized, body, parseErr := parseMCPToolResponse(respBody, t.cfg.MCP.ResultTextPath, t.cfg.MCP.Tool, args)
	return core.Response{
		StatusCode:   statusCode,
		Body:         body,
		Text:         text,
		Latency:      latency,
		ExtractError: parseErr,
		Normalized:   normalized,
	}
}

// initialize performs the MCP handshake at most once per adapter. A failed
// attempt is NOT cached: the next Invoke retries, so a transient failure (a
// restarting server, the first scenario's deadline expiring mid-handshake)
// fails that one scenario instead of poisoning every remaining scenario in
// the run. The lock serializes concurrent first invokes, matching the
// blocking behavior sync.Once had.
func (t *MCP) initialize(ctx context.Context) error {
	t.initMu.Lock()
	defer t.initMu.Unlock()
	if t.initialized {
		return nil
	}
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      t.nextRequestID(),
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "cleanr",
				"version": "v1alpha1",
			},
		},
	}
	if _, _, err := t.postJSONRPC(ctx, initReq); err != nil {
		return fmt.Errorf("initialize mcp target: %w", err)
	}
	notify := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	if _, _, err := t.postJSONRPC(ctx, notify); err != nil {
		return fmt.Errorf("notify initialized mcp target: %w", err)
	}
	t.initialized = true
	return nil
}

func (t *MCP) nextRequestID() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.requestIDSeq++
	return t.requestIDSeq
}

func (t *MCP) postJSONRPC(ctx context.Context, payload any) ([]byte, int, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.cfg.MCP.URL, bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range t.cfg.Headers {
		httpReq.Header.Set(k, v)
	}
	for k, v := range t.cfg.MCP.Headers {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, 0, err
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, httpResp.StatusCode, err
	}
	if httpResp.StatusCode >= 400 {
		return body, httpResp.StatusCode, fmt.Errorf("mcp target http error (%d)", httpResp.StatusCode)
	}
	return body, httpResp.StatusCode, nil
}

type mcpRPCResponse struct {
	Result map[string]any `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func buildMCPArguments(req core.Request, cfg core.TargetConfig) map[string]any {
	template := cfg.MCP.ArgumentsTemplate
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
	args, ok := interpolateValue(rendered, replacements, req.Scenario.Metadata, cfg, req).(map[string]any)
	if !ok {
		return map[string]any{}
	}
	delete(args, cfg.PromptField)
	delete(args, cfg.SystemField)
	return args
}

func parseMCPToolResponse(raw []byte, textPath string, tool string, args map[string]any) (string, core.ProviderResponse, []byte, error) {
	var payload mcpRPCResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", core.ProviderResponse{}, raw, err
	}
	if payload.Error != nil {
		return "", core.ProviderResponse{
			Provider: "mcp",
			ToolCalls: []core.ToolCall{{
				Name:       tool,
				Type:       "mcp_tool",
				ParsedArgs: args,
			}},
		}, raw, fmt.Errorf("mcp rpc error (%d): %s", payload.Error.Code, payload.Error.Message)
	}

	result := payload.Result
	body := result
	if structured, ok := result["structuredContent"].(map[string]any); ok {
		body = structured
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		bodyBytes = raw
	}
	text, extractErr := extractMCPText(bodyBytes, result, textPath)
	normalized := core.ProviderResponse{
		Provider: "mcp",
		Status:   "ok",
		ToolCalls: []core.ToolCall{{
			Name:       tool,
			Type:       "mcp_tool",
			ParsedArgs: args,
			Raw:        map[string]any{"arguments": args},
		}},
		Raw: result,
	}
	if isErr, ok := result["isError"].(bool); ok && isErr {
		normalized.Status = "tool_error"
	}
	return text, normalized, bodyBytes, extractErr
}

func extractMCPText(body []byte, result map[string]any, textPath string) (string, error) {
	if strings.TrimSpace(textPath) != "" {
		return extractResponseField(body, textPath)
	}
	if content, ok := result["content"].([]any); ok {
		parts := make([]string, 0, len(content))
		for _, item := range content {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := obj["text"].(string); ok && strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			return strings.TrimSpace(strings.Join(parts, "\n")), nil
		}
	}
	return string(body), nil
}
