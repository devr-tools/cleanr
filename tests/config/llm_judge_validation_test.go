package tests

import (
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/cleanr/core"
)

func judgeBaseConfig(judge core.LLMJudgeConfig, scenarios ...core.Scenario) cleanr.Config {
	cfg := cleanr.Config{
		Target: core.TargetConfig{
			Type: "openai", OpenAI: core.OpenAIConfig{Model: "gpt-4.1-mini", APIMode: "responses"},
		},
		Scenarios: scenarios,
	}
	cfg.Suites.LLMJudge = judge
	return cfg
}

func judgeBaseConfigWithDefaults(t *testing.T, judge core.LLMJudgeConfig, scenarios ...core.Scenario) cleanr.Config {
	t.Helper()

	cfg := judgeBaseConfig(judge, scenarios...)
	data, err := cleanr.MarshalConfig(cfg, "yaml")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	loaded, err := cleanr.LoadConfigData(data, "yaml")
	if err != nil {
		t.Fatalf("load config with defaults: %v", err)
	}
	return loaded
}

func validationMessage(t *testing.T, err error, field string) string {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error for %s, got nil", field)
	}
	return err.Error()
}

func TestValidateLLMJudgeRequiresProvider(t *testing.T) {
	cfg := judgeBaseConfig(core.LLMJudgeConfig{Enabled: true},
		core.Scenario{Name: "s", Input: "hi"})
	msg := validationMessage(t, cleanr.ValidateConfig(cfg), "provider")
	if !strings.Contains(msg, "suites.llm_judge.provider.type") {
		t.Fatalf("expected provider.type error, got: %s", msg)
	}
}

func TestValidateLLMJudgeRejectsOutOfRangeMinScore(t *testing.T) {
	cfg := judgeBaseConfig(core.LLMJudgeConfig{
		Enabled:  true,
		Provider: core.TargetConfig{Type: "openai", OpenAI: core.OpenAIConfig{Model: "gpt-4.1-mini", APIMode: "responses"}},
		MinScore: 1.5,
	}, core.Scenario{Name: "s", Input: "hi"})
	msg := validationMessage(t, cleanr.ValidateConfig(cfg), "min_score")
	if !strings.Contains(msg, "suites.llm_judge.min_score") {
		t.Fatalf("expected min_score error, got: %s", msg)
	}
}

func TestValidateLLMJudgeRequireReferenceScopedByTags(t *testing.T) {
	judge := core.LLMJudgeConfig{
		Enabled:          true,
		Provider:         core.TargetConfig{Type: "openai", OpenAI: core.OpenAIConfig{Model: "gpt-4.1-mini", APIMode: "responses"}},
		RequireReference: true,
		StableTags:       []string{"graded"},
	}
	cfg := judgeBaseConfig(judge,
		core.Scenario{Name: "graded-missing", Input: "a", Tags: []string{"graded"}},
		core.Scenario{Name: "untagged-ok", Input: "b", Tags: []string{"other"}},
	)
	msg := validationMessage(t, cleanr.ValidateConfig(cfg), "reference_answer")
	if !strings.Contains(msg, "scenarios[0].reference_answer") {
		t.Fatalf("expected reference_answer error on tagged scenario, got: %s", msg)
	}
	if strings.Contains(msg, "scenarios[1].reference_answer") {
		t.Fatalf("untagged scenario should not require a reference: %s", msg)
	}
}

func TestValidateLLMJudgeValidConfigPasses(t *testing.T) {
	judge := core.LLMJudgeConfig{
		Enabled:          true,
		Provider:         core.TargetConfig{Type: "anthropic", Anthropic: core.AnthropicConfig{Model: "claude-sonnet-4-20250514"}},
		MinScore:         0.7,
		Samples:          3,
		MaxDisagreement:  0.4,
		RequireReference: true,
	}
	cfg := judgeBaseConfig(judge,
		core.Scenario{Name: "s", Input: "hi", ReferenceAnswer: "hello"},
	)
	if err := cleanr.ValidateConfig(cfg); err != nil {
		t.Fatalf("expected valid judge config, got: %v", err)
	}
}

func TestValidateLLMJudgePairwiseRequiresBaseline(t *testing.T) {
	judge := core.LLMJudgeConfig{
		Enabled:  true,
		Mode:     "pairwise",
		Provider: core.TargetConfig{Type: "openai", OpenAI: core.OpenAIConfig{Model: "gpt-4.1-mini", APIMode: "responses"}},
	}
	cfg := judgeBaseConfig(judge, core.Scenario{Name: "s", Input: "hi"})
	msg := validationMessage(t, cleanr.ValidateConfig(cfg), "baseline")
	if !strings.Contains(msg, "suites.llm_judge.baseline.type") {
		t.Fatalf("expected baseline.type error, got: %s", msg)
	}
}

func TestValidateLLMJudgeRejectsUnknownMode(t *testing.T) {
	judge := core.LLMJudgeConfig{
		Enabled:  true,
		Mode:     "tournament",
		Provider: core.TargetConfig{Type: "openai", OpenAI: core.OpenAIConfig{Model: "gpt-4.1-mini", APIMode: "responses"}},
	}
	cfg := judgeBaseConfig(judge, core.Scenario{Name: "s", Input: "hi"})
	msg := validationMessage(t, cleanr.ValidateConfig(cfg), "mode")
	if !strings.Contains(msg, "suites.llm_judge.mode") {
		t.Fatalf("expected mode error, got: %s", msg)
	}
}

func TestValidateLLMJudgePairwiseValidConfigPasses(t *testing.T) {
	judge := core.LLMJudgeConfig{
		Enabled:    true,
		Mode:       "pairwise",
		Provider:   core.TargetConfig{Type: "openai", OpenAI: core.OpenAIConfig{Model: "gpt-4.1-mini", APIMode: "responses"}},
		Baseline:   core.TargetConfig{Type: "anthropic", Anthropic: core.AnthropicConfig{Model: "claude-sonnet-4-20250514"}},
		MinWinRate: 0.6,
		Samples:    3,
	}
	cfg := judgeBaseConfig(judge, core.Scenario{Name: "s", Input: "hi"})
	if err := cleanr.ValidateConfig(cfg); err != nil {
		t.Fatalf("expected valid pairwise config, got: %v", err)
	}
}

func TestValidateLLMJudgeCalibrationRequiresFile(t *testing.T) {
	judge := core.LLMJudgeConfig{
		Enabled:                true,
		Provider:               core.TargetConfig{Type: "openai", OpenAI: core.OpenAIConfig{Model: "gpt-4.1-mini", APIMode: "responses"}},
		MinCalibrationAccuracy: 0.9,
	}
	cfg := judgeBaseConfig(judge, core.Scenario{Name: "s", Input: "hi"})
	msg := validationMessage(t, cleanr.ValidateConfig(cfg), "calibration_file")
	if !strings.Contains(msg, "suites.llm_judge.calibration_file") {
		t.Fatalf("expected calibration_file error, got: %s", msg)
	}
}

func TestApplyLLMJudgePairwiseDefaults(t *testing.T) {
	cfg := judgeBaseConfigWithDefaults(t, core.LLMJudgeConfig{
		Enabled:  true,
		Mode:     "pairwise",
		Provider: core.TargetConfig{Type: "openai", OpenAI: core.OpenAIConfig{Model: "gpt-4.1-mini"}},
		Baseline: core.TargetConfig{Type: "anthropic", Anthropic: core.AnthropicConfig{Model: "claude-sonnet-4-20250514"}},
	}, core.Scenario{Name: "s", Input: "hi"})

	j := cfg.Suites.LLMJudge
	if j.MinWinRate != 0.5 {
		t.Fatalf("expected default min_win_rate 0.5, got %v", j.MinWinRate)
	}
	if j.Baseline.Anthropic.APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Fatalf("expected baseline defaults applied, got %q", j.Baseline.Anthropic.APIKeyEnv)
	}
}

func TestApplyLLMJudgeDefaults(t *testing.T) {
	cfg := judgeBaseConfigWithDefaults(t, core.LLMJudgeConfig{
		Enabled:  true,
		Provider: core.TargetConfig{Type: "openai", OpenAI: core.OpenAIConfig{Model: "gpt-4.1-mini"}},
		Samples:  2,
	}, core.Scenario{Name: "s", Input: "hi"})

	j := cfg.Suites.LLMJudge
	if j.Scale != 5 {
		t.Fatalf("expected default scale 5, got %d", j.Scale)
	}
	if j.MinScore != 0.6 {
		t.Fatalf("expected default min_score 0.6, got %v", j.MinScore)
	}
	if j.MaxDisagreement != 0.4 {
		t.Fatalf("expected default max_disagreement 0.4 when sampling, got %v", j.MaxDisagreement)
	}
	if j.ConfidenceLevel != 0.95 {
		t.Fatalf("expected default confidence_level 0.95, got %v", j.ConfidenceLevel)
	}
	if j.Provider.OpenAI.APIKeyEnv != "OPENAI_API_KEY" {
		t.Fatalf("expected provider defaults applied, got %q", j.Provider.OpenAI.APIKeyEnv)
	}
}
