package engines

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
	"gopkg.in/yaml.v3"
)

type judgeCalibrationFile struct {
	Labels []judgeCalibrationLabel `json:"labels" yaml:"labels"`
}

type judgeCalibrationLabel struct {
	Scenario string   `json:"scenario" yaml:"scenario"`
	Score    *float64 `json:"score,omitempty" yaml:"score,omitempty"`
	Pass     *bool    `json:"pass,omitempty" yaml:"pass,omitempty"`
}

func buildJudgePool(cfg core.LLMJudgeConfig, primary core.Target) []namedTarget {
	out := []namedTarget{{
		label:  judgeTargetDisplayLabel(cfg.Provider, "judge"),
		cfg:    cfg.Provider,
		target: primary,
	}}
	for _, provider := range cfg.Ensemble {
		out = append(out, namedTarget{
			label:  judgeTargetDisplayLabel(provider, "ensemble"),
			cfg:    provider,
			target: judgeTargetFactory(provider),
		})
	}
	return out
}

func judgeTargetDisplayLabel(cfg core.TargetConfig, fallback string) string {
	if name := strings.TrimSpace(cfg.Name); name != "" {
		return name
	}
	if model := strings.TrimSpace(cfg.OpenAI.Model); model != "" {
		return model
	}
	if model := strings.TrimSpace(cfg.Anthropic.Model); model != "" {
		return model
	}
	if command := strings.TrimSpace(cfg.CLI.Command); command != "" {
		return fallback + ":" + command
	}
	if targetType := strings.TrimSpace(cfg.Type); targetType != "" {
		return fallback + ":" + targetType
	}
	return fallback
}

func judgeTargetComparisonLabel(cfg core.TargetConfig, idx int) string {
	label := judgeTargetDisplayLabel(cfg, fmt.Sprintf("target-%d", idx+1))
	return strings.TrimSpace(label)
}

func applyJudgePostAnalysis(ctx context.Context, runCtx *core.RunContext, cfg core.LLMJudgeConfig, judge core.Target, scenarios []core.Scenario, suite core.SuiteResult) core.SuiteResult {
	if suite.Meta == nil {
		suite.Meta = map[string]any{}
	}
	if flaky := collectFlakyCases(suite.Cases); len(flaky) > 0 {
		suite.Meta["flaky_cases"] = flaky
	}
	if strings.TrimSpace(cfg.CalibrationFile) != "" {
		report, err := calibrateJudgeCases(cfg, suite.Cases)
		if err != nil {
			suite.Passed = false
			suite.Findings = append(suite.Findings, core.Finding{Severity: "high", Message: fmt.Sprintf("judge calibration: %v", err)})
		} else {
			suite.Meta["calibration"] = report
			if !report["passed"].(bool) {
				suite.Passed = false
				for _, finding := range calibrationFindings(report) {
					suite.Findings = append(suite.Findings, finding)
				}
			}
		}
	}
	if len(cfg.ComparisonTargets) > 0 {
		matrix, err := buildComparisonMatrix(ctx, runCtx, cfg, judge, scenarios)
		if err != nil {
			suite.Findings = append(suite.Findings, core.Finding{Severity: "medium", Message: fmt.Sprintf("comparison matrix: %v", err)})
		} else {
			suite.Meta["comparison_matrix"] = matrix
		}
	}
	if len(suite.Findings) > 0 && suite.Passed {
		suite.Passed = false
	}
	return suite
}

func collectFlakyCases(cases []core.CaseResult) []string {
	out := make([]string, 0)
	for _, c := range cases {
		if detailsFlag(c.Details, "flake_detected") {
			out = append(out, c.Name)
		}
	}
	return out
}

func detailsFlag(details map[string]any, key string) bool {
	if details == nil {
		return false
	}
	flag, _ := details[key].(bool)
	return flag
}

func calibrateJudgeCases(cfg core.LLMJudgeConfig, cases []core.CaseResult) (map[string]any, error) {
	labels, err := loadCalibrationLabels(cfg.CalibrationFile)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]core.CaseResult, len(cases))
	for _, c := range cases {
		byName[c.Name] = c
	}
	matched := 0
	passMatches := 0
	scoreComparisons := 0
	totalAbsErr := 0.0
	for _, label := range labels {
		c, ok := byName[label.Scenario]
		if !ok {
			continue
		}
		matched++
		expectedPass := false
		if label.Pass != nil {
			expectedPass = *label.Pass
		} else if label.Score != nil {
			threshold := cfg.MinScore
			if cfg.ModeValue() == "pairwise" {
				threshold = cfg.MinWinRate
			}
			expectedPass = *label.Score >= threshold
		}
		if c.Passed == expectedPass {
			passMatches++
		}
		if label.Score != nil {
			if score, ok := caseReliabilityScore(c); ok {
				totalAbsErr += abs(score - *label.Score)
				scoreComparisons++
			}
		}
	}
	if matched == 0 {
		return nil, fmt.Errorf("no calibration labels matched executed scenarios")
	}
	accuracy := round3(float64(passMatches) / float64(matched))
	mae := 0.0
	if scoreComparisons > 0 {
		mae = round3(totalAbsErr / float64(scoreComparisons))
	}
	passed := true
	if cfg.MinCalibrationAccuracy > 0 && accuracy < cfg.MinCalibrationAccuracy {
		passed = false
	}
	if cfg.MaxCalibrationMAE > 0 && scoreComparisons > 0 && mae > cfg.MaxCalibrationMAE {
		passed = false
	}
	return map[string]any{
		"matched_labels":  matched,
		"accuracy":        accuracy,
		"mae":             mae,
		"score_labels":    scoreComparisons,
		"expected_passes": passMatches,
		"passed":          passed,
	}, nil
}

func calibrationFindings(report map[string]any) []core.Finding {
	findings := make([]core.Finding, 0, 2)
	if accuracy, ok := report["accuracy"].(float64); ok {
		findings = append(findings, core.Finding{Severity: "medium", Message: fmt.Sprintf("judge calibration accuracy %.2f did not meet the configured threshold", accuracy)})
	}
	if mae, ok := report["mae"].(float64); ok && mae > 0 {
		findings = append(findings, core.Finding{Severity: "medium", Message: fmt.Sprintf("judge calibration MAE %.2f exceeded the configured threshold", mae)})
	}
	return findings
}

func loadCalibrationLabels(path string) ([]judgeCalibrationLabel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var labels []judgeCalibrationLabel
	if err := json.Unmarshal(data, &labels); err == nil && len(labels) > 0 {
		return labels, nil
	}
	var wrapped judgeCalibrationFile
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Labels) > 0 {
		return wrapped.Labels, nil
	}
	if err := yaml.Unmarshal(data, &labels); err == nil && len(labels) > 0 {
		return labels, nil
	}
	if err := yaml.Unmarshal(data, &wrapped); err == nil && len(wrapped.Labels) > 0 {
		return wrapped.Labels, nil
	}
	return nil, fmt.Errorf("expected a JSON or YAML array of calibration labels")
}

func buildComparisonMatrix(ctx context.Context, runCtx *core.RunContext, cfg core.LLMJudgeConfig, judge core.Target, scenarios []core.Scenario) (map[string]any, error) {
	if cfg.ModeValue() != "score" {
		return map[string]any{"skipped": "comparison matrix currently supports score mode only"}, nil
	}
	run := scoreRun{
		scale:           cfg.ScaleValue(),
		samples:         cfg.SamplesValue(),
		minScore:        cfg.MinScore,
		maxDisagreement: cfg.MaxDisagreement,
		confidenceLevel: cfg.ConfidenceLevelValue(),
		minPassRate:     cfg.MinPassRate,
		maxFlakeRate:    cfg.MaxFlakeRate,
		cascadeMargin:   cfg.CascadeMargin,
	}
	targets := []namedTarget{{
		label:  judgeTargetComparisonLabel(runCtx.Config.Target, 0),
		cfg:    runCtx.Config.Target,
		target: runCtx.Target,
	}}
	for i, targetCfg := range cfg.ComparisonTargets {
		targets = append(targets, namedTarget{
			label:  judgeTargetComparisonLabel(targetCfg, i+1),
			cfg:    targetCfg,
			target: comparisonTargetFactory(targetCfg),
		})
	}

	labels := make([]string, 0, len(targets))
	averages := map[string]float64{}
	counts := map[string]int{}
	rows := make([]map[string]any, 0, len(scenarios))
	for _, target := range targets {
		labels = append(labels, target.label)
	}
	judges := buildJudgePool(cfg, judge)
	for _, scenario := range scenarios {
		scores := map[string]float64{}
		bestLabel := ""
		bestScore := -1.0
		for _, target := range targets {
			caseResult := (LLMJudgeEngine{}).runScoreCase(ctx, scoreCaseInput{
				runCtx:   &core.RunContext{Config: runCtx.Config, Target: target.target},
				cfg:      cfg,
				judge:    judge,
				judges:   judges,
				scenario: scenario,
				run:      run,
			})
			score, ok := caseReliabilityScore(caseResult)
			if !ok {
				score = 0
			}
			scores[target.label] = round3(score)
			averages[target.label] += score
			counts[target.label]++
			if score > bestScore {
				bestScore = score
				bestLabel = target.label
			}
		}
		rows = append(rows, map[string]any{
			"scenario": scenario.Name,
			"scores":   scores,
			"winner":   bestLabel,
		})
	}
	ranking := append([]string(nil), labels...)
	for label, total := range averages {
		if counts[label] > 0 {
			averages[label] = round3(total / float64(counts[label]))
		}
	}
	sort.Slice(ranking, func(i, j int) bool {
		if averages[ranking[i]] == averages[ranking[j]] {
			return ranking[i] < ranking[j]
		}
		return averages[ranking[i]] > averages[ranking[j]]
	})
	return map[string]any{
		"targets":   labels,
		"averages":  averages,
		"ranking":   ranking,
		"scenarios": rows,
	}, nil
}

func caseReliabilityScore(result core.CaseResult) (float64, bool) {
	if result.Details == nil {
		return 0, false
	}
	for _, key := range []string{"normalized_score", "win_rate"} {
		if value, ok := result.Details[key].(float64); ok {
			return value, true
		}
	}
	return 0, false
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
