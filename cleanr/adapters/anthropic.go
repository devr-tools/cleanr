package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
	profilepkg "github.com/devr-tools/cleanr/cleanr/profile"
)

type Anthropic struct {
	cfg    core.TargetConfig
	client *http.Client
}

func NewAnthropic(cfg core.TargetConfig, client *http.Client) *Anthropic {
	return &Anthropic{cfg: cfg, client: client}
}

func (t *Anthropic) Invoke(ctx context.Context, req core.Request) core.Response {
	apiKeyEnv := t.apiKeyEnv()
	apiKey, err := t.apiKey(apiKeyEnv)
	if err != nil {
		return core.Response{Err: err}
	}
	if apiKey == "" {
		return core.Response{Err: fmt.Errorf("anthropic api key env %q is not set", apiKeyEnv)}
	}

	body := t.buildRequestBody(req)
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

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, t.endpointURL(), bytes.NewReader(data))
	if err != nil {
		return core.Response{Err: err}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", t.cfg.Anthropic.VersionValue())
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

	text, normalized, usage, parseErr := parseAnthropicResponse(respBody)
	if httpResp.StatusCode >= 400 {
		return core.Response{
			StatusCode: httpResp.StatusCode,
			Body:       respBody,
			Text:       text,
			Latency:    latency,
			Usage:      usage,
			Normalized: normalized,
			Err:        anthropicAPIError(respBody, httpResp.StatusCode),
		}
	}

	return core.Response{
		StatusCode:   httpResp.StatusCode,
		Body:         respBody,
		Text:         text,
		Latency:      latency,
		ExtractError: parseErr,
		Usage:        usage,
		Normalized:   normalized,
	}
}

func (t *Anthropic) buildRequestBody(req core.Request) map[string]any {
	body := map[string]any{
		"model":      t.cfg.Anthropic.Model,
		"max_tokens": t.cfg.Anthropic.MaxTokensValue(),
		"messages":   anthropicMessages(req),
	}
	if system := anthropicSystem(req); system != "" {
		body["system"] = system
	}
	return body
}

func anthropicSystem(req core.Request) string {
	if len(req.Messages) == 0 {
		return strings.TrimSpace(req.System)
	}
	parts := make([]string, 0, len(req.Messages))
	for _, turn := range req.Messages {
		if strings.EqualFold(strings.TrimSpace(turn.Role), "system") {
			parts = append(parts, strings.TrimSpace(turn.Content))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func anthropicMessages(req core.Request) []map[string]any {
	if len(req.Messages) == 0 {
		return []map[string]any{{
			"role":    "user",
			"content": req.Prompt,
		}}
	}
	messages := make([]map[string]any, 0, len(req.Messages))
	for _, turn := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(turn.Role))
		content := strings.TrimSpace(turn.Content)
		if role == "" || content == "" || role == "system" {
			continue
		}
		msgRole := role
		switch role {
		case "assistant", "user":
		case "tool":
			msgRole = "user"
		default:
			msgRole = "user"
		}
		messages = append(messages, map[string]any{
			"role":    msgRole,
			"content": content,
		})
	}
	if len(messages) == 0 {
		return []map[string]any{{
			"role":    "user",
			"content": req.Prompt,
		}}
	}
	return messages
}

func (t *Anthropic) endpointURL() string {
	base := strings.TrimRight(t.cfg.Anthropic.BaseURL, "/")
	if base == "" {
		base = "https://api.anthropic.com/v1"
	}
	return base + "/messages"
}

func (t *Anthropic) apiKeyEnv() string {
	if strings.TrimSpace(t.cfg.Anthropic.APIKeyEnv) == "" {
		return "ANTHROPIC_API_KEY"
	}
	return strings.TrimSpace(t.cfg.Anthropic.APIKeyEnv)
}

func (t *Anthropic) apiKey(apiKeyEnv string) (string, error) {
	if apiKey := strings.TrimSpace(os.Getenv(apiKeyEnv)); apiKey != "" {
		return apiKey, nil
	}
	apiKey, err := profilepkg.LookupAPIKey("anthropic", apiKeyEnv)
	if err != nil {
		return "", fmt.Errorf("load stored anthropic api key: %w", err)
	}
	return apiKey, nil
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type anthropicMessageEnvelope struct {
	ID           string `json:"id"`
	Model        string `json:"model"`
	Role         string `json:"role"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Content      []struct {
		Type  string `json:"type"`
		Text  string `json:"text"`
		ID    string `json:"id"`
		Name  string `json:"name"`
		Input any    `json:"input"`
	} `json:"content"`
	Usage anthropicUsage `json:"usage"`
}

func parseAnthropicResponse(body []byte) (string, core.ProviderResponse, core.TokenUsage, error) {
	var payload anthropicMessageEnvelope
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", core.ProviderResponse{}, core.TokenUsage{}, err
	}

	parts := make([]string, 0, len(payload.Content))
	toolCalls := make([]core.ToolCall, 0)
	for _, item := range payload.Content {
		if item.Type == "text" && item.Text != "" {
			parts = append(parts, item.Text)
		}
		if item.Type == "tool_use" {
			toolCalls = append(toolCalls, core.ToolCall{
				ID:    item.ID,
				Type:  item.Type,
				Name:  item.Name,
				Input: item.Input,
			})
		}
	}

	text := strings.TrimSpace(strings.Join(parts, "\n"))
	usage := core.TokenUsage{
		InputTokens:  payload.Usage.InputTokens,
		OutputTokens: payload.Usage.OutputTokens,
		TotalTokens:  payload.Usage.InputTokens + payload.Usage.OutputTokens,
	}
	normalized := core.ProviderResponse{
		Provider:     "anthropic",
		ID:           payload.ID,
		Model:        payload.Model,
		Role:         payload.Role,
		FinishReason: payload.StopReason,
		StopSequence: payload.StopSequence,
		ToolCalls:    toolCalls,
	}
	if text == "" && len(toolCalls) == 0 {
		return "", normalized, usage, io.EOF
	}
	return text, normalized, usage, nil
}

func anthropicAPIError(body []byte, statusCode int) error {
	var payload anthropicErrorEnvelope
	if err := json.Unmarshal(body, &payload); err == nil && strings.TrimSpace(payload.Error.Message) != "" {
		return fmt.Errorf("anthropic api error (%d): %s", statusCode, payload.Error.Message)
	}
	return fmt.Errorf("anthropic api error (%d)", statusCode)
}
