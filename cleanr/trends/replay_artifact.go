package trends

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
	"github.com/devr-tools/cleanr/cleanr/fsatomic"
	"gopkg.in/yaml.v3"
)

var (
	sensitiveArtifactKeyPattern = regexp.MustCompile(`(?i)(api[-_]?key|auth(orization)?|token|secret|password|passwd|cookie|private[-_]?key|client[-_]?secret)`)
	bearerTokenPattern          = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{8,}`)
	openAIKeyPattern            = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{10,}\b`)
	awsAccessKeyPattern         = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)
	pemBlockPattern             = regexp.MustCompile(`-----BEGIN [A-Z ]+-----[\s\S]+?-----END [A-Z ]+-----`)
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
	return scrubReplayArtifact(artifact)
}

func LoadReplayArtifactFile(path string) (core.ReplayArtifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return core.ReplayArtifact{}, err
	}
	return LoadReplayArtifactData(data, path)
}

func LoadReplayArtifactData(data []byte, path string) (core.ReplayArtifact, error) {
	if isYAMLPath(path) {
		var generic any
		if err := yaml.Unmarshal(data, &generic); err != nil {
			return core.ReplayArtifact{}, fmt.Errorf("decode replay artifact: %w", err)
		}
		normalized := normalizeYAMLValue(generic)
		raw, err := json.Marshal(normalized)
		if err != nil {
			return core.ReplayArtifact{}, fmt.Errorf("decode replay artifact: %w", err)
		}
		var artifact core.ReplayArtifact
		if err := json.Unmarshal(raw, &artifact); err != nil {
			return core.ReplayArtifact{}, fmt.Errorf("decode replay artifact: %w", err)
		}
		return artifact, nil
	}
	var artifact core.ReplayArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return core.ReplayArtifact{}, fmt.Errorf("decode replay artifact: %w", err)
	}
	return artifact, nil
}

func WriteReplayArtifactFile(path string, artifact core.ReplayArtifact) error {
	data, err := encodeReplayArtifact(artifact, path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Atomic to match the trend-history writer: attestation digests and replay
	// tooling must never observe a truncated artifact.
	return fsatomic.WriteFile(path, append(data, '\n'), 0o644)
}

func encodeReplayArtifact(artifact core.ReplayArtifact, path string) ([]byte, error) {
	artifact = scrubReplayArtifact(artifact)
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
		out[key] = scrubArtifactValue(key, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func scrubReplayArtifact(artifact core.ReplayArtifact) core.ReplayArtifact {
	artifact.Target = scrubArtifactString("", artifact.Target)
	artifact.BuildID = scrubArtifactString("build_id", artifact.BuildID)
	if artifact.Metadata != nil {
		metadata := *artifact.Metadata
		metadata.BuildID = scrubArtifactString("build_id", metadata.BuildID)
		metadata.TargetType = scrubArtifactString("target_type", metadata.TargetType)
		metadata.ProviderModel = scrubArtifactString("provider_model", metadata.ProviderModel)
		metadata.ScenarioFingerprints = scrubScenarioFingerprints(metadata.ScenarioFingerprints)
		artifact.Metadata = &metadata
	}
	if artifact.BuildDiff != nil {
		buildDiff := *artifact.BuildDiff
		buildDiff.TargetTypeBefore = scrubArtifactString("target_type_before", buildDiff.TargetTypeBefore)
		buildDiff.TargetTypeAfter = scrubArtifactString("target_type_after", buildDiff.TargetTypeAfter)
		buildDiff.ModelBefore = scrubArtifactString("model_before", buildDiff.ModelBefore)
		buildDiff.ModelAfter = scrubArtifactString("model_after", buildDiff.ModelAfter)
		artifact.BuildDiff = &buildDiff
	}
	artifact.Failures = scrubReplayArtifactCases(artifact.Failures)
	return artifact
}

func scrubReplayArtifactCases(cases []core.ReplayArtifactCase) []core.ReplayArtifactCase {
	if len(cases) == 0 {
		return nil
	}
	out := make([]core.ReplayArtifactCase, 0, len(cases))
	for _, item := range cases {
		scrubbed := item
		scrubbed.Suite = scrubArtifactString("suite", scrubbed.Suite)
		scrubbed.Name = scrubArtifactString("name", scrubbed.Name)
		if scrubbed.Scenario != nil {
			scenario := *scrubbed.Scenario
			scenario.Name = scrubArtifactString("scenario_name", scenario.Name)
			scenario.Tags = scrubStringSlice("tags", scenario.Tags)
			scrubbed.Scenario = &scenario
		}
		scrubbed.Findings = scrubFindings(scrubbed.Findings)
		scrubbed.Evidence = scrubStringMapAny("", scrubbed.Evidence)
		out = append(out, scrubbed)
	}
	return out
}

func scrubScenarioFingerprints(items []core.ScenarioFingerprint) []core.ScenarioFingerprint {
	if len(items) == 0 {
		return nil
	}
	out := make([]core.ScenarioFingerprint, 0, len(items))
	for _, item := range items {
		scrubbed := item
		scrubbed.Name = scrubArtifactString("scenario_name", scrubbed.Name)
		scrubbed.Tags = scrubStringSlice("tags", scrubbed.Tags)
		out = append(out, scrubbed)
	}
	return out
}

func scrubFindings(findings []core.Finding) []core.Finding {
	if len(findings) == 0 {
		return nil
	}
	out := make([]core.Finding, 0, len(findings))
	for _, finding := range findings {
		scrubbed := finding
		scrubbed.Message = scrubArtifactString("message", scrubbed.Message)
		out = append(out, scrubbed)
	}
	return out
}

func scrubStringMapAny(key string, src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for childKey, value := range src {
		out[childKey] = scrubArtifactValue(childKey, value)
	}
	return out
}

func scrubStringSlice(key string, values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, scrubArtifactString(key, value))
	}
	return out
}

func scrubArtifactValue(key string, value any) any {
	switch typed := value.(type) {
	case string:
		return scrubArtifactString(key, typed)
	case []string:
		return scrubStringSlice(key, typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = scrubArtifactValue(key, item)
		}
		return out
	case map[string]any:
		return scrubStringMapAny(key, typed)
	default:
		return value
	}
}

func scrubArtifactString(key, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value
	}
	if sensitiveArtifactKeyPattern.MatchString(strings.TrimSpace(key)) {
		return "[REDACTED]"
	}
	scrubbed := bearerTokenPattern.ReplaceAllString(trimmed, "Bearer [REDACTED]")
	scrubbed = openAIKeyPattern.ReplaceAllString(scrubbed, "[REDACTED]")
	scrubbed = awsAccessKeyPattern.ReplaceAllString(scrubbed, "[REDACTED]")
	scrubbed = pemBlockPattern.ReplaceAllString(scrubbed, "[REDACTED PEM]")
	return scrubbed
}
