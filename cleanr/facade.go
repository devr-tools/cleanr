package cleanr

import (
	"context"
	"io"
	"net/http"

	adapterspkg "github.com/devr-tools/cleanr/cleanr/adapters"
	attestpkg "github.com/devr-tools/cleanr/cleanr/attest"
	configpkg "github.com/devr-tools/cleanr/cleanr/config"
	"github.com/devr-tools/cleanr/cleanr/core"
	generationpkg "github.com/devr-tools/cleanr/cleanr/generation"
	integrationspkg "github.com/devr-tools/cleanr/cleanr/integrations"
	reportpkg "github.com/devr-tools/cleanr/cleanr/report"
	snapshotspkg "github.com/devr-tools/cleanr/cleanr/snapshots"
	trendspkg "github.com/devr-tools/cleanr/cleanr/trends"
)

type Config = core.Config
type TargetConfig = core.TargetConfig
type OpenAIConfig = core.OpenAIConfig
type AnthropicConfig = core.AnthropicConfig
type ScenarioGenerationConfig = core.ScenarioGenerationConfig
type ScenarioGenerationSpec = core.ScenarioGenerationSpec
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
type GovernanceConfig = core.GovernanceConfig
type AttestationConfig = core.AttestationConfig
type IntegrationsConfig = core.IntegrationsConfig
type ResultSinkConfig = core.ResultSinkConfig
type TrendSourceConfig = core.TrendSourceConfig
type SummaryConfig = core.SummaryConfig
type PluginManifest = core.PluginManifest
type PluginSuite = core.PluginSuite
type PluginStateAdapter = core.PluginStateAdapter
type RunMetadata = core.RunMetadata
type ScenarioFingerprint = core.ScenarioFingerprint
type BuildDiff = core.BuildDiff
type ScenarioDiff = core.ScenarioDiff
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
type IntegrationReport = core.IntegrationReport
type ExternalTrendReport = core.ExternalTrendReport
type ResultSinkReport = core.ResultSinkReport
type SummaryArtifactReport = core.SummaryArtifactReport
type TrendReport = core.TrendReport
type TrendGateReport = core.TrendGateReport
type TrendSummary = core.TrendSummary
type SuiteTrend = core.SuiteTrend
type CaseTrend = core.CaseTrend
type FailureBucket = core.FailureBucket
type DriftTrend = core.DriftTrend
type ReplayArtifact = core.ReplayArtifact
type ReplayArtifactCase = core.ReplayArtifactCase
type ScenarioDataset = integrationspkg.ScenarioDataset
type ScenarioDatasetEntry = integrationspkg.ScenarioDatasetEntry
type DatasetScenarioOrigin = integrationspkg.DatasetScenarioOrigin
type ScenarioDatasetGenerator = integrationspkg.ScenarioDatasetGenerator
type ReviewedScenarioDataset = integrationspkg.ReviewedScenarioDataset
type ReviewedScenarioEntry = integrationspkg.ReviewedScenarioEntry
type DatasetReviewDiff = integrationspkg.DatasetReviewDiff
type DatasetReviewAnalysis = integrationspkg.DatasetReviewAnalysis
type DatasetReviewDecision = integrationspkg.DatasetReviewDecision
type DatasetReviewSummary = integrationspkg.DatasetReviewSummary
type DatasetReviewOptions = integrationspkg.DatasetReviewOptions
type DatasetReviewPolicy = integrationspkg.DatasetReviewPolicy
type DatasetReviewPolicyRule = integrationspkg.DatasetReviewPolicyRule
type BraintrustInsightDataset = integrationspkg.BraintrustInsightDataset
type BraintrustConfigPatchSet = integrationspkg.BraintrustConfigPatchSet
type BraintrustConfigPatchOperation = integrationspkg.BraintrustConfigPatchOperation
type ReleaseGateAttestation = core.ReleaseGateAttestation
type AttestationSubject = core.AttestationSubject
type AttestationPredicate = core.AttestationPredicate
type AttestationSignature = core.AttestationSignature
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

func LoadTrendHistoryData(data []byte, path string) (TrendHistoryFile, error) {
	return trendspkg.LoadData(data, path)
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

func BuildReplayArtifact(report Report) ReplayArtifact {
	return trendspkg.BuildReplayArtifact(report)
}

func LoadReplayArtifactFile(path string) (ReplayArtifact, error) {
	return trendspkg.LoadReplayArtifactFile(path)
}

func LoadReplayArtifactData(data []byte, path string) (ReplayArtifact, error) {
	return trendspkg.LoadReplayArtifactData(data, path)
}

func WriteReplayArtifactFile(path string, artifact ReplayArtifact) error {
	return trendspkg.WriteReplayArtifactFile(path, artifact)
}

func CompareTrendSources(ctx context.Context, cfg IntegrationsConfig, report Report, configPath string) []ExternalTrendReport {
	return integrationspkg.CompareTrendSources(ctx, cfg, report, configPath)
}

func PublishResultSinks(ctx context.Context, cfg IntegrationsConfig, report Report, replay *ReplayArtifact, attestation *ReleaseGateAttestation) []ResultSinkReport {
	return integrationspkg.PublishResultSinks(ctx, cfg, report, replay, attestation)
}

func EnsureIntegrationReport(report *Report) *IntegrationReport {
	return integrationspkg.EnsureReport(report)
}

func WriteSummaries(cfg IntegrationsConfig, report Report, configPath string) []SummaryArtifactReport {
	return integrationspkg.WriteSummaries(cfg, report, configPath)
}

func LoadScenarioDatasetFile(path string) (ScenarioDataset, error) {
	return integrationspkg.LoadScenarioDatasetFile(path)
}

func LoadScenarioDatasetData(data []byte, path string) (ScenarioDataset, error) {
	return integrationspkg.LoadScenarioDatasetData(data, path)
}

func WriteScenarioDatasetFile(path string, dataset ScenarioDataset) error {
	return integrationspkg.WriteScenarioDatasetFile(path, dataset)
}

func LoadReviewedScenarioDatasetFile(path string) (ReviewedScenarioDataset, error) {
	return integrationspkg.LoadReviewedScenarioDatasetFile(path)
}

func LoadReviewedScenarioDatasetData(data []byte, path string) (ReviewedScenarioDataset, error) {
	return integrationspkg.LoadReviewedScenarioDatasetData(data, path)
}

func WriteReviewedScenarioDatasetFile(path string, reviewed ReviewedScenarioDataset) error {
	return integrationspkg.WriteReviewedScenarioDatasetFile(path, reviewed)
}

func LoadDatasetReviewPolicyFile(path string) (DatasetReviewPolicy, error) {
	return integrationspkg.LoadDatasetReviewPolicyFile(path)
}

func LoadDatasetReviewPolicyData(data []byte, path string) (DatasetReviewPolicy, error) {
	return integrationspkg.LoadDatasetReviewPolicyData(data, path)
}

func WriteDatasetReviewPolicyFile(path string, policy DatasetReviewPolicy) error {
	return integrationspkg.WriteDatasetReviewPolicyFile(path, policy)
}

func ExportScenarioDataset(cfg Config, artifact ReplayArtifact, includeAll bool) ScenarioDataset {
	return integrationspkg.ExportScenarioDataset(cfg, artifact, includeAll)
}

func GenerateScenarioDataset(ctx context.Context, cfg Config, client *http.Client) (ScenarioDataset, error) {
	return generationpkg.GenerateDataset(ctx, cfg, client)
}

func ReviewDatasetAgainstConfig(base Config, dataset ScenarioDataset, opts DatasetReviewOptions) (ReviewedScenarioDataset, error) {
	return integrationspkg.ReviewDatasetAgainstConfig(base, dataset, opts)
}

func ApprovedDatasetFromReview(reviewed ReviewedScenarioDataset) ScenarioDataset {
	return integrationspkg.ApprovedDatasetFromReview(reviewed)
}

func MergeDatasetIntoConfig(base Config, dataset ScenarioDataset) Config {
	return integrationspkg.MergeDatasetIntoConfig(base, dataset)
}

func MergeReviewedDatasetIntoConfig(base Config, reviewed ReviewedScenarioDataset) Config {
	return integrationspkg.MergeReviewedDatasetIntoConfig(base, reviewed)
}

func LoadBraintrustInsightDatasetFile(path string) (BraintrustInsightDataset, error) {
	return integrationspkg.LoadBraintrustInsightDatasetFile(path)
}

func LoadBraintrustInsightDatasetData(data []byte, path string) (BraintrustInsightDataset, error) {
	return integrationspkg.LoadBraintrustInsightDatasetData(data, path)
}

func WriteBraintrustInsightDatasetFile(path string, dataset BraintrustInsightDataset) error {
	return integrationspkg.WriteBraintrustInsightDatasetFile(path, dataset)
}

func FetchBraintrustInsightDataset(ctx context.Context, source TrendSourceConfig, base Config) (BraintrustInsightDataset, error) {
	return integrationspkg.FetchBraintrustInsightDataset(ctx, source, base)
}

func ApplyBraintrustInsightDataset(base Config, dataset BraintrustInsightDataset, applyScenarios, applyPatches, approved bool) (Config, error) {
	return integrationspkg.ApplyBraintrustInsightDataset(base, dataset, applyScenarios, applyPatches, approved)
}

func BuildReleaseGateAttestation(report Report, artifact ReplayArtifact, rawKey string, keyID string) (ReleaseGateAttestation, error) {
	return attestpkg.BuildReleaseGateAttestation(report, artifact, rawKey, keyID)
}

func WriteReleaseGateAttestationFile(path string, attestation ReleaseGateAttestation) error {
	return attestpkg.WriteReleaseGateAttestationFile(path, attestation)
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
