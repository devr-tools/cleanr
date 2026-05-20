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

	"cleanr/cleanr/core"
	profilepkg "cleanr/cleanr/profile"
)

type OpenAI struct {
	cfg    core.TargetConfig
	client *http.Client
}

func NewOpenAI(cfg core.TargetConfig, client *http.Client) *OpenAI {
	return &OpenAI{cfg: cfg, client: client}
}

func (t *OpenAI) Invoke(ctx context.Context, req core.Request) core.Response {
	apiKeyEnv := t.apiKeyEnv()
	apiKey, err := t.apiKey(apiKeyEnv)
	if err != nil {
		return core.Response{Err: err}
	}
	if apiKey == "" {
		return core.Response{Err: fmt.Errorf("openai api key env %q is not set", apiKeyEnv)}
	}

	body, err := t.buildRequestBody(req)
	if err != nil {
		return core.Response{Err: err}
	}
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
	for k, v := range t.cfg.Headers {
		httpReq.Header.Set(k, v)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if httpReq.Header.Get("Authorization") == "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if t.cfg.OpenAI.Organization != "" && httpReq.Header.Get("OpenAI-Organization") == "" {
		httpReq.Header.Set("OpenAI-Organization", t.cfg.OpenAI.Organization)
	}
	if t.cfg.OpenAI.Project != "" && httpReq.Header.Get("OpenAI-Project") == "" {
		httpReq.Header.Set("OpenAI-Project", t.cfg.OpenAI.Project)
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

	text, normalized, usage, parseErr := t.parseResponse(respBody)
	if httpResp.StatusCode >= 400 {
		return core.Response{
			StatusCode: httpResp.StatusCode,
			Body:       respBody,
			Text:       text,
			Latency:    latency,
			Usage:      usage,
			Normalized: normalized,
			Err:        openAIAPIError(respBody, httpResp.StatusCode),
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

func (t *OpenAI) buildRequestBody(req core.Request) (map[string]any, error) {
	switch t.cfg.OpenAI.APIModeValue() {
	case "responses":
		body := map[string]any{
			"model": t.cfg.OpenAI.Model,
			"input": req.Prompt,
		}
		if strings.TrimSpace(req.System) != "" {
			body["instructions"] = req.System
		}
		if len(req.Scenario.Metadata) > 0 {
			body["metadata"] = req.Scenario.Metadata
		}
		return body, nil
	case "chat_completions":
		messages := make([]map[string]any, 0, 2)
		if strings.TrimSpace(req.System) != "" {
			messages = append(messages, map[string]any{
				"role":    "developer",
				"content": req.System,
			})
		}
		messages = append(messages, map[string]any{
			"role":    "user",
			"content": req.Prompt,
		})
		body := map[string]any{
			"model":    t.cfg.OpenAI.Model,
			"messages": messages,
		}
		if len(req.Scenario.Metadata) > 0 {
			body["metadata"] = req.Scenario.Metadata
		}
		return body, nil
	default:
		return nil, fmt.Errorf("unsupported openai api_mode %q", t.cfg.OpenAI.APIMode)
	}
}

func (t *OpenAI) endpointURL() string {
	base := strings.TrimRight(t.cfg.OpenAI.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	switch t.cfg.OpenAI.APIModeValue() {
	case "chat_completions":
		return base + "/chat/completions"
	default:
		return base + "/responses"
	}
}

func (t *OpenAI) apiKeyEnv() string {
	if strings.TrimSpace(t.cfg.OpenAI.APIKeyEnv) == "" {
		return "OPENAI_API_KEY"
	}
	return strings.TrimSpace(t.cfg.OpenAI.APIKeyEnv)
}

func (t *OpenAI) apiKey(apiKeyEnv string) (string, error) {
	if apiKey := strings.TrimSpace(os.Getenv(apiKeyEnv)); apiKey != "" {
		return apiKey, nil
	}
	apiKey, err := profilepkg.LookupAPIKey("openai", apiKeyEnv)
	if err != nil {
		return "", fmt.Errorf("load stored openai api key: %w", err)
	}
	return apiKey, nil
}

func (t *OpenAI) parseResponse(body []byte) (string, core.ProviderResponse, core.TokenUsage, error) {
	switch t.cfg.OpenAI.APIModeValue() {
	case "chat_completions":
		return parseOpenAIChatResponse(body)
	default:
		return parseOpenAIResponsesResponse(body)
	}
}

type openAIUsage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	TotalTokens      int `json:"total_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type openAIErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

type openAIResponsesEnvelope struct {
	ID               string `json:"id"`
	Model            string `json:"model"`
	Status           string `json:"status"`
	IncompleteReason struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Output []struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Status  string `json:"status"`
		Role    string `json:"role"`
		CallID  string `json:"call_id"`
		Name    string `json:"name"`
		Args    string `json:"arguments"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Usage openAIUsage `json:"usage"`
}

type openAIChatEnvelope struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string `json:"role"`
			Content   any    `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage openAIUsage `json:"usage"`
}

func parseOpenAIResponsesResponse(body []byte) (string, core.ProviderResponse, core.TokenUsage, error) {
	var payload openAIResponsesEnvelope
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", core.ProviderResponse{}, core.TokenUsage{}, err
	}

	var parts []string
	toolCalls := make([]core.ToolCall, 0)
	for _, item := range payload.Output {
		for _, content := range item.Content {
			if content.Type == "output_text" && content.Text != "" {
				parts = append(parts, content.Text)
			}
		}
		if toolCall, ok := normalizeOpenAIResponsesToolCall(item); ok {
			toolCalls = append(toolCalls, toolCall)
		}
	}

	text := strings.TrimSpace(strings.Join(parts, "\n"))
	normalized := core.ProviderResponse{
		Provider:  "openai",
		ID:        payload.ID,
		Model:     payload.Model,
		Status:    payload.Status,
		ToolCalls: toolCalls,
	}
	if payload.IncompleteReason.Reason != "" {
		normalized.Raw = map[string]any{
			"incomplete_reason": payload.IncompleteReason.Reason,
		}
	}
	if text == "" && len(toolCalls) == 0 {
		return "", normalized, tokenUsageFromOpenAI(payload.Usage), io.EOF
	}
	return text, normalized, tokenUsageFromOpenAI(payload.Usage), nil
}

func parseOpenAIChatResponse(body []byte) (string, core.ProviderResponse, core.TokenUsage, error) {
	var payload openAIChatEnvelope
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", core.ProviderResponse{}, core.TokenUsage{}, err
	}
	if len(payload.Choices) == 0 {
		normalized := core.ProviderResponse{
			Provider: "openai",
			ID:       payload.ID,
			Model:    payload.Model,
		}
		if payload.Object != "" {
			normalized.Raw = map[string]any{"object": payload.Object}
		}
		return "", normalized, tokenUsageFromOpenAI(payload.Usage), io.EOF
	}

	choice := payload.Choices[0]
	toolCalls := normalizeOpenAIChatToolCalls(choice.Message.ToolCalls)
	text, err := parseChatMessageContent(choice.Message.Content)
	normalized := core.ProviderResponse{
		Provider:     "openai",
		ID:           payload.ID,
		Model:        payload.Model,
		Role:         choice.Message.Role,
		FinishReason: choice.FinishReason,
		ToolCalls:    toolCalls,
	}
	if payload.Object != "" {
		normalized.Raw = map[string]any{"object": payload.Object}
	}
	if err != nil && len(toolCalls) == 0 {
		return "", normalized, tokenUsageFromOpenAI(payload.Usage), err
	}
	if strings.TrimSpace(text) == "" && len(toolCalls) == 0 {
		return "", normalized, tokenUsageFromOpenAI(payload.Usage), io.EOF
	}
	return text, normalized, tokenUsageFromOpenAI(payload.Usage), nil
}

func parseChatMessageContent(content any) (string, error) {
	if content == nil {
		return "", nil
	}
	switch typed := content.(type) {
	case string:
		return typed, nil
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := obj["text"].(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n")), nil
	default:
		return "", fmt.Errorf("unsupported chat completion content type %T", content)
	}
}

func normalizeOpenAIResponsesToolCall(item struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Role    string `json:"role"`
	CallID  string `json:"call_id"`
	Name    string `json:"name"`
	Args    string `json:"arguments"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}) (core.ToolCall, bool) {
	if item.Type == "" || item.Type == "message" {
		return core.ToolCall{}, false
	}
	if item.Type == "reasoning" {
		return core.ToolCall{}, false
	}
	return core.ToolCall{
		ID:        item.ID,
		CallID:    item.CallID,
		Type:      item.Type,
		Name:      item.Name,
		Arguments: item.Args,
		Status:    item.Status,
		Raw: map[string]any{
			"role": item.Role,
		},
	}, true
}

func normalizeOpenAIChatToolCalls(items []struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}) []core.ToolCall {
	if len(items) == 0 {
		return nil
	}
	toolCalls := make([]core.ToolCall, 0, len(items))
	for _, item := range items {
		toolCalls = append(toolCalls, core.ToolCall{
			ID:        item.ID,
			Type:      item.Type,
			Name:      item.Function.Name,
			Arguments: item.Function.Arguments,
		})
	}
	return toolCalls
}

func tokenUsageFromOpenAI(usage openAIUsage) core.TokenUsage {
	inputTokens := usage.InputTokens
	outputTokens := usage.OutputTokens
	totalTokens := usage.TotalTokens

	if inputTokens == 0 && usage.PromptTokens > 0 {
		inputTokens = usage.PromptTokens
	}
	if outputTokens == 0 && usage.CompletionTokens > 0 {
		outputTokens = usage.CompletionTokens
	}
	if totalTokens == 0 {
		totalTokens = inputTokens + outputTokens
	}

	return core.TokenUsage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  totalTokens,
	}
}

func openAIAPIError(body []byte, statusCode int) error {
	var payload openAIErrorEnvelope
	if err := json.Unmarshal(body, &payload); err == nil && strings.TrimSpace(payload.Error.Message) != "" {
		return fmt.Errorf("openai api error (%d): %s", statusCode, payload.Error.Message)
	}
	return fmt.Errorf("openai api error (%d)", statusCode)
}
