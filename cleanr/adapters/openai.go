package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
	profilepkg "github.com/devr-tools/cleanr/cleanr/profile"
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
	authHeader := t.cfg.OpenAI.AuthHeaderValue()
	if httpReq.Header.Get(authHeader) == "" {
		authValue := strings.TrimSpace(apiKey)
		if scheme := strings.TrimSpace(t.cfg.OpenAI.AuthSchemeValue()); scheme != "" {
			authValue = scheme + " " + authValue
		}
		httpReq.Header.Set(authHeader, authValue)
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

	if isSSEContentType(httpResp.Header.Get("Content-Type")) {
		return t.invokeStream(httpResp, latency)
	}

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
			"input": openAIResponsesInput(req, t.systemRole()),
		}
		if t.cfg.Stream {
			body["stream"] = true
		}
		if len(req.Messages) == 0 && strings.TrimSpace(req.System) != "" {
			body["instructions"] = req.System
		}
		if len(req.Scenario.Metadata) > 0 {
			body["metadata"] = req.Scenario.Metadata
		}
		return body, nil
	case "chat_completions":
		messages := openAIChatMessages(req, t.systemRole())
		body := map[string]any{
			"model":    t.cfg.OpenAI.Model,
			"messages": messages,
		}
		if t.cfg.Stream {
			body["stream"] = true
			body["stream_options"] = map[string]any{"include_usage": true}
		}
		if len(req.Scenario.Metadata) > 0 {
			body["metadata"] = req.Scenario.Metadata
		}
		return body, nil
	default:
		return nil, fmt.Errorf("unsupported openai api_mode %q", t.cfg.OpenAI.APIMode)
	}
}

func (t *OpenAI) systemRole() string {
	if t.cfg.TargetType() == "openai_compatible" {
		return "system"
	}
	return "developer"
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
		return parseOpenAIChatResponse(body, t.cfg.OpenAI.ProviderValue(t.cfg.TargetType()))
	default:
		return parseOpenAIResponsesResponse(body, t.cfg.OpenAI.ProviderValue(t.cfg.TargetType()))
	}
}

func (t *OpenAI) invokeStream(httpResp *http.Response, latency time.Duration) core.Response {
	events, stream, err := parseSSEStream(httpResp.Body)
	if err != nil {
		return core.Response{StatusCode: httpResp.StatusCode, Latency: latency, Stream: stream, Err: err}
	}

	text, normalized, usage, parseErr := t.parseStream(events, &stream)
	body := marshalSSEEvents(events)

	if httpResp.StatusCode >= 400 {
		return core.Response{
			StatusCode: httpResp.StatusCode,
			Body:       body,
			Text:       text,
			Latency:    latency,
			Stream:     stream,
			Usage:      usage,
			Normalized: normalized,
			Err:        openAIAPIError(body, httpResp.StatusCode),
		}
	}

	return core.Response{
		StatusCode:   httpResp.StatusCode,
		Body:         body,
		Text:         text,
		Latency:      latency,
		Stream:       stream,
		ExtractError: parseErr,
		Usage:        usage,
		Normalized:   normalized,
	}
}

func (t *OpenAI) parseStream(events []sseEvent, metrics *core.StreamMetrics) (string, core.ProviderResponse, core.TokenUsage, error) {
	switch t.cfg.OpenAI.APIModeValue() {
	case "chat_completions":
		return parseOpenAIChatStream(events, t.cfg.OpenAI.ProviderValue(t.cfg.TargetType()), metrics)
	default:
		return parseOpenAIResponsesStream(events, t.cfg.OpenAI.ProviderValue(t.cfg.TargetType()), metrics)
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

type openAIChatStreamEnvelope struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Delta        struct {
			Role      string `json:"role"`
			Content   any    `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		Message struct {
			Role string `json:"role"`
		} `json:"message"`
	} `json:"choices"`
	Usage openAIUsage `json:"usage"`
}

func parseOpenAIResponsesResponse(body []byte, provider string) (string, core.ProviderResponse, core.TokenUsage, error) {
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
		Provider:  provider,
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

func parseOpenAIChatResponse(body []byte, provider string) (string, core.ProviderResponse, core.TokenUsage, error) {
	var payload openAIChatEnvelope
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", core.ProviderResponse{}, core.TokenUsage{}, err
	}
	if len(payload.Choices) == 0 {
		normalized := core.ProviderResponse{
			Provider: provider,
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
		Provider:     provider,
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
		ID:         item.ID,
		CallID:     item.CallID,
		Type:       item.Type,
		Name:       item.Name,
		Arguments:  item.Args,
		ParsedArgs: decodeJSONString(item.Args),
		Status:     item.Status,
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
			ID:         item.ID,
			Type:       item.Type,
			Name:       item.Function.Name,
			Arguments:  item.Function.Arguments,
			ParsedArgs: decodeJSONString(item.Function.Arguments),
		})
	}
	return toolCalls
}

func parseOpenAIResponsesStream(events []sseEvent, provider string, metrics *core.StreamMetrics) (string, core.ProviderResponse, core.TokenUsage, error) {
	var (
		text       strings.Builder
		normalized = core.ProviderResponse{Provider: provider}
		usage      core.TokenUsage
		parseErr   error
	)

	for _, event := range events {
		data := strings.TrimSpace(event.Data)
		if data == "" {
			continue
		}
		switch {
		case data == "[DONE]":
			metrics.CompletionState = "completed"
		case event.Name == "response.output_text.delta":
			var payload struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				recordOpenAIStreamParseError(metrics, err, &parseErr)
				continue
			}
			text.WriteString(payload.Delta)
		case event.Name == "response.completed":
			var payload openAIResponsesEnvelope
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				recordOpenAIStreamParseError(metrics, err, &parseErr)
				continue
			}
			completedText, completedNormalized, completedUsage, err := parseOpenAIResponsesResponse([]byte(data), provider)
			if err != nil && parseErr == nil {
				parseErr = err
			}
			if text.Len() == 0 {
				text.WriteString(completedText)
			}
			normalized = completedNormalized
			usage = completedUsage
			metrics.CompletionState = payload.Status
		case strings.HasSuffix(event.Name, ".failed"):
			metrics.CompletionState = "error"
		}
	}

	finalText := strings.TrimSpace(text.String())
	if metrics.CompletionState == "" {
		metrics.CompletionState = fallbackOpenAIStreamCompletionState(events)
	}
	if metrics.CompletionState == "completed" {
		markStreamParseRecovery(metrics)
	}
	if finalText == "" && len(normalized.ToolCalls) == 0 && parseErr == nil {
		parseErr = io.EOF
	}
	return finalText, normalized, usage, parseErr
}

func parseOpenAIChatStream(events []sseEvent, provider string, metrics *core.StreamMetrics) (string, core.ProviderResponse, core.TokenUsage, error) {
	var (
		text       strings.Builder
		normalized = core.ProviderResponse{Provider: provider}
		usage      core.TokenUsage
		parseErr   error
		toolCalls  = map[int]*core.ToolCall{}
	)

	for _, event := range events {
		payload, ok := decodeOpenAIChatStreamEvent(event, metrics, &parseErr)
		if !ok {
			continue
		}
		applyOpenAIChatStreamEnvelope(&normalized, &usage, payload)
		applyOpenAIChatStreamChoices(payload.Choices, &normalized, &text, toolCalls)
	}

	finalizeOpenAIChatStream(&normalized, toolCalls, events, metrics)
	return finalizeOpenAIStreamResult(strings.TrimSpace(text.String()), normalized, usage, parseErr)
}

func decodeOpenAIChatStreamEvent(event sseEvent, metrics *core.StreamMetrics, parseErr *error) (*openAIChatStreamEnvelope, bool) {
	data := strings.TrimSpace(event.Data)
	switch data {
	case "":
		return nil, false
	case "[DONE]":
		metrics.CompletionState = "completed"
		return nil, false
	}
	var payload openAIChatStreamEnvelope
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		recordOpenAIStreamParseError(metrics, err, parseErr)
		return nil, false
	}
	return &payload, true
}

func applyOpenAIChatStreamEnvelope(normalized *core.ProviderResponse, usage *core.TokenUsage, payload *openAIChatStreamEnvelope) {
	if payload.ID != "" {
		normalized.ID = payload.ID
	}
	if payload.Model != "" {
		normalized.Model = payload.Model
	}
	if payload.Object != "" {
		normalized.Raw = map[string]any{"object": payload.Object}
	}
	if hasOpenAIUsage(payload.Usage) {
		*usage = tokenUsageFromOpenAI(payload.Usage)
	}
}

func applyOpenAIChatStreamChoices(choices []struct {
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
	Delta        struct {
		Role      string `json:"role"`
		Content   any    `json:"content"`
		ToolCalls []struct {
			Index    int    `json:"index"`
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"delta"`
	Message struct {
		Role string `json:"role"`
	} `json:"message"`
}, normalized *core.ProviderResponse, text *strings.Builder, toolCalls map[int]*core.ToolCall) {
	for _, choice := range choices {
		applyOpenAIChatStreamChoice(choice, normalized, text, toolCalls)
	}
}

func applyOpenAIChatStreamChoice(choice struct {
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
	Delta        struct {
		Role      string `json:"role"`
		Content   any    `json:"content"`
		ToolCalls []struct {
			Index    int    `json:"index"`
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"delta"`
	Message struct {
		Role string `json:"role"`
	} `json:"message"`
}, normalized *core.ProviderResponse, text *strings.Builder, toolCalls map[int]*core.ToolCall) {
	setOpenAIChatStreamRole(normalized, choice)
	if choice.FinishReason != "" {
		normalized.FinishReason = choice.FinishReason
	}
	if piece, err := parseChatMessageContent(choice.Delta.Content); err == nil && piece != "" {
		text.WriteString(piece)
	}
	for _, item := range choice.Delta.ToolCalls {
		mergeOpenAIChatStreamToolCall(toolCalls, item)
	}
}

func setOpenAIChatStreamRole(normalized *core.ProviderResponse, choice struct {
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
	Delta        struct {
		Role      string `json:"role"`
		Content   any    `json:"content"`
		ToolCalls []struct {
			Index    int    `json:"index"`
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"delta"`
	Message struct {
		Role string `json:"role"`
	} `json:"message"`
}) {
	if role := strings.TrimSpace(choice.Delta.Role); role != "" {
		normalized.Role = role
		return
	}
	if role := strings.TrimSpace(choice.Message.Role); role != "" {
		normalized.Role = role
	}
}

func mergeOpenAIChatStreamToolCall(toolCalls map[int]*core.ToolCall, item struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}) {
	call := toolCalls[item.Index]
	if call == nil {
		call = &core.ToolCall{}
		toolCalls[item.Index] = call
	}
	if item.ID != "" {
		call.ID = item.ID
	}
	if item.Type != "" {
		call.Type = item.Type
	}
	if item.Function.Name != "" {
		call.Name = item.Function.Name
	}
	call.Arguments += item.Function.Arguments
}

func finalizeOpenAIChatStream(normalized *core.ProviderResponse, toolCalls map[int]*core.ToolCall, events []sseEvent, metrics *core.StreamMetrics) {
	normalized.ToolCalls = orderedOpenAIStreamToolCalls(toolCalls)
	for i := range normalized.ToolCalls {
		normalized.ToolCalls[i].ParsedArgs = decodeJSONString(normalized.ToolCalls[i].Arguments)
	}
	if metrics.CompletionState == "" {
		metrics.CompletionState = fallbackOpenAIStreamCompletionState(events)
	}
	if metrics.CompletionState == "completed" {
		markStreamParseRecovery(metrics)
	}
}

func finalizeOpenAIStreamResult(finalText string, normalized core.ProviderResponse, usage core.TokenUsage, parseErr error) (string, core.ProviderResponse, core.TokenUsage, error) {
	if finalText == "" && len(normalized.ToolCalls) == 0 && parseErr == nil {
		parseErr = io.EOF
	}
	return finalText, normalized, usage, parseErr
}

func hasOpenAIUsage(usage openAIUsage) bool {
	return usage.TotalTokens > 0 ||
		usage.InputTokens > 0 ||
		usage.OutputTokens > 0 ||
		usage.PromptTokens > 0 ||
		usage.CompletionTokens > 0
}

func orderedOpenAIStreamToolCalls(items map[int]*core.ToolCall) []core.ToolCall {
	if len(items) == 0 {
		return nil
	}
	indices := make([]int, 0, len(items))
	for index := range items {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	toolCalls := make([]core.ToolCall, 0, len(indices))
	for _, index := range indices {
		if item := items[index]; item != nil {
			toolCalls = append(toolCalls, *item)
		}
	}
	return toolCalls
}

func recordOpenAIStreamParseError(metrics *core.StreamMetrics, err error, parseErr *error) {
	metrics.ErrorCount++
	if *parseErr == nil {
		*parseErr = err
	}
}

func fallbackOpenAIStreamCompletionState(events []sseEvent) string {
	for i := len(events) - 1; i >= 0; i-- {
		name := strings.TrimSpace(events[i].Name)
		data := strings.TrimSpace(events[i].Data)
		switch {
		case data == "[DONE]":
			return "completed"
		case strings.HasSuffix(name, ".completed"):
			return "completed"
		case strings.HasSuffix(name, ".failed"):
			return "error"
		}
	}
	return "eof"
}

func openAIChatMessages(req core.Request, systemRole string) []map[string]any {
	if len(req.Messages) == 0 {
		messages := make([]map[string]any, 0, 2)
		if strings.TrimSpace(req.System) != "" {
			messages = append(messages, map[string]any{
				"role":    systemRole,
				"content": req.System,
			})
		}
		messages = append(messages, map[string]any{
			"role":    "user",
			"content": req.Prompt,
		})
		return messages
	}

	messages := make([]map[string]any, 0, len(req.Messages))
	for _, turn := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(turn.Role))
		content := strings.TrimSpace(turn.Content)
		if role == "" || content == "" {
			continue
		}
		msg := map[string]any{
			"role":    role,
			"content": content,
		}
		switch role {
		case "system":
			msg["role"] = systemRole
		case "tool":
			msg["role"] = "tool"
			if turn.Name != "" {
				msg["name"] = turn.Name
			}
			if turn.ToolCallID != "" {
				msg["tool_call_id"] = turn.ToolCallID
			}
		}
		messages = append(messages, msg)
	}
	return messages
}

func openAIResponsesInput(req core.Request, systemRole string) any {
	if len(req.Messages) == 0 {
		return req.Prompt
	}
	return openAIChatMessages(req, systemRole)
}

func decodeJSONString(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var out any
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil
	}
	return out
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
