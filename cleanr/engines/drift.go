package engines

import (
	"context"
	"fmt"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
	snapshotspkg "github.com/devr-tools/cleanr/cleanr/snapshots"
)

type DriftEngine struct{}

func (DriftEngine) Name() string { return "drift" }

func (DriftEngine) Run(ctx context.Context, runCtx *core.RunContext) core.SuiteResult {
	cfg := runCtx.Config.Suites.Drift
	stable := filterStableScenarios(runCtx.Config.Scenarios, cfg.StableTags)
	if len(stable) == 0 {
		stable = runCtx.Config.Scenarios
	}
	baseline, failedResult := loadDriftBaseline(cfg.BaselineFile)
	if failedResult != nil {
		return *failedResult
	}
	cases := make([]core.CaseResult, 0, len(stable))
	for _, scenario := range stable {
		cases = append(cases, evaluateDriftScenario(ctx, runCtx, cfg, baseline, scenario))
	}
	return core.SuiteResult{Name: "drift", Passed: allPassed(cases), Cases: cases, Meta: driftMeta(cfg)}
}

func loadDriftBaseline(path string) (snapshotspkg.File, *core.SuiteResult) {
	if path == "" {
		return snapshotspkg.File{}, nil
	}
	baseline, err := snapshotspkg.LoadFile(path)
	if err == nil {
		return baseline, nil
	}
	result := core.SuiteResult{
		Name:   "drift",
		Passed: false,
		Cases: []core.CaseResult{{
			Name:     "baseline-load",
			Passed:   false,
			Duration: 0,
			Findings: []core.Finding{{
				Severity: "critical",
				Message:  fmt.Sprintf("failed to load baseline file %s: %v", path, err),
			}},
		}},
	}
	return snapshotspkg.File{}, &result
}

func evaluateDriftScenario(ctx context.Context, runCtx *core.RunContext, cfg core.DriftConfig, baseline snapshotspkg.File, scenario core.Scenario) core.CaseResult {
	start := time.Now()
	responses, representative, representativeSet, findings := collectDriftResponses(ctx, runCtx, cfg, scenario)
	drift, consistency := measureDrift(responses)
	semanticDrift, semanticConsistency := measureSemanticDrift(responses)
	findings = append(findings, evaluateDriftThresholdFindings(cfg, semanticDrift, semanticConsistency)...)
	details := buildDriftDetails(cfg, drift, semanticDrift, consistency, semanticConsistency, len(responses))
	if representativeSet {
		details = responseDetails(representative, details)
	}
	findings, details = applyBaselineDriftComparison(findings, details, cfg, baseline, scenario.Name, representative, representativeSet)
	return core.CaseResult{
		Name:     scenario.Name,
		Passed:   len(findings) == 0,
		Duration: time.Since(start),
		Score:    semanticConsistency,
		Findings: findings,
		Details:  details,
	}
}

func collectDriftResponses(ctx context.Context, runCtx *core.RunContext, cfg core.DriftConfig, scenario core.Scenario) ([]string, core.Response, bool, []core.Finding) {
	responses := make([]string, 0, cfg.Iterations)
	findings := make([]core.Finding, 0)
	var representative core.Response
	var representativeSet bool
	for i := 0; i < cfg.Iterations; i++ {
		resp := runCtx.Target.Invoke(ctx, core.Request{
			Scenario: scenario,
			System:   scenario.System,
			Prompt:   scenario.Input,
			Timeout:  runCtx.Config.Target.Timeout(),
		})
		if resp.Err != nil {
			findings = append(findings, core.Finding{Severity: "high", Message: resp.Err.Error()})
			continue
		}
		responses = append(responses, resp.Text)
		if !representativeSet {
			representative = resp
			representativeSet = true
		}
	}
	return responses, representative, representativeSet, findings
}

func evaluateDriftThresholdFindings(cfg core.DriftConfig, semanticDrift, semanticConsistency float64) []core.Finding {
	findings := make([]core.Finding, 0)
	if semanticDrift > cfg.MaxSemanticDrift {
		findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("semantic drift %.3f exceeded threshold %.3f", semanticDrift, cfg.MaxSemanticDrift)})
	}
	if semanticConsistency < cfg.MinSemanticConsistencyScore {
		findings = append(findings, core.Finding{Severity: "medium", Message: fmt.Sprintf("semantic consistency score %.3f fell below threshold %.3f", semanticConsistency, cfg.MinSemanticConsistencyScore)})
	}
	return findings
}

func buildDriftDetails(cfg core.DriftConfig, drift, semanticDrift, consistency, semanticConsistency float64, sampleCount int) map[string]any {
	details := map[string]any{
		"normalized_drift":            round3(drift),
		"semantic_drift":              round3(semanticDrift),
		"consistency_score":           round3(consistency),
		"semantic_consistency_score":  round3(semanticConsistency),
		"samples":                     sampleCount,
		"semantic_similarity_profile": "local_similarity_v1",
	}
	if drift > cfg.MaxNormalizedDrift && semanticDrift <= cfg.MaxSemanticDrift {
		details["lexical_drift_note"] = fmt.Sprintf("normalized drift %.3f exceeded threshold %.3f, but semantic drift remained within threshold", round3(drift), cfg.MaxNormalizedDrift)
	}
	if consistency < cfg.MinConsistencyScore && semanticConsistency >= cfg.MinSemanticConsistencyScore {
		details["lexical_consistency_note"] = fmt.Sprintf("consistency score %.3f fell below threshold %.3f, but semantic consistency remained within threshold", round3(consistency), cfg.MinConsistencyScore)
	}
	return details
}

func applyBaselineDriftComparison(findings []core.Finding, details map[string]any, cfg core.DriftConfig, baseline snapshotspkg.File, scenarioName string, representative core.Response, representativeSet bool) ([]core.Finding, map[string]any) {
	if cfg.BaselineFile == "" {
		return findings, details
	}
	if !representativeSet {
		findings = append(findings, core.Finding{Severity: "high", Message: "no successful response available for baseline comparison"})
		return findings, details
	}
	snapshot, ok := baseline.FindScenario(scenarioName)
	if !ok {
		findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("missing baseline snapshot for scenario %s", scenarioName)})
		return findings, details
	}
	snapshotDrift := normalizedDistance(snapshot.Text, representative.Text)
	snapshotSemanticDrift := semanticDistance(snapshot.Text, representative.Text)
	details["baseline_text"] = trimForReport(snapshot.Text)
	details["current_text"] = trimForReport(representative.Text)
	details["baseline_drift"] = round3(snapshotDrift)
	details["baseline_semantic_drift"] = round3(snapshotSemanticDrift)
	details["baseline_status_code"] = snapshot.StatusCode
	findings = append(findings, baselineDriftFindings(cfg, snapshot, representative, snapshotSemanticDrift)...)
	if snapshotDrift > cfg.MaxSnapshotDrift && snapshotSemanticDrift <= cfg.MaxSemanticSnapshotDrift {
		details["baseline_lexical_note"] = fmt.Sprintf("baseline drift %.3f exceeded threshold %.3f, but semantic baseline drift remained within threshold", round3(snapshotDrift), cfg.MaxSnapshotDrift)
	}
	return findings, details
}

func baselineDriftFindings(cfg core.DriftConfig, snapshot snapshotspkg.ScenarioSnapshot, representative core.Response, snapshotSemanticDrift float64) []core.Finding {
	findings := make([]core.Finding, 0)
	if snapshot.StatusCode != representative.StatusCode {
		findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("baseline status code %d changed to %d", snapshot.StatusCode, representative.StatusCode)})
	}
	if snapshot.Normalized.FinishReason != "" && snapshot.Normalized.FinishReason != representative.Normalized.FinishReason {
		findings = append(findings, core.Finding{Severity: "medium", Message: fmt.Sprintf("baseline finish reason %q changed to %q", snapshot.Normalized.FinishReason, representative.Normalized.FinishReason)})
	}
	if len(snapshot.Normalized.ToolCalls) != len(representative.Normalized.ToolCalls) {
		findings = append(findings, core.Finding{Severity: "medium", Message: fmt.Sprintf("baseline tool call count %d changed to %d", len(snapshot.Normalized.ToolCalls), len(representative.Normalized.ToolCalls))})
	}
	if snapshotSemanticDrift > cfg.MaxSemanticSnapshotDrift {
		findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("semantic baseline drift %.3f exceeded threshold %.3f", snapshotSemanticDrift, cfg.MaxSemanticSnapshotDrift)})
	}
	return findings
}

func driftMeta(cfg core.DriftConfig) map[string]any {
	if cfg.BaselineFile == "" {
		return map[string]any{}
	}
	return map[string]any{"baseline_file": cfg.BaselineFile}
}
