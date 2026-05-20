package engines

import (
	"context"
	"fmt"
	"time"

	"cleanr/cleanr/core"
	snapshotspkg "cleanr/cleanr/snapshots"
)

type DriftEngine struct{}

func (DriftEngine) Name() string { return "drift" }

func (DriftEngine) Run(ctx context.Context, runCtx *core.RunContext) core.SuiteResult {
	cfg := runCtx.Config.Suites.Drift
	stable := filterStableScenarios(runCtx.Config.Scenarios, cfg.StableTags)
	if len(stable) == 0 {
		stable = runCtx.Config.Scenarios
	}
	var baseline snapshotspkg.File
	var baselineErr error
	if cfg.BaselineFile != "" {
		baseline, baselineErr = snapshotspkg.LoadFile(cfg.BaselineFile)
		if baselineErr != nil {
			return core.SuiteResult{
				Name:   "drift",
				Passed: false,
				Cases: []core.CaseResult{{
					Name:     "baseline-load",
					Passed:   false,
					Duration: 0,
					Findings: []core.Finding{{
						Severity: "critical",
						Message:  fmt.Sprintf("failed to load baseline file %s: %v", cfg.BaselineFile, baselineErr),
					}},
				}},
			}
		}
	}
	cases := make([]core.CaseResult, 0, len(stable))
	for _, scenario := range stable {
		start := time.Now()
		responses := make([]string, 0, cfg.Iterations)
		var representative core.Response
		var representativeSet bool
		findings := make([]core.Finding, 0)
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
		drift, consistency := measureDrift(responses)
		if drift > cfg.MaxNormalizedDrift {
			findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("normalized drift %.3f exceeded threshold %.3f", drift, cfg.MaxNormalizedDrift)})
		}
		if consistency < cfg.MinConsistencyScore {
			findings = append(findings, core.Finding{Severity: "medium", Message: fmt.Sprintf("consistency score %.3f fell below threshold %.3f", consistency, cfg.MinConsistencyScore)})
		}
		details := map[string]any{
			"normalized_drift": drift,
			"samples":          len(responses),
		}
		if representativeSet {
			details = responseDetails(representative, details)
		}
		if cfg.BaselineFile != "" && representativeSet {
			if snapshot, ok := baseline.FindScenario(scenario.Name); ok {
				snapshotDrift := normalizedDistance(snapshot.Text, representative.Text)
				details["baseline_text"] = trimForReport(snapshot.Text)
				details["current_text"] = trimForReport(representative.Text)
				details["baseline_drift"] = round3(snapshotDrift)
				details["baseline_status_code"] = snapshot.StatusCode
				if snapshot.StatusCode != representative.StatusCode {
					findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("baseline status code %d changed to %d", snapshot.StatusCode, representative.StatusCode)})
				}
				if snapshot.Normalized.FinishReason != "" && snapshot.Normalized.FinishReason != representative.Normalized.FinishReason {
					findings = append(findings, core.Finding{Severity: "medium", Message: fmt.Sprintf("baseline finish reason %q changed to %q", snapshot.Normalized.FinishReason, representative.Normalized.FinishReason)})
				}
				if len(snapshot.Normalized.ToolCalls) != len(representative.Normalized.ToolCalls) {
					findings = append(findings, core.Finding{Severity: "medium", Message: fmt.Sprintf("baseline tool call count %d changed to %d", len(snapshot.Normalized.ToolCalls), len(representative.Normalized.ToolCalls))})
				}
				if snapshotDrift > cfg.MaxSnapshotDrift {
					findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("baseline drift %.3f exceeded threshold %.3f", snapshotDrift, cfg.MaxSnapshotDrift)})
				}
			} else {
				findings = append(findings, core.Finding{Severity: "high", Message: fmt.Sprintf("missing baseline snapshot for scenario %s", scenario.Name)})
			}
		} else if cfg.BaselineFile != "" {
			findings = append(findings, core.Finding{Severity: "high", Message: "no successful response available for baseline comparison"})
		}
		cases = append(cases, core.CaseResult{
			Name:     scenario.Name,
			Passed:   len(findings) == 0,
			Duration: time.Since(start),
			Score:    consistency,
			Findings: findings,
			Details:  details,
		})
	}
	meta := map[string]any{}
	if cfg.BaselineFile != "" {
		meta["baseline_file"] = cfg.BaselineFile
	}
	return core.SuiteResult{Name: "drift", Passed: allPassed(cases), Cases: cases, Meta: meta}
}
