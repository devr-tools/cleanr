package devtools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

type ReportOptions struct {
	Input  string
	Format string
	Preset string
}

func (r Runner) Report(opts ReportOptions) error {
	format := strings.TrimSpace(opts.Format)
	if format == "" {
		format = "text"
	}

	report, source, err := loadPreviewReport(r.WorkDir, opts)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(r.Stdout, "rendering %s report from %s\n\n", format, source); err != nil {
		return err
	}
	return cleanr.WriteReport(r.Stdout, report, format)
}

func loadPreviewReport(workDir string, opts ReportOptions) (cleanr.Report, string, error) {
	input := strings.TrimSpace(opts.Input)
	if input == "" {
		preset := strings.TrimSpace(opts.Preset)
		if preset == "" {
			preset = "fail"
		}
		report, err := demoReport(preset)
		if err != nil {
			return cleanr.Report{}, "", err
		}
		return report, "built-in " + preset + " preset", nil
	}

	path := resolvePath(workDir, input)
	data, err := os.ReadFile(path)
	if err != nil {
		return cleanr.Report{}, "", fmt.Errorf("read report input: %w", err)
	}

	var report cleanr.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return cleanr.Report{}, "", fmt.Errorf("decode report input: %w", err)
	}
	return report, path, nil
}

func demoReport(preset string) (cleanr.Report, error) {
	generated := time.Date(2026, time.May, 20, 14, 30, 0, 0, time.UTC)
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "pass":
		return cleanr.Report{
			Name:         "demo-assistant",
			Passed:       true,
			GeneratedAt:  generated,
			Duration:     1840 * time.Millisecond,
			TotalSuites:  2,
			FailedSuites: 0,
			TotalCases:   3,
			FailedCases:  0,
			Suites: []cleanr.SuiteResult{
				{
					Name:     "security",
					Passed:   true,
					Duration: 640 * time.Millisecond,
					Cases: []cleanr.CaseResult{
						{
							Name:     "pii-redaction",
							Passed:   true,
							Duration: 180 * time.Millisecond,
							Details: map[string]any{
								"provider":       "openai",
								"provider_model": "gpt-4o-mini",
							},
						},
						{
							Name:     "secret-blocking",
							Passed:   true,
							Duration: 210 * time.Millisecond,
							Details: map[string]any{
								"provider":       "openai",
								"provider_model": "gpt-4o-mini",
							},
						},
					},
				},
				{
					Name:     "drift",
					Passed:   true,
					Duration: 1200 * time.Millisecond,
					Cases: []cleanr.CaseResult{
						{
							Name:     "stable-support-answer",
							Passed:   true,
							Duration: 1200 * time.Millisecond,
							Score:    0.98,
							Details: map[string]any{
								"normalized_drift": 0.02,
								"samples":          3,
							},
						},
					},
				},
			},
		}, nil
	case "", "fail":
		return cleanr.Report{
			Name:            "demo-assistant",
			Passed:          false,
			GeneratedAt:     generated,
			Duration:        2620 * time.Millisecond,
			TotalSuites:     3,
			FailedSuites:    2,
			TotalCases:      4,
			FailedCases:     2,
			Recommendations: []string{"Tighten output length caps and redact sensitive data before returning model output."},
			Suites: []cleanr.SuiteResult{
				{
					Name:     "security",
					Passed:   false,
					Duration: 710 * time.Millisecond,
					Cases: []cleanr.CaseResult{
						{
							Name:     "secret-blocking",
							Passed:   false,
							Duration: 220 * time.Millisecond,
							Findings: []cleanr.Finding{
								{Severity: "critical", Message: "forbidden content detected: sk-live-123"},
							},
							Details: map[string]any{
								"provider":       "openai",
								"provider_model": "gpt-4o-mini",
								"finish_reason":  "stop",
							},
						},
						{
							Name:     "pii-redaction",
							Passed:   true,
							Duration: 190 * time.Millisecond,
							Details: map[string]any{
								"provider":       "openai",
								"provider_model": "gpt-4o-mini",
							},
						},
					},
				},
				{
					Name:     "token-optimization",
					Passed:   false,
					Duration: 880 * time.Millisecond,
					Cases: []cleanr.CaseResult{
						{
							Name:     "verbose-refund-answer",
							Passed:   false,
							Duration: 300 * time.Millisecond,
							Findings: []cleanr.Finding{
								{Severity: "high", Message: "estimated output tokens 482 exceeded threshold 180"},
								{Severity: "medium", Message: "response duplication ratio 0.41 exceeded threshold 0.12"},
							},
							Details: map[string]any{
								"input_tokens":               91,
								"output_tokens":              482,
								"total_tokens":               573,
								"output_input_ratio":         5.29,
								"response_duplication_ratio": 0.41,
								"estimated_savings_tokens":   214,
							},
						},
					},
					Meta: map[string]any{
						"total_input_tokens":  91,
						"total_output_tokens": 482,
						"total_tokens":        573,
						"estimated_savings":   214,
					},
				},
				{
					Name:     "drift",
					Passed:   true,
					Duration: 1030 * time.Millisecond,
					Cases: []cleanr.CaseResult{
						{
							Name:     "stable-support-answer",
							Passed:   true,
							Duration: 1030 * time.Millisecond,
							Score:    0.94,
							Details: map[string]any{
								"normalized_drift": 0.06,
								"samples":          3,
							},
						},
					},
				},
			},
		}, nil
	default:
		return cleanr.Report{}, fmt.Errorf("unsupported report preset: %s", preset)
	}
}
