package devtools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

func (r Runner) ReviewUIPreview(ctx context.Context) error {
	dir, err := os.MkdirTemp("", "cleanr-review-ui-*")
	if err != nil {
		return fmt.Errorf("create preview temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	paths := reviewUIPreviewPaths{
		config:  filepath.Join(dir, "cleanr.yaml"),
		dataset: filepath.Join(dir, "cleanr.dataset.yaml"),
		output:  filepath.Join(dir, "cleanr.reviewed.yaml"),
		merge:   filepath.Join(dir, "cleanr.reviewed-config.yaml"),
	}

	cfg := buildReviewUIPreviewConfig()
	if err := writeReviewUIPreviewConfig(paths.config, cfg); err != nil {
		return err
	}
	if err := writeReviewUIPreviewDataset(paths.dataset, cfg); err != nil {
		return err
	}

	return r.runInteractiveCommand(
		ctx,
		nil,
		"go",
		"run", "./cmd/cleanr",
		"dataset", "review",
		"-interactive",
		"-input", paths.dataset,
		"-base-config", paths.config,
		"-output", paths.output,
		"-merge-output", paths.merge,
	)
}

type reviewUIPreviewPaths struct {
	config  string
	dataset string
	output  string
	merge   string
}

func buildReviewUIPreviewConfig() cleanr.Config {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{
		{
			Name:             "existing-password-reset",
			System:           "You are a careful support assistant.",
			Input:            "Help me reset my password.",
			Tags:             []string{"stable"},
			ExpectedContains: []string{"reset"},
		},
	}
	return cfg
}

func writeReviewUIPreviewConfig(configPath string, cfg cleanr.Config) error {
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		return fmt.Errorf("write preview config: %w", err)
	}
	return nil
}

func writeReviewUIPreviewDataset(datasetPath string, cfg cleanr.Config) error {
	if err := cleanr.WriteScenarioDatasetFile(datasetPath, buildReviewUIPreviewDataset(cfg)); err != nil {
		return fmt.Errorf("write preview dataset: %w", err)
	}
	return nil
}

func buildReviewUIPreviewDataset(cfg cleanr.Config) cleanr.ScenarioDataset {
	return cleanr.ScenarioDataset{
		Version:     "v1alpha1",
		Source:      "cleanr-replay",
		Target:      cfg.Target.Name,
		BuildID:     "preview-build-1",
		GeneratedAt: time.Now().UTC(),
		Scenarios: []cleanr.ScenarioDatasetEntry{
			reviewUIPreviewDatasetEntry(reviewUIPreviewEntrySpec{
				name:     "candidate-hours",
				input:    "Summarize support hours in one sentence.",
				tags:     []string{"generated"},
				expected: []string{"weekday"},
				severity: "high",
				message:  "new issue surfaced in replay",
			}),
			reviewUIPreviewDatasetEntry(reviewUIPreviewEntrySpec{
				name:     "candidate-password-duplicate",
				input:    "Help me reset my password.",
				tags:     []string{"generated"},
				severity: "medium",
				message:  "likely duplicate regression candidate",
			}),
			reviewUIPreviewDatasetEntry(reviewUIPreviewEntrySpec{
				name:     "candidate-lockout",
				input:    "What should I do after three failed login attempts?",
				tags:     []string{"generated", "security"},
				expected: []string{"contact support"},
				severity: "critical",
				message:  "account lockout flow regressed",
			}),
		},
	}
}

type reviewUIPreviewEntrySpec struct {
	name     string
	input    string
	tags     []string
	expected []string
	severity string
	message  string
}

func reviewUIPreviewDatasetEntry(spec reviewUIPreviewEntrySpec) cleanr.ScenarioDatasetEntry {
	return cleanr.ScenarioDatasetEntry{
		Scenario: cleanr.Scenario{
			Name:             spec.name,
			System:           "You are a careful support assistant.",
			Input:            spec.input,
			Tags:             spec.tags,
			ExpectedContains: spec.expected,
		},
		Origin: cleanr.DatasetScenarioOrigin{
			Suite: "replay",
			Case:  spec.name,
			Findings: []cleanr.Finding{{
				Severity: spec.severity,
				Message:  spec.message,
			}},
		},
	}
}
