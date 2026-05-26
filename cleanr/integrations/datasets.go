package integrations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
	"gopkg.in/yaml.v3"
)

type ScenarioDataset struct {
	Version        string                    `json:"version"`
	Source         string                    `json:"source,omitempty"`
	Target         string                    `json:"target,omitempty"`
	BuildID        string                    `json:"build_id,omitempty"`
	GeneratedAt    time.Time                 `json:"generated_at"`
	ReviewRequired bool                      `json:"review_required,omitempty"`
	Warnings       []string                  `json:"warnings,omitempty"`
	Generator      *ScenarioDatasetGenerator `json:"generator,omitempty"`
	Scenarios      []ScenarioDatasetEntry    `json:"scenarios"`
}

type ScenarioDatasetEntry struct {
	Scenario core.Scenario         `json:"scenario"`
	Origin   DatasetScenarioOrigin `json:"origin,omitempty"`
}

type DatasetScenarioOrigin struct {
	Suite    string         `json:"suite,omitempty"`
	Case     string         `json:"case,omitempty"`
	BuildID  string         `json:"build_id,omitempty"`
	Findings []core.Finding `json:"findings,omitempty"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

type ScenarioDatasetGenerator struct {
	Provider       string    `json:"provider,omitempty"`
	ProviderID     string    `json:"provider_id,omitempty"`
	Model          string    `json:"model,omitempty"`
	TargetType     string    `json:"target_type,omitempty"`
	TargetName     string    `json:"target_name,omitempty"`
	RequestedCount int       `json:"requested_count,omitempty"`
	ReturnedCount  int       `json:"returned_count,omitempty"`
	AppKind        string    `json:"app_kind,omitempty"`
	Goals          []string  `json:"goals,omitempty"`
	RiskAreas      []string  `json:"risk_areas,omitempty"`
	PromptHash     string    `json:"prompt_hash,omitempty"`
	GeneratedAt    time.Time `json:"generated_at,omitempty"`
}

func LoadScenarioDatasetFile(path string) (ScenarioDataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ScenarioDataset{}, err
	}
	return LoadScenarioDatasetData(data, path)
}

func LoadScenarioDatasetData(data []byte, path string) (ScenarioDataset, error) {
	if isYAMLPath(path) {
		var generic any
		if err := yaml.Unmarshal(data, &generic); err != nil {
			return ScenarioDataset{}, fmt.Errorf("decode scenario dataset: %w", err)
		}
		normalized := normalizeYAMLValue(generic)
		raw, err := json.Marshal(normalized)
		if err != nil {
			return ScenarioDataset{}, fmt.Errorf("decode scenario dataset: %w", err)
		}
		var dataset ScenarioDataset
		if err := json.Unmarshal(raw, &dataset); err != nil {
			return ScenarioDataset{}, fmt.Errorf("decode scenario dataset: %w", err)
		}
		return dataset, nil
	}
	var dataset ScenarioDataset
	if err := json.Unmarshal(data, &dataset); err != nil {
		return ScenarioDataset{}, fmt.Errorf("decode scenario dataset: %w", err)
	}
	return dataset, nil
}

func WriteScenarioDatasetFile(path string, dataset ScenarioDataset) error {
	data, err := encodeScenarioDataset(dataset, path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func ExportScenarioDataset(cfg core.Config, artifact core.ReplayArtifact, includeAll bool) ScenarioDataset {
	scenariosByName := make(map[string]core.Scenario, len(cfg.Scenarios))
	for _, scenario := range cfg.Scenarios {
		scenariosByName[scenario.Name] = scenario
	}

	entriesByName := map[string]ScenarioDatasetEntry{}
	if includeAll {
		for _, scenario := range cfg.Scenarios {
			entriesByName[scenario.Name] = ScenarioDatasetEntry{Scenario: scenario}
		}
	}

	for _, failure := range artifact.Failures {
		scenarioName := failure.Name
		if failure.Scenario != nil && strings.TrimSpace(failure.Scenario.Name) != "" {
			scenarioName = failure.Scenario.Name
		}
		scenario, ok := scenariosByName[scenarioName]
		if !ok {
			continue
		}
		scenario.Tags = ensureTag(scenario.Tags, "regression")
		entriesByName[scenario.Name] = ScenarioDatasetEntry{
			Scenario: scenario,
			Origin: DatasetScenarioOrigin{
				Suite:    failure.Suite,
				Case:     failure.Name,
				BuildID:  artifact.BuildID,
				Findings: append([]core.Finding(nil), failure.Findings...),
				Evidence: cloneEvidence(failure.Evidence),
			},
		}
	}

	names := make([]string, 0, len(entriesByName))
	for name := range entriesByName {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]ScenarioDatasetEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, entriesByName[name])
	}

	return ScenarioDataset{
		Version:     "v1alpha1",
		Source:      "cleanr-replay",
		Target:      cfg.Target.Name,
		BuildID:     artifact.BuildID,
		GeneratedAt: time.Now().UTC(),
		Scenarios:   entries,
	}
}

func MergeDatasetIntoConfig(base core.Config, dataset ScenarioDataset) core.Config {
	if base.Version == "" {
		base.Version = "v1alpha1"
	}
	if len(dataset.Scenarios) == 0 {
		base.Scenarios = nil
		return base
	}
	merged := make([]core.Scenario, 0, len(base.Scenarios)+len(dataset.Scenarios))
	seen := make(map[string]int, len(base.Scenarios)+len(dataset.Scenarios))
	for _, scenario := range base.Scenarios {
		seen[scenario.Name] = len(merged)
		merged = append(merged, scenario)
	}
	for _, item := range dataset.Scenarios {
		name := item.Scenario.Name
		if idx, ok := seen[name]; ok {
			merged[idx] = item.Scenario
			continue
		}
		seen[name] = len(merged)
		merged = append(merged, item.Scenario)
	}
	base.Scenarios = merged
	return base
}

func encodeScenarioDataset(dataset ScenarioDataset, path string) ([]byte, error) {
	if isYAMLPath(path) {
		raw, err := json.Marshal(dataset)
		if err != nil {
			return nil, fmt.Errorf("encode scenario dataset: %w", err)
		}
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			return nil, fmt.Errorf("encode scenario dataset: %w", err)
		}
		data, err := yaml.Marshal(generic)
		if err != nil {
			return nil, fmt.Errorf("encode scenario dataset: %w", err)
		}
		return data, nil
	}
	data, err := json.MarshalIndent(dataset, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode scenario dataset: %w", err)
	}
	return data, nil
}

func ensureTag(tags []string, tag string) []string {
	for _, existing := range tags {
		if existing == tag {
			return append([]string(nil), tags...)
		}
	}
	out := append([]string(nil), tags...)
	return append(out, tag)
}

func cloneEvidence(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func isYAMLPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func normalizeYAMLValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = normalizeYAMLValue(value)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[fmt.Sprint(key)] = normalizeYAMLValue(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = normalizeYAMLValue(value)
		}
		return out
	default:
		return v
	}
}
