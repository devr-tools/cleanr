package trends

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"cleanr/cleanr/core"
	"gopkg.in/yaml.v3"
)

func BuildReplayArtifact(report core.Report) core.ReplayArtifact {
	artifact := core.ReplayArtifact{
		Version:      "v1alpha1",
		Target:       report.Name,
		GeneratedAt:  report.GeneratedAt,
		Passed:       report.Passed,
		FailedSuites: report.FailedSuites,
		FailedCases:  report.FailedCases,
		Metadata:     report.Metadata,
	}
	if report.Metadata != nil {
		artifact.BuildID = report.Metadata.BuildID
	}
	if report.Trend != nil {
		artifact.BuildDiff = report.Trend.BuildDiff
		if !report.Trend.Baseline {
			summary := report.Trend.Summary
			artifact.TrendSummary = &summary
		}
	}

	scenarioByName := scenarioFingerprintMap(report.Metadata)
	for _, suite := range report.Suites {
		if len(suite.Findings) > 0 {
			artifact.Failures = append(artifact.Failures, core.ReplayArtifactCase{
				Suite:    suite.Name,
				Name:     "_suite",
				Findings: append([]core.Finding(nil), suite.Findings...),
				Evidence: filteredEvidence(suite.Meta),
				Failed:   !suite.Passed,
			})
		}
		for _, c := range suite.Cases {
			if c.Passed && len(c.Findings) == 0 {
				continue
			}
			caseArtifact := core.ReplayArtifactCase{
				Suite:    suite.Name,
				Name:     c.Name,
				Findings: append([]core.Finding(nil), c.Findings...),
				Evidence: filteredEvidence(c.Details),
				Failed:   !c.Passed,
			}
			if scenario, ok := scenarioByName[c.Name]; ok {
				copyScenario := scenario
				caseArtifact.Scenario = &copyScenario
			}
			artifact.Failures = append(artifact.Failures, caseArtifact)
		}
	}

	sort.Slice(artifact.Failures, func(i, j int) bool {
		if artifact.Failures[i].Suite == artifact.Failures[j].Suite {
			return artifact.Failures[i].Name < artifact.Failures[j].Name
		}
		return artifact.Failures[i].Suite < artifact.Failures[j].Suite
	})
	return artifact
}

func WriteReplayArtifactFile(path string, artifact core.ReplayArtifact) error {
	data, err := encodeReplayArtifact(artifact, path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func encodeReplayArtifact(artifact core.ReplayArtifact, path string) ([]byte, error) {
	if isYAMLPath(path) {
		raw, err := json.Marshal(artifact)
		if err != nil {
			return nil, fmt.Errorf("encode replay artifact: %w", err)
		}
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			return nil, fmt.Errorf("encode replay artifact: %w", err)
		}
		data, err := yaml.Marshal(generic)
		if err != nil {
			return nil, fmt.Errorf("encode replay artifact: %w", err)
		}
		return data, nil
	}
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode replay artifact: %w", err)
	}
	return data, nil
}

func scenarioFingerprintMap(metadata *core.RunMetadata) map[string]core.ScenarioFingerprint {
	if metadata == nil || len(metadata.ScenarioFingerprints) == 0 {
		return nil
	}
	out := make(map[string]core.ScenarioFingerprint, len(metadata.ScenarioFingerprints))
	for _, scenario := range metadata.ScenarioFingerprints {
		out[scenario.Name] = scenario
	}
	return out
}

func filteredEvidence(details map[string]any) map[string]any {
	if len(details) == 0 {
		return nil
	}
	out := make(map[string]any)
	for key, value := range details {
		if key == "provider_raw" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
