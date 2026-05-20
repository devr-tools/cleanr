package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cleanr/cleanr"
)

func decodeArgs(args map[string]any, dest any) error {
	raw, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
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

func structuredToolResult(v any, text string) Result {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		trimmed = "{}"
	}
	return Result{
		Content: []Content{{
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

func renderReport(report cleanr.Report, format string) (string, error) {
	var buf bytes.Buffer
	if err := cleanr.WriteReport(&buf, report, normalizeReportFormat(format)); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

func runWithConfig(ctx context.Context, cfg cleanr.Config, reportFormat string, timeoutMS int) (runOutput, error) {
	runCtx := ctx
	if timeoutMS > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
		defer cancel()
	}

	report := cleanr.NewConfigRunner(cfg).Run(runCtx)
	reportText, err := renderReport(report, reportFormat)
	if err != nil {
		return runOutput{}, err
	}

	exitCode := 0
	if !report.Passed {
		exitCode = 1
	}

	return runOutput{
		Passed:       report.Passed,
		ExitCode:     exitCode,
		TargetName:   report.Name,
		ReportFormat: normalizeReportFormat(reportFormat),
		ReportText:   reportText,
		DurationMS:   report.Duration.Milliseconds(),
		Report:       report,
	}, nil
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
