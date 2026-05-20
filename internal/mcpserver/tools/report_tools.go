package tools

import (
	"context"
	"encoding/json"

	"cleanr/cleanr"
)

func callRun(ctx context.Context, args map[string]any) (Result, error) {
	var input runArgs
	if err := decodeArgs(args, &input); err != nil {
		return Result{}, err
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

	out, err := runWithConfig(ctx, cfg, input.ReportType, input.TimeoutMS)
	if err != nil {
		return Result{}, err
	}
	return structuredToolResult(out, out.ReportText), nil
}

func callRenderReport(_ context.Context, args map[string]any) (Result, error) {
	var input renderReportArgs
	if err := decodeArgs(args, &input); err != nil {
		return Result{}, err
	}

	var report cleanr.Report
	if err := json.Unmarshal([]byte(input.ReportJSON), &report); err != nil {
		return Result{}, err
	}

	rendered, err := renderReport(report, input.Format)
	if err != nil {
		return Result{}, err
	}

	out := renderReportOutput{
		Format:   normalizeReportFormat(input.Format),
		Rendered: rendered,
	}
	return structuredToolResult(out, rendered), nil
}
