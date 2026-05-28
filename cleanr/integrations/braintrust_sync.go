package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
	runtimepkg "github.com/devr-tools/cleanr/cleanr/integrations/runtime"
	"gopkg.in/yaml.v3"
)

const braintrustInsightDatasetVersion = "v1alpha1"

type BraintrustInsightDataset struct {
	Version         string                    `json:"version"`
	Source          string                    `json:"source,omitempty"`
	Project         string                    `json:"project,omitempty"`
	Experiment      string                    `json:"experiment,omitempty"`
	ExperimentID    string                    `json:"experiment_id,omitempty"`
	ExperimentURL   string                    `json:"experiment_url,omitempty"`
	BuildID         string                    `json:"build_id,omitempty"`
	GeneratedAt     time.Time                 `json:"generated_at"`
	ReviewRequired  bool                      `json:"review_required,omitempty"`
	Warnings        []string                  `json:"warnings,omitempty"`
	ScenarioDataset *ScenarioDataset          `json:"scenario_dataset,omitempty"`
	ConfigPatch     *BraintrustConfigPatchSet `json:"config_patch,omitempty"`
}

type BraintrustConfigPatchSet struct {
	ReviewRequired bool                             `json:"review_required,omitempty"`
	Operations     []BraintrustConfigPatchOperation `json:"operations,omitempty"`
}

type BraintrustConfigPatchOperation struct {
	Op     string `json:"op"`
	Path   string `json:"path"`
	Reason string `json:"reason,omitempty"`
	Source string `json:"source,omitempty"`
	Value  any    `json:"value,omitempty"`
}

func LoadBraintrustInsightDatasetFile(path string) (BraintrustInsightDataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BraintrustInsightDataset{}, err
	}
	return LoadBraintrustInsightDatasetData(data, path)
}

func LoadBraintrustInsightDatasetData(data []byte, path string) (BraintrustInsightDataset, error) {
	if isYAMLPath(path) {
		var generic any
		if err := yaml.Unmarshal(data, &generic); err != nil {
			return BraintrustInsightDataset{}, fmt.Errorf("decode braintrust insight dataset: %w", err)
		}
		normalized := normalizeYAMLValue(generic)
		raw, err := json.Marshal(normalized)
		if err != nil {
			return BraintrustInsightDataset{}, fmt.Errorf("decode braintrust insight dataset: %w", err)
		}
		var dataset BraintrustInsightDataset
		if err := json.Unmarshal(raw, &dataset); err != nil {
			return BraintrustInsightDataset{}, fmt.Errorf("decode braintrust insight dataset: %w", err)
		}
		return dataset, nil
	}
	var dataset BraintrustInsightDataset
	if err := json.Unmarshal(data, &dataset); err != nil {
		return BraintrustInsightDataset{}, fmt.Errorf("decode braintrust insight dataset: %w", err)
	}
	return dataset, nil
}

func WriteBraintrustInsightDatasetFile(path string, dataset BraintrustInsightDataset) error {
	data, err := encodeBraintrustInsightDataset(dataset, path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func FetchBraintrustInsightDataset(ctx context.Context, source core.TrendSourceConfig, base core.Config) (BraintrustInsightDataset, error) {
	artifacts, err := runtimepkg.LoadBraintrustSyncArtifacts(ctx, source)
	if err != nil {
		return BraintrustInsightDataset{}, err
	}

	out := BraintrustInsightDataset{
		Version:       braintrustInsightDatasetVersion,
		Source:        "braintrust",
		Project:       strings.TrimSpace(source.Project),
		Experiment:    strings.TrimSpace(source.Experiment),
		ExperimentID:  artifacts.ExperimentID,
		ExperimentURL: artifacts.ExperimentURL,
		GeneratedAt:   artifacts.ExperimentCreated.UTC(),
	}

	if artifacts.ReplayArtifact != nil {
		exported := ExportScenarioDataset(base, *artifacts.ReplayArtifact, false)
		if strings.TrimSpace(exported.BuildID) != "" {
			out.BuildID = exported.BuildID
		}
		out.ScenarioDataset = &exported
	}

	if len(artifacts.InsightPayload) > 0 {
		raw, err := json.Marshal(artifacts.InsightPayload)
		if err != nil {
			return BraintrustInsightDataset{}, fmt.Errorf("decode braintrust sync insight: %w", err)
		}
		var remote BraintrustInsightDataset
		if err := json.Unmarshal(raw, &remote); err != nil {
			return BraintrustInsightDataset{}, fmt.Errorf("decode braintrust sync insight: %w", err)
		}
		out = mergeBraintrustInsights(out, remote)
	}

	if out.Version == "" {
		out.Version = braintrustInsightDatasetVersion
	}
	if out.GeneratedAt.IsZero() {
		out.GeneratedAt = time.Now().UTC()
	}
	return out, nil
}

func ApplyBraintrustInsightDataset(base core.Config, dataset BraintrustInsightDataset, applyScenarios, applyPatches, approved bool) (core.Config, error) {
	if requiresReview(dataset) && !approved {
		return core.Config{}, fmt.Errorf("braintrust sync insight requires explicit review; rerun with approval enabled after review")
	}

	cfg := base
	if applyScenarios && dataset.ScenarioDataset != nil && len(dataset.ScenarioDataset.Scenarios) > 0 {
		cfg = MergeDatasetIntoConfig(cfg, *dataset.ScenarioDataset)
	}
	if applyPatches && dataset.ConfigPatch != nil && len(dataset.ConfigPatch.Operations) > 0 {
		patched, err := ApplyBraintrustConfigPatchSet(cfg, *dataset.ConfigPatch)
		if err != nil {
			return core.Config{}, err
		}
		cfg = patched
	}
	return cfg, nil
}

func ApplyBraintrustConfigPatchSet(base core.Config, patch BraintrustConfigPatchSet) (core.Config, error) {
	raw, err := json.Marshal(base)
	if err != nil {
		return core.Config{}, fmt.Errorf("apply config patch: %w", err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return core.Config{}, fmt.Errorf("apply config patch: %w", err)
	}
	for _, op := range patch.Operations {
		if err := applyConfigPatchOperation(generic, op); err != nil {
			return core.Config{}, err
		}
	}
	raw, err = json.Marshal(generic)
	if err != nil {
		return core.Config{}, fmt.Errorf("apply config patch: %w", err)
	}
	var cfg core.Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return core.Config{}, fmt.Errorf("apply config patch: %w", err)
	}
	return cfg, nil
}

func encodeBraintrustInsightDataset(dataset BraintrustInsightDataset, path string) ([]byte, error) {
	if isYAMLPath(path) {
		raw, err := json.Marshal(dataset)
		if err != nil {
			return nil, fmt.Errorf("encode braintrust insight dataset: %w", err)
		}
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			return nil, fmt.Errorf("encode braintrust insight dataset: %w", err)
		}
		data, err := yaml.Marshal(generic)
		if err != nil {
			return nil, fmt.Errorf("encode braintrust insight dataset: %w", err)
		}
		return data, nil
	}
	data, err := json.MarshalIndent(dataset, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode braintrust insight dataset: %w", err)
	}
	return data, nil
}

func mergeBraintrustInsights(base, remote BraintrustInsightDataset) BraintrustInsightDataset {
	if strings.TrimSpace(remote.Version) != "" {
		base.Version = remote.Version
	}
	if strings.TrimSpace(remote.Source) != "" {
		base.Source = remote.Source
	}
	if strings.TrimSpace(remote.Project) != "" {
		base.Project = remote.Project
	}
	if strings.TrimSpace(remote.Experiment) != "" {
		base.Experiment = remote.Experiment
	}
	if strings.TrimSpace(remote.ExperimentID) != "" {
		base.ExperimentID = remote.ExperimentID
	}
	if strings.TrimSpace(remote.ExperimentURL) != "" {
		base.ExperimentURL = remote.ExperimentURL
	}
	if strings.TrimSpace(remote.BuildID) != "" {
		base.BuildID = remote.BuildID
	}
	if !remote.GeneratedAt.IsZero() {
		base.GeneratedAt = remote.GeneratedAt.UTC()
	}
	base.ReviewRequired = base.ReviewRequired || remote.ReviewRequired
	base.Warnings = append(base.Warnings, remote.Warnings...)
	if remote.ScenarioDataset != nil {
		if base.ScenarioDataset == nil {
			copyDataset := *remote.ScenarioDataset
			base.ScenarioDataset = &copyDataset
		} else {
			merged := mergeScenarioDatasets(*base.ScenarioDataset, *remote.ScenarioDataset)
			base.ScenarioDataset = &merged
		}
	}
	if remote.ConfigPatch != nil {
		if base.ConfigPatch == nil {
			copyPatch := *remote.ConfigPatch
			base.ConfigPatch = &copyPatch
		} else {
			base.ConfigPatch.ReviewRequired = base.ConfigPatch.ReviewRequired || remote.ConfigPatch.ReviewRequired
			base.ConfigPatch.Operations = append(base.ConfigPatch.Operations, remote.ConfigPatch.Operations...)
		}
	}
	return base
}

func mergeScenarioDatasets(base, remote ScenarioDataset) ScenarioDataset {
	if strings.TrimSpace(base.Version) == "" {
		base.Version = remote.Version
	}
	if strings.TrimSpace(base.Source) == "" {
		base.Source = remote.Source
	}
	if strings.TrimSpace(base.Target) == "" {
		base.Target = remote.Target
	}
	if strings.TrimSpace(base.BuildID) == "" {
		base.BuildID = remote.BuildID
	}
	if base.GeneratedAt.IsZero() {
		base.GeneratedAt = remote.GeneratedAt
	}
	base.ReviewRequired = base.ReviewRequired || remote.ReviewRequired
	base.Warnings = append(base.Warnings, remote.Warnings...)
	merged := MergeDatasetIntoConfig(core.Config{Scenarios: scenariosFromDataset(base)}, remote)
	base.Scenarios = make([]ScenarioDatasetEntry, 0, len(merged.Scenarios))
	for _, scenario := range merged.Scenarios {
		base.Scenarios = append(base.Scenarios, ScenarioDatasetEntry{Scenario: scenario})
	}
	return base
}

func scenariosFromDataset(dataset ScenarioDataset) []core.Scenario {
	out := make([]core.Scenario, 0, len(dataset.Scenarios))
	for _, item := range dataset.Scenarios {
		out = append(out, item.Scenario)
	}
	return out
}

func requiresReview(dataset BraintrustInsightDataset) bool {
	if dataset.ReviewRequired {
		return true
	}
	if dataset.ScenarioDataset != nil && dataset.ScenarioDataset.ReviewRequired {
		return true
	}
	if dataset.ConfigPatch != nil && dataset.ConfigPatch.ReviewRequired {
		return true
	}
	return false
}
