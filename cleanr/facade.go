package cleanr

import (
	"context"
	"net/http"

	adapterspkg "cleanr/cleanr/adapters"
	configpkg "cleanr/cleanr/config"
	"cleanr/cleanr/core"
	snapshotspkg "cleanr/cleanr/snapshots"
	trendspkg "cleanr/cleanr/trends"
)

type Config = core.Config
type TargetConfig = core.TargetConfig
type OpenAIConfig = core.OpenAIConfig
type AnthropicConfig = core.AnthropicConfig
type Scenario = core.Scenario
type Assertion = core.Assertion
type SuitesConfig = core.SuitesConfig
type PromptInjectionConfig = core.PromptInjectionConfig
type SecurityConfig = core.SecurityConfig
type LoadConfig = core.LoadConfig
type ChaosConfig = core.ChaosConfig
type DriftConfig = core.DriftConfig
type TokenOptimizationConfig = core.TokenOptimizationConfig
type ReportingConfig = core.ReportingConfig
type Request = core.Request
type Response = core.Response
type TokenUsage = core.TokenUsage
type ProviderResponse = core.ProviderResponse
type ToolCall = core.ToolCall
type SnapshotFile = snapshotspkg.File
type ScenarioSnapshot = snapshotspkg.ScenarioSnapshot
type TrendHistoryFile = trendspkg.HistoryFile
type TrendHistoryRun = trendspkg.HistoryRun
type Finding = core.Finding
type CaseResult = core.CaseResult
type SuiteResult = core.SuiteResult
type Report = core.Report
type TrendReport = core.TrendReport
type TrendSummary = core.TrendSummary
type SuiteTrend = core.SuiteTrend
type DriftTrend = core.DriftTrend
type Target = core.Target
type Engine = core.Engine
type RunContext = core.RunContext
type FieldError = configpkg.FieldError
type ValidationErrors = configpkg.ValidationErrors

func LoadConfigFile(path string) (Config, error) {
	return configpkg.LoadConfigFile(path)
}

func WriteConfigFile(path string, cfg Config) error {
	return configpkg.WriteConfigFile(path, cfg)
}

func LoadConfigData(data []byte, format string) (Config, error) {
	return configpkg.LoadConfigData(data, format)
}

func MarshalConfig(cfg Config, format string) ([]byte, error) {
	return configpkg.MarshalConfig(cfg, format)
}

func ValidateConfig(cfg Config) error {
	return configpkg.ValidateConfig(cfg)
}

func ExampleConfig() Config {
	return configpkg.ExampleConfig()
}

func LoadSnapshotFile(path string) (SnapshotFile, error) {
	return snapshotspkg.LoadFile(path)
}

func WriteSnapshotFile(path string, snapshot SnapshotFile) error {
	return snapshotspkg.WriteFile(path, snapshot)
}

func CaptureSnapshots(ctx context.Context, cfg Config, target Target) (SnapshotFile, error) {
	return snapshotspkg.Capture(ctx, cfg, target)
}

func LoadTrendHistoryFile(path string) (TrendHistoryFile, error) {
	return trendspkg.LoadFile(path)
}

func WriteTrendHistoryFile(path string, history TrendHistoryFile) error {
	return trendspkg.WriteFile(path, history)
}

func AttachTrendHistory(report *Report, path, buildID string, limit int) error {
	return trendspkg.AttachAndPersist(report, path, buildID, limit)
}

func NewTarget(cfg TargetConfig, client *http.Client) Target {
	return adapterspkg.NewTargetFromConfig(cfg, client)
}

func NewHTTPTarget(cfg TargetConfig, client *http.Client) Target {
	return adapterspkg.NewHTTP(cfg, client)
}

func NewOpenAITarget(cfg TargetConfig, client *http.Client) Target {
	return adapterspkg.NewOpenAI(cfg, client)
}

func NewAnthropicTarget(cfg TargetConfig, client *http.Client) Target {
	return adapterspkg.NewAnthropic(cfg, client)
}
