package integrations

import (
	"context"

	"github.com/devr-tools/cleanr/cleanr/core"
	runtimepkg "github.com/devr-tools/cleanr/cleanr/integrations/runtime"
)

func EnsureReport(report *core.Report) *core.IntegrationReport {
	return runtimepkg.EnsureReport(report)
}

func CompareTrendSources(ctx context.Context, cfg core.IntegrationsConfig, report core.Report, configPath string) []core.ExternalTrendReport {
	return runtimepkg.CompareTrendSources(ctx, cfg, report, configPath)
}

func PublishResultSinks(ctx context.Context, cfg core.IntegrationsConfig, report core.Report, replay *core.ReplayArtifact, attestation *core.ReleaseGateAttestation) []core.ResultSinkReport {
	return runtimepkg.PublishResultSinks(ctx, cfg, report, replay, attestation)
}

func WriteSummaries(cfg core.IntegrationsConfig, report core.Report, configPath string) []core.SummaryArtifactReport {
	return runtimepkg.WriteSummaries(cfg, report, configPath)
}
