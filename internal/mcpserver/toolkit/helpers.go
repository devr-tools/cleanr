package toolkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cleanr/cleanr"
)

func DecodeArgs(args map[string]any, dest any) error {
	raw, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

func LoadConfigSource(src ConfigSource) (cleanr.Config, error) {
	if strings.TrimSpace(src.ConfigPath) != "" {
		return cleanr.LoadConfigFile(strings.TrimSpace(src.ConfigPath))
	}
	if strings.TrimSpace(src.Config) == "" {
		return cleanr.Config{}, fmt.Errorf("provide config or config_path")
	}
	return cleanr.LoadConfigData([]byte(src.Config), NormalizeConfigFormat(src.Format))
}

func StructuredToolResult(v any, text string) Result {
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

func NormalizeConfigFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "yaml", "yml":
		return "yaml"
	default:
		return "json"
	}
}

func NormalizeReportFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return "json"
	case "junit":
		return "junit"
	default:
		return "text"
	}
}

func RenderReport(report cleanr.Report, format string) (string, error) {
	var buf bytes.Buffer
	if err := cleanr.WriteReport(&buf, report, NormalizeReportFormat(format)); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

func RunWithConfig(ctx context.Context, cfg cleanr.Config, reportFormat string, timeoutMS int) (RunOutput, error) {
	runCtx := ctx
	if timeoutMS > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
		defer cancel()
	}

	report := cleanr.NewConfigRunner(cfg).Run(runCtx)
	reportText, err := RenderReport(report, reportFormat)
	if err != nil {
		return RunOutput{}, err
	}

	exitCode := 0
	if !report.Passed {
		exitCode = 1
	}

	return RunOutput{
		Passed:       report.Passed,
		ExitCode:     exitCode,
		TargetName:   report.Name,
		ReportFormat: NormalizeReportFormat(reportFormat),
		ReportText:   reportText,
		DurationMS:   report.Duration.Milliseconds(),
		Report:       report,
	}, nil
}

func ConfigSourceSchema() map[string]any {
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
