package runtime

import (
	"context"
	"encoding/json"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/mcpserver/toolkit"
)

func RunDefinition() toolkit.Definition {
	return toolkit.Definition{
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
					"enum":        []string{"text", "json", "junit", "sarif", "agent"},
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
	}
}

func RenderReportDefinition() toolkit.Definition {
	return toolkit.Definition{
		Name:        "cleanr_render_report",
		Title:       "Render cleanr report",
		Description: "Render a JSON cleanr report as text, json, junit, sarif, or agent.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"report_json": map[string]any{
					"type":        "string",
					"description": "A serialized cleanr report JSON object.",
				},
				"format": map[string]any{
					"type":        "string",
					"description": "Render format.",
					"enum":        []string{"text", "json", "junit", "sarif", "agent"},
				},
			},
			"required": []string{"report_json"},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"format":   map[string]any{"type": "string"},
				"rendered": map[string]any{"type": "string"},
			},
			"required": []string{"format", "rendered"},
		},
	}
}

func Run(ctx context.Context, args map[string]any) (toolkit.Result, error) {
	var input toolkit.RunArgs
	if err := toolkit.DecodeArgs(args, &input); err != nil {
		return toolkit.Result{}, err
	}

	cfg, err := toolkit.LoadConfigSource(toolkit.ConfigSource{
		Config:     input.Config,
		ConfigPath: input.ConfigPath,
		Format:     input.Format,
	})
	if err != nil {
		out := toolkit.RunOutput{
			Passed:       false,
			ExitCode:     2,
			ReportFormat: toolkit.NormalizeReportFormat(input.ReportType),
			ReportText:   err.Error(),
			Error:        err.Error(),
		}
		return toolkit.StructuredToolResult(out, out.ReportText), nil
	}

	if err := toolkit.GuardMCPConfig(cfg); err != nil {
		out := toolkit.RunOutput{
			Passed:       false,
			ExitCode:     2,
			ReportFormat: toolkit.NormalizeReportFormat(input.ReportType),
			ReportText:   err.Error(),
			Error:        err.Error(),
		}
		return toolkit.StructuredToolResult(out, out.ReportText), nil
	}

	out, err := toolkit.RunWithConfig(ctx, cfg, input.ReportType, input.TimeoutMS)
	if err != nil {
		return toolkit.Result{}, err
	}
	return toolkit.StructuredToolResult(out, out.ReportText), nil
}

func RenderReport(_ context.Context, args map[string]any) (toolkit.Result, error) {
	var input toolkit.RenderReportArgs
	if err := toolkit.DecodeArgs(args, &input); err != nil {
		return toolkit.Result{}, err
	}

	var report cleanr.Report
	if err := json.Unmarshal([]byte(input.ReportJSON), &report); err != nil {
		return toolkit.Result{}, err
	}

	rendered, err := toolkit.RenderReport(report, input.Format)
	if err != nil {
		return toolkit.Result{}, err
	}

	out := toolkit.RenderReportOutput{
		Format:   toolkit.NormalizeReportFormat(input.Format),
		Rendered: rendered,
	}
	return toolkit.StructuredToolResult(out, rendered), nil
}
