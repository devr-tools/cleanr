// Package cleanr (root) is a thin, legacy forwarding facade over the canonical
// public API in the cleanr/ subpackage (github.com/devr-tools/cleanr/cleanr,
// defined in cleanr/facade.go). Every symbol here is an alias or a one-line
// forward to that canonical package, which is where all new public API should
// be added.
//
// This root package exists only for backwards compatibility with earlier
// import paths and intentionally re-exports a drifted subset of the canonical
// facade. Do NOT add new API surface here; add it to cleanr/facade.go and, only
// if back-compat requires it, forward the new symbol from this file.
//
// TODO(pre-v1): reconcile this root facade with cleanr/facade.go — either
// regenerate it as a complete, mechanical mirror or remove it in favor of the
// canonical import path once external consumers have migrated.
package cleanr

import (
	"context"
	"io"
	"net/http"

	corepkg "github.com/devr-tools/cleanr/cleanr"
)

type Config = corepkg.Config
type TargetConfig = corepkg.TargetConfig
type ScenarioGenerationConfig = corepkg.ScenarioGenerationConfig
type ScenarioGenerationSpec = corepkg.ScenarioGenerationSpec
type Report = corepkg.Report
type ReplayArtifact = corepkg.ReplayArtifact
type ReleaseGateAttestation = corepkg.ReleaseGateAttestation
type TrendAnalysis = corepkg.TrendAnalysis
type TrendGateConfig = corepkg.TrendGateConfig
type IntegrationsConfig = corepkg.IntegrationsConfig
type ScenarioDataset = corepkg.ScenarioDataset
type ScenarioDatasetGenerator = corepkg.ScenarioDatasetGenerator
type Target = corepkg.Target
type Runner = corepkg.Runner

func LoadConfigFile(path string) (Config, error) {
	return corepkg.LoadConfigFile(path)
}

func WriteConfigFile(path string, cfg Config) error {
	return corepkg.WriteConfigFile(path, cfg)
}

func ValidateConfig(cfg Config) error {
	return corepkg.ValidateConfig(cfg)
}

func ExampleConfig() Config {
	return corepkg.ExampleConfig()
}

func NewRunner(cfg Config, target Target) *Runner {
	return corepkg.NewRunner(cfg, target)
}

func NewConfigRunner(cfg Config) *Runner {
	return corepkg.NewConfigRunner(cfg)
}

func NewHTTPRunner(cfg Config) *Runner {
	return corepkg.NewHTTPRunner(cfg)
}

func NewTarget(cfg TargetConfig, client *http.Client) Target {
	return corepkg.NewTarget(cfg, client)
}

func NewHTTPTarget(cfg TargetConfig, client *http.Client) Target {
	return corepkg.NewHTTPTarget(cfg, client)
}

func NewOpenAITarget(cfg TargetConfig, client *http.Client) Target {
	return corepkg.NewOpenAITarget(cfg, client)
}

func NewAnthropicTarget(cfg TargetConfig, client *http.Client) Target {
	return corepkg.NewAnthropicTarget(cfg, client)
}

func LoadTrendHistoryFile(path string) (corepkg.TrendHistoryFile, error) {
	return corepkg.LoadTrendHistoryFile(path)
}

func AnalyzeTrendHistoryFile(path string, window int) (TrendAnalysis, error) {
	return corepkg.AnalyzeTrendHistoryFile(path, window)
}

func WriteTrendAnalysis(w io.Writer, analysis TrendAnalysis, format string) error {
	return corepkg.WriteTrendAnalysis(w, analysis, format)
}

func BuildReplayArtifact(report Report) ReplayArtifact {
	return corepkg.BuildReplayArtifact(report)
}

func BuildReleaseGateAttestation(report Report, artifact ReplayArtifact, rawKey string, keyID string) (ReleaseGateAttestation, error) {
	return corepkg.BuildReleaseGateAttestation(report, artifact, rawKey, keyID)
}

func WriteReleaseGateAttestationFile(path string, attestation ReleaseGateAttestation) error {
	return corepkg.WriteReleaseGateAttestationFile(path, attestation)
}

func EvaluateTrendGates(report *Report, cfg TrendGateConfig) {
	corepkg.EvaluateTrendGates(report, cfg)
}

func CompareTrendSources(ctx context.Context, cfg IntegrationsConfig, report Report, configPath string) []corepkg.ExternalTrendReport {
	return corepkg.CompareTrendSources(ctx, cfg, report, configPath)
}

func PublishResultSinks(ctx context.Context, cfg IntegrationsConfig, report Report, replay *ReplayArtifact, attestation *ReleaseGateAttestation) []corepkg.ResultSinkReport {
	return corepkg.PublishResultSinks(ctx, cfg, report, replay, attestation)
}

func ExportScenarioDataset(cfg Config, artifact ReplayArtifact, includeAll bool) ScenarioDataset {
	return corepkg.ExportScenarioDataset(cfg, artifact, includeAll)
}

func GenerateScenarioDataset(ctx context.Context, cfg Config, client *http.Client) (ScenarioDataset, error) {
	return corepkg.GenerateScenarioDataset(ctx, cfg, client)
}

func MergeDatasetIntoConfig(base Config, dataset ScenarioDataset) Config {
	return corepkg.MergeDatasetIntoConfig(base, dataset)
}

func WriteReport(w io.Writer, report Report, format string) error {
	return corepkg.WriteReport(w, report, format)
}

func TextReport(report Report) string {
	return corepkg.TextReport(report)
}
