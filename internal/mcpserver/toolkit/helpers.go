package toolkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	trendspkg "github.com/devr-tools/cleanr/cleanr/trends"
	"gopkg.in/yaml.v3"
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
	case "sarif":
		return "sarif"
	case "agent":
		return "agent"
	case "html":
		return "html"
	default:
		return "text"
	}
}

func NormalizeDataFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "yaml", "yml":
		return "yaml"
	default:
		return "json"
	}
}

func RenderReport(report cleanr.Report, format string) (string, error) {
	var buf bytes.Buffer
	if err := cleanr.WriteReport(&buf, report, NormalizeReportFormat(format)); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

func RenderTrendAnalysis(analysis cleanr.TrendAnalysis, format string) (string, error) {
	var buf bytes.Buffer
	if err := cleanr.WriteTrendAnalysis(&buf, analysis, NormalizeTrendFormat(format)); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

func NormalizeTrendFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return "json"
	case "html":
		return "html"
	default:
		return "text"
	}
}

func EncodeData(v any, format string) (string, error) {
	switch NormalizeDataFormat(format) {
	case "yaml":
		raw, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			return "", err
		}
		data, err := yaml.Marshal(generic)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(data), "\n"), nil
	default:
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
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

func DatasetSourceSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"dataset": map[string]any{
				"type":        "string",
				"description": "Raw cleanr scenario dataset content.",
			},
			"dataset_path": map[string]any{
				"type":        "string",
				"description": "Local path to a scenario dataset file.",
			},
			"dataset_format": map[string]any{
				"type":        "string",
				"description": "Dataset format when dataset is provided inline.",
				"enum":        []string{"json", "yaml"},
			},
		},
	}
}

func LoadScenarioDatasetSource(path, data, format string) (cleanr.ScenarioDataset, error) {
	return loadSource(path, data, format, "dataset", "dataset_path",
		cleanr.LoadScenarioDatasetFile,
		cleanr.LoadScenarioDatasetData,
	)
}

func LoadDatasetReviewPolicySource(path, data, format string) (*cleanr.DatasetReviewPolicy, error) {
	if strings.TrimSpace(path) == "" && strings.TrimSpace(data) == "" {
		return nil, nil
	}
	policy, err := loadSource(path, data, format, "policy", "policy_path",
		cleanr.LoadDatasetReviewPolicyFile,
		cleanr.LoadDatasetReviewPolicyData,
	)
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func LoadTrendHistorySource(path, data, format string) (cleanr.TrendHistoryFile, error) {
	return loadSource(path, data, format, "history", "history_path",
		cleanr.LoadTrendHistoryFile,
		cleanr.LoadTrendHistoryData,
	)
}

func AnalyzeTrendHistorySource(path, data, format string, window int) (cleanr.TrendAnalysis, error) {
	history, err := LoadTrendHistorySource(path, data, format)
	if err != nil {
		return cleanr.TrendAnalysis{}, err
	}
	return trendspkg.Analyze(history, window), nil
}

func LoadReplayArtifactSource(path, data, format string) (cleanr.ReplayArtifact, error) {
	return loadSource(path, data, format, "replay", "replay_path",
		cleanr.LoadReplayArtifactFile,
		cleanr.LoadReplayArtifactData,
	)
}

func loadSource[T any](path, data, format, label, pathLabel string, loadFile func(string) (T, error), loadData func([]byte, string) (T, error)) (T, error) {
	if strings.TrimSpace(path) != "" {
		return loadFile(strings.TrimSpace(path))
	}
	if strings.TrimSpace(data) == "" {
		var zero T
		return zero, fmt.Errorf("provide %s or %s", label, pathLabel)
	}
	inlinePath := "inline." + NormalizeDataFormat(format)
	return loadData([]byte(data), inlinePath)
}
