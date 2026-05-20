package mcpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"cleanr/cleanr"
	"cleanr/internal/version"
)

const protocolVersion = "2025-06-18"

const (
	jsonRPCParseError     = -32700
	jsonRPCInvalidRequest = -32600
	jsonRPCMethodNotFound = -32601
	jsonRPCInvalidParams  = -32602
	jsonRPCInternalError  = -32603
	jsonRPCServerError    = -32000
)

type Server struct {
	initialized bool
	tools       []toolDefinition
}

type requestEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type responseEnvelope struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Result  any            `json:"result,omitempty"`
	Error   *errorEnvelope `json:"error,omitempty"`
}

type errorEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      map[string]any `json:"clientInfo"`
}

type toolsListParams struct {
	Cursor string `json:"cursor"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type toolDefinition struct {
	Name         string         `json:"name"`
	Title        string         `json:"title,omitempty"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content           []toolContent `json:"content"`
	StructuredContent any           `json:"structuredContent,omitempty"`
	IsError           bool          `json:"isError,omitempty"`
}

type configSource struct {
	Config     string `json:"config"`
	ConfigPath string `json:"config_path"`
	Format     string `json:"format"`
}

type exampleConfigArgs struct {
	Format string `json:"format"`
}

type runArgs struct {
	Config     string `json:"config"`
	ConfigPath string `json:"config_path"`
	Format     string `json:"format"`
	ReportType string `json:"report_format"`
	TimeoutMS  int    `json:"timeout_ms"`
}

type exampleConfigOutput struct {
	Format string `json:"format"`
	Config string `json:"config"`
}

type validateConfigOutput struct {
	Valid         bool     `json:"valid"`
	TargetName    string   `json:"target_name,omitempty"`
	ScenarioCount int      `json:"scenario_count,omitempty"`
	Errors        []string `json:"errors,omitempty"`
}

type runOutput struct {
	Passed       bool          `json:"passed"`
	ExitCode     int           `json:"exit_code"`
	TargetName   string        `json:"target_name,omitempty"`
	ReportFormat string        `json:"report_format"`
	ReportText   string        `json:"report_text"`
	DurationMS   int64         `json:"duration_ms,omitempty"`
	Report       cleanr.Report `json:"report,omitempty"`
	Error        string        `json:"error,omitempty"`
}

func New() *Server {
	return &Server{
		tools: []toolDefinition{
			{
				Name:        "cleanr_example_config",
				Title:       "Generate cleanr example config",
				Description: "Return a starter cleanr config in JSON or YAML for agent editing.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"format": map[string]any{
							"type":        "string",
							"description": "Config format to generate.",
							"enum":        []string{"json", "yaml"},
						},
					},
				},
				OutputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"format": map[string]any{"type": "string"},
						"config": map[string]any{"type": "string"},
					},
					"required": []string{"format", "config"},
				},
			},
			{
				Name:        "cleanr_validate_config",
				Title:       "Validate cleanr config",
				Description: "Validate a cleanr config provided inline or by local path.",
				InputSchema: configSourceSchema(),
				OutputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"valid":          map[string]any{"type": "boolean"},
						"target_name":    map[string]any{"type": "string"},
						"scenario_count": map[string]any{"type": "integer"},
						"errors": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
					},
					"required": []string{"valid"},
				},
			},
			{
				Name:        "cleanr_run",
				Title:       "Run cleanr suites",
				Description: "Execute cleanr against a config provided inline or by local path and return the report.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"config": map[string]any{
							"type":        "string",
							"description": "Raw cleanr config content.",
						},
						"config_path": map[string]any{
							"type":        "string",
							"description": "Local path to a cleanr config file.",
						},
						"format": map[string]any{
							"type":        "string",
							"description": "Config format when config is provided inline.",
							"enum":        []string{"json", "yaml"},
						},
						"report_format": map[string]any{
							"type":        "string",
							"description": "Rendered report format.",
							"enum":        []string{"text", "json", "junit"},
						},
						"timeout_ms": map[string]any{
							"type":        "integer",
							"description": "Optional overall run timeout in milliseconds.",
							"minimum":     0,
						},
					},
				},
				OutputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"passed":        map[string]any{"type": "boolean"},
						"exit_code":     map[string]any{"type": "integer"},
						"target_name":   map[string]any{"type": "string"},
						"report_format": map[string]any{"type": "string"},
						"report_text":   map[string]any{"type": "string"},
						"duration_ms":   map[string]any{"type": "integer"},
						"report":        map[string]any{"type": "object"},
						"error":         map[string]any{"type": "string"},
					},
					"required": []string{"passed", "exit_code", "report_format", "report_text"},
				},
			},
		},
	}
}

func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		resp := s.HandleLine(ctx, line)
		if resp == nil {
			continue
		}
		if err := writeMessage(out, resp); err != nil {
			return err
		}
	}
}

func (s *Server) HandleLine(ctx context.Context, line []byte) *responseEnvelope {
	var req requestEnvelope
	if err := json.Unmarshal(line, &req); err != nil {
		return errorResponse(nil, jsonRPCParseError, "Parse error", nil)
	}
	return s.HandleRequest(ctx, req)
}

func (s *Server) HandleRequest(ctx context.Context, req requestEnvelope) *responseEnvelope {
	if req.JSONRPC != "2.0" {
		return errorResponse(req.ID, jsonRPCInvalidRequest, "invalid JSON-RPC version", nil)
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		s.initialized = true
		return nil
	case "ping":
		return successResponse(req.ID, map[string]any{})
	case "tools/list":
		if errResp := s.requireInitialized(req.ID); errResp != nil {
			return errResp
		}
		return s.handleToolsList(req)
	case "tools/call":
		if errResp := s.requireInitialized(req.ID); errResp != nil {
			return errResp
		}
		return s.handleToolCall(ctx, req)
	default:
		if strings.HasPrefix(req.Method, "notifications/") {
			return nil
		}
		return errorResponse(req.ID, jsonRPCMethodNotFound, fmt.Sprintf("method not found: %s", req.Method), nil)
	}
}

func (s *Server) handleInitialize(req requestEnvelope) *responseEnvelope {
	var params initializeParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return errorResponse(req.ID, jsonRPCInvalidParams, "invalid initialize params", nil)
		}
	}

	negotiatedVersion := protocolVersion
	if strings.TrimSpace(params.ProtocolVersion) == protocolVersion {
		negotiatedVersion = params.ProtocolVersion
	}

	return successResponse(req.ID, map[string]any{
		"protocolVersion": negotiatedVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{
				"listChanged": false,
			},
		},
		"serverInfo": map[string]any{
			"name":    "cleanr",
			"title":   "cleanr MCP Server",
			"version": version.Number,
		},
		"instructions": "Use cleanr_example_config to scaffold configs, cleanr_validate_config before execution, and cleanr_run to run suites and inspect reports.",
	})
}

func (s *Server) handleToolsList(req requestEnvelope) *responseEnvelope {
	return successResponse(req.ID, map[string]any{
		"tools": s.tools,
	})
}

func (s *Server) handleToolCall(ctx context.Context, req requestEnvelope) *responseEnvelope {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, jsonRPCInvalidParams, "invalid tool call params", nil)
	}
	if strings.TrimSpace(params.Name) == "" {
		return errorResponse(req.ID, jsonRPCInvalidParams, "tool name is required", nil)
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}

	var (
		result toolResult
		err    error
	)

	switch params.Name {
	case "cleanr_example_config":
		result, err = s.callExampleConfig(params.Arguments)
	case "cleanr_validate_config":
		result, err = s.callValidateConfig(params.Arguments)
	case "cleanr_run":
		result, err = s.callRun(ctx, params.Arguments)
	default:
		return errorResponse(req.ID, jsonRPCInvalidParams, fmt.Sprintf("unknown tool: %s", params.Name), nil)
	}
	if err != nil {
		return errorResponse(req.ID, jsonRPCInternalError, err.Error(), nil)
	}
	return successResponse(req.ID, result)
}

func (s *Server) callExampleConfig(args map[string]any) (toolResult, error) {
	var input exampleConfigArgs
	if err := decodeArgs(args, &input); err != nil {
		return toolResult{}, err
	}

	format := normalizeConfigFormat(input.Format)
	data, err := cleanr.MarshalConfig(cleanr.ExampleConfig(), format)
	if err != nil {
		return toolResult{}, err
	}

	out := exampleConfigOutput{
		Format: format,
		Config: string(data),
	}
	return structuredToolResult(out, out.Config), nil
}

func (s *Server) callValidateConfig(args map[string]any) (toolResult, error) {
	var input configSource
	if err := decodeArgs(args, &input); err != nil {
		return toolResult{}, err
	}

	cfg, err := loadConfigSource(input)
	if err != nil {
		out := validateConfigOutput{
			Valid:  false,
			Errors: []string{err.Error()},
		}
		return structuredToolResult(out, strings.Join(out.Errors, "\n")), nil
	}

	out := validateConfigOutput{
		Valid:         true,
		TargetName:    cfg.Target.Name,
		ScenarioCount: len(cfg.Scenarios),
	}
	return structuredToolResult(out, fmt.Sprintf("valid config for %s with %d scenarios", out.TargetName, out.ScenarioCount)), nil
}

func (s *Server) callRun(ctx context.Context, args map[string]any) (toolResult, error) {
	var input runArgs
	if err := decodeArgs(args, &input); err != nil {
		return toolResult{}, err
	}

	cfg, err := loadConfigSource(configSource{
		Config:     input.Config,
		ConfigPath: input.ConfigPath,
		Format:     input.Format,
	})
	if err != nil {
		out := runOutput{
			Passed:       false,
			ExitCode:     2,
			ReportFormat: normalizeReportFormat(input.ReportType),
			ReportText:   err.Error(),
			Error:        err.Error(),
		}
		return structuredToolResult(out, out.ReportText), nil
	}

	runCtx := ctx
	if input.TimeoutMS > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(input.TimeoutMS)*time.Millisecond)
		defer cancel()
	}

	report := cleanr.NewConfigRunner(cfg).Run(runCtx)
	reportFormat := normalizeReportFormat(input.ReportType)
	var buf bytes.Buffer
	if err := cleanr.WriteReport(&buf, report, reportFormat); err != nil {
		return toolResult{}, err
	}

	exitCode := 0
	if !report.Passed {
		exitCode = 1
	}
	out := runOutput{
		Passed:       report.Passed,
		ExitCode:     exitCode,
		TargetName:   report.Name,
		ReportFormat: reportFormat,
		ReportText:   strings.TrimRight(buf.String(), "\n"),
		DurationMS:   report.Duration.Milliseconds(),
		Report:       report,
	}
	return structuredToolResult(out, out.ReportText), nil
}

func (s *Server) requireInitialized(id any) *responseEnvelope {
	if s.initialized {
		return nil
	}
	return errorResponse(id, jsonRPCServerError, "server not initialized", nil)
}

func decodeArgs(args map[string]any, dest any) error {
	raw, err := json.Marshal(args)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return err
	}
	return nil
}

func loadConfigSource(src configSource) (cleanr.Config, error) {
	if strings.TrimSpace(src.ConfigPath) != "" {
		return cleanr.LoadConfigFile(strings.TrimSpace(src.ConfigPath))
	}
	if strings.TrimSpace(src.Config) == "" {
		return cleanr.Config{}, fmt.Errorf("provide config or config_path")
	}
	return cleanr.LoadConfigData([]byte(src.Config), normalizeConfigFormat(src.Format))
}

func structuredToolResult(v any, text string) toolResult {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		trimmed = "{}"
	}
	return toolResult{
		Content: []toolContent{{
			Type: "text",
			Text: trimmed,
		}},
		StructuredContent: v,
	}
}

func normalizeConfigFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "yaml", "yml":
		return "yaml"
	default:
		return "json"
	}
}

func normalizeReportFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return "json"
	case "junit":
		return "junit"
	default:
		return "text"
	}
}

func successResponse(id any, result any) *responseEnvelope {
	return &responseEnvelope{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func errorResponse(id any, code int, message string, data any) *responseEnvelope {
	return &responseEnvelope{
		JSONRPC: "2.0",
		ID:      id,
		Error: &errorEnvelope{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func writeMessage(w io.Writer, msg *responseEnvelope) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func configSourceSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"config": map[string]any{
				"type":        "string",
				"description": "Raw cleanr config content.",
			},
			"config_path": map[string]any{
				"type":        "string",
				"description": "Local path to a cleanr config file.",
			},
			"format": map[string]any{
				"type":        "string",
				"description": "Config format when config is provided inline.",
				"enum":        []string{"json", "yaml"},
			},
		},
	}
}

func Run(ctx context.Context) error {
	return New().Serve(ctx, os.Stdin, os.Stdout)
}
