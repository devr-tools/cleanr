package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type GraphQL struct {
	cfg    core.TargetConfig
	client *http.Client
}

func NewGraphQL(cfg core.TargetConfig, client *http.Client) *GraphQL {
	return &GraphQL{cfg: cfg, client: client}
}

func (t *GraphQL) Invoke(ctx context.Context, req core.Request) core.Response {
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
	normalized := extractGraphQLNormalized(respBody, httpResp.Status, t.cfg.GraphQL.OperationName)
	return core.Response{
		StatusCode:   httpResp.StatusCode,
		Body:         respBody,
		Text:         text,
		Latency:      latency,
		ExtractError: extractErr,
		Normalized:   normalized,
	}
}

func (t *GraphQL) buildRequestBody(req core.Request) map[string]any {
	body := map[string]any{
		"query": t.cfg.GraphQL.Query,
	}
	if operation := strings.TrimSpace(t.cfg.GraphQL.OperationName); operation != "" {
		body["operationName"] = operation
	}
	if variables := graphQLVariables(req, t.cfg); variables != nil {
		body["variables"] = variables
	}
	return body
}

func graphQLVariables(req core.Request, cfg core.TargetConfig) any {
	template := req.Template
	if template == nil {
		template = cfg.GraphQL.VariablesTemplate
	}
	if template == nil {
		return nil
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

func extractGraphQLNormalized(body []byte, transportStatus, operationName string) core.ProviderResponse {
	normalized := core.ProviderResponse{
		Provider: "graphql",
		Status:   graphQLStatus(body),
		Raw: map[string]any{
			"transport_status": transportStatus,
		},
	}
	if strings.TrimSpace(operationName) != "" {
		normalized.Raw["operation_name"] = strings.TrimSpace(operationName)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		normalized.Status = transportStatus
		return normalized
	}
	if errorsValue, ok := payload["errors"]; ok {
		normalized.Raw["errors"] = errorsValue
	}
	if extensionsValue, ok := payload["extensions"]; ok {
		normalized.Raw["extensions"] = extensionsValue
	}
	return normalized
}

func graphQLStatus(body []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	_, hasData := payload["data"]
	errorsValue, hasErrors := payload["errors"]
	switch {
	case hasData && hasErrors && !graphQLErrorsEmpty(errorsValue):
		return "partial_error"
	case hasErrors && !graphQLErrorsEmpty(errorsValue):
		return "error"
	case hasData:
		return "ok"
	default:
		return ""
	}
}

func graphQLErrorsEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case []any:
		return len(typed) == 0
	default:
		return false
	}
}
