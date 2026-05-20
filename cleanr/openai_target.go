package cleanr

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
)

type OpenAITarget struct {
	cfg    TargetConfig
	client *http.Client
}

func NewOpenAITarget(cfg TargetConfig, client *http.Client) *OpenAITarget {
	return &OpenAITarget{cfg: cfg, client: client}
}

func (t *OpenAITarget) Invoke(ctx context.Context, req Request) Response {
	apiKeyEnv := t.apiKeyEnv()
	apiKey := strings.TrimSpace(os.Getenv(apiKeyEnv))
	if apiKey == "" {
		return Response{Err: fmt.Errorf("openai api key env %q is not set", apiKeyEnv)}
	}

	body, err := t.buildRequestBody(req)
	if err != nil {
		return Response{Err: err}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return Response{Err: err}
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = t.cfg.Timeout()
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, t.endpointURL(), bytes.NewReader(data))
	if err != nil {
		return Response{Err: err}
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
		return Response{Err: err, Latency: latency}
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{StatusCode: httpResp.StatusCode, Latency: latency, Err: err}
	}

	text, usage, parseErr := t.parseResponse(respBody)
	if httpResp.StatusCode >= 400 {
		return Response{
			StatusCode: httpResp.StatusCode,
			Body:       respBody,
			Text:       text,
			Latency:    latency,
			Usage:      usage,
			Err:        openAIAPIError(respBody, httpResp.StatusCode),
		}
	}

	return Response{
		StatusCode:   httpResp.StatusCode,
		Body:         respBody,
		Text:         text,
		Latency:      latency,
		ExtractError: parseErr,
		Usage:        usage,
	}
}

func (t *OpenAITarget) buildRequestBody(req Request) (map[string]any, error) {
	switch t.cfg.OpenAI.apiMode() {
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

func (t *OpenAITarget) endpointURL() string {
	base := strings.TrimRight(t.cfg.OpenAI.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	switch t.cfg.OpenAI.apiMode() {
	case "chat_completions":
		return base + "/chat/completions"
	default:
		return base + "/responses"
	}
}

func (t *OpenAITarget) apiKeyEnv() string {
	if strings.TrimSpace(t.cfg.OpenAI.APIKeyEnv) == "" {
		return "OPENAI_API_KEY"
	}
	return strings.TrimSpace(t.cfg.OpenAI.APIKeyEnv)
}

func (t *OpenAITarget) parseResponse(body []byte) (string, TokenUsage, error) {
	switch t.cfg.OpenAI.apiMode() {
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
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Usage openAIUsage `json:"usage"`
}

type openAIChatEnvelope struct {
	Choices []struct {
		Message struct {
			Content any `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage openAIUsage `json:"usage"`
}

func parseOpenAIResponsesResponse(body []byte) (string, TokenUsage, error) {
	var payload openAIResponsesEnvelope
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", TokenUsage{}, err
	}

	var parts []string
	for _, item := range payload.Output {
		for _, content := range item.Content {
			if content.Type == "output_text" && content.Text != "" {
				parts = append(parts, content.Text)
			}
		}
	}

	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if text == "" {
		return "", tokenUsageFromOpenAI(payload.Usage), io.EOF
	}
	return text, tokenUsageFromOpenAI(payload.Usage), nil
}

func parseOpenAIChatResponse(body []byte) (string, TokenUsage, error) {
	var payload openAIChatEnvelope
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", TokenUsage{}, err
	}
	if len(payload.Choices) == 0 {
		return "", tokenUsageFromOpenAI(payload.Usage), io.EOF
	}

	text, err := parseChatMessageContent(payload.Choices[0].Message.Content)
	if err != nil {
		return "", tokenUsageFromOpenAI(payload.Usage), err
	}
	if strings.TrimSpace(text) == "" {
		return "", tokenUsageFromOpenAI(payload.Usage), io.EOF
	}
	return text, tokenUsageFromOpenAI(payload.Usage), nil
}

func parseChatMessageContent(content any) (string, error) {
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

func tokenUsageFromOpenAI(usage openAIUsage) TokenUsage {
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

	return TokenUsage{
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
