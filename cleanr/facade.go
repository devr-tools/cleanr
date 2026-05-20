package cleanr

import (
	"context"
	"io"
	"net/http"

	adapterspkg "cleanr/cleanr/adapters"
	configpkg "cleanr/cleanr/config"
	"cleanr/cleanr/core"
	reportpkg "cleanr/cleanr/report"
	snapshotspkg "cleanr/cleanr/snapshots"
	trendspkg "cleanr/cleanr/trends"
)

type Config = core.Config
type TargetConfig = core.TargetConfig
type OpenAIConfig = core.OpenAIConfig
type AnthropicConfig = core.AnthropicConfig
type Scenario = core.Scenario
type ContextSource = core.ContextSource
type MemoryReplaySession = core.MemoryReplaySession
type ExpectedMutation = core.ExpectedMutation
type ExpectedStateChange = core.ExpectedStateChange
type Assertion = core.Assertion
type SuitesConfig = core.SuitesConfig
type PromptInjectionConfig = core.PromptInjectionConfig
type SecurityConfig = core.SecurityConfig
type LoadConfig = core.LoadConfig
type ChaosConfig = core.ChaosConfig
type DriftConfig = core.DriftConfig
type ShadowStateConfig = core.ShadowStateConfig
type ProvenanceConfig = core.ProvenanceConfig
type ClaimTraceConfig = core.ClaimTraceConfig
type ReleasePolicyConfig = core.ReleasePolicyConfig
type PolicyRule = core.PolicyRule
type MemorySafetyConfig = core.MemorySafetyConfig
type TokenOptimizationConfig = core.TokenOptimizationConfig
type ReportingConfig = core.ReportingConfig
type TrendGateConfig = core.TrendGateConfig
type Request = core.Request
type Response = core.Response
type TokenUsage = core.TokenUsage
type ProviderResponse = core.ProviderResponse
type ToolCall = core.ToolCall
type SourceUse = core.SourceUse
type ApprovalArtifact = core.ApprovalArtifact
type StateChange = core.StateChange
type MemoryOperation = core.MemoryOperation
type SnapshotFile = snapshotspkg.File
type ScenarioSnapshot = snapshotspkg.ScenarioSnapshot
type TrendHistoryFile = trendspkg.HistoryFile
type TrendHistoryRun = trendspkg.HistoryRun
type HistorySuite = trendspkg.HistorySuite
type HistoryCase = trendspkg.HistoryCase
type HistoryDriftMetrics = trendspkg.HistoryDriftMetrics
type TrendAnalysis = trendspkg.Analysis
type TrendRunSnapshot = trendspkg.RunSnapshot
type TrendAnalysisDelta = trendspkg.AnalysisDelta
type TrendDriftWindow = trendspkg.DriftWindow
type Finding = core.Finding
type CaseResult = core.CaseResult
type SuiteResult = core.SuiteResult
type Report = core.Report
type TrendReport = core.TrendReport
type TrendGateReport = core.TrendGateReport
type TrendSummary = core.TrendSummary
type SuiteTrend = core.SuiteTrend
type CaseTrend = core.CaseTrend
type FailureBucket = core.FailureBucket
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

func EvaluateTrendGates(report *Report, cfg TrendGateConfig) {
	trendspkg.EvaluateGates(report, cfg)
}

func WriteReport(w io.Writer, report Report, format string) error {
	return reportpkg.Write(w, report, format)
}

func TextReport(report Report) string {
	return reportpkg.Text(report)
}

func AnalyzeTrendHistoryFile(path string, window int) (TrendAnalysis, error) {
	return trendspkg.AnalyzeFile(path, window)
}

func WriteTrendAnalysis(w io.Writer, analysis TrendAnalysis, format string) error {
	return trendspkg.WriteAnalysis(w, analysis, format)
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
