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

	configPath := filepath.Join(dir, "cleanr.yaml")
	datasetPath := filepath.Join(dir, "cleanr.dataset.yaml")
	outputPath := filepath.Join(dir, "cleanr.reviewed.yaml")
	mergePath := filepath.Join(dir, "cleanr.reviewed-config.yaml")

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
	if err := cleanr.WriteConfigFile(configPath, cfg); err != nil {
		return fmt.Errorf("write preview config: %w", err)
	}

	dataset := cleanr.ScenarioDataset{
		Version:     "v1alpha1",
		Source:      "cleanr-replay",
		Target:      cfg.Target.Name,
		BuildID:     "preview-build-1",
		GeneratedAt: time.Now().UTC(),
		Scenarios: []cleanr.ScenarioDatasetEntry{
			{
				Scenario: cleanr.Scenario{
					Name:             "candidate-hours",
					System:           "You are a careful support assistant.",
					Input:            "Summarize support hours in one sentence.",
					Tags:             []string{"generated"},
					ExpectedContains: []string{"weekday"},
				},
				Origin: cleanr.DatasetScenarioOrigin{
					Suite: "replay",
					Case:  "candidate-hours",
					Findings: []cleanr.Finding{{
						Severity: "high",
						Message:  "new issue surfaced in replay",
					}},
				},
			},
			{
				Scenario: cleanr.Scenario{
					Name:   "candidate-password-duplicate",
					System: "You are a careful support assistant.",
					Input:  "Help me reset my password.",
					Tags:   []string{"generated"},
				},
				Origin: cleanr.DatasetScenarioOrigin{
					Suite: "replay",
					Case:  "candidate-password-duplicate",
					Findings: []cleanr.Finding{{
						Severity: "medium",
						Message:  "likely duplicate regression candidate",
					}},
				},
			},
			{
				Scenario: cleanr.Scenario{
					Name:             "candidate-lockout",
					System:           "You are a careful support assistant.",
					Input:            "What should I do after three failed login attempts?",
					Tags:             []string{"generated", "security"},
					ExpectedContains: []string{"contact support"},
				},
				Origin: cleanr.DatasetScenarioOrigin{
					Suite: "replay",
					Case:  "candidate-lockout",
					Findings: []cleanr.Finding{{
						Severity: "critical",
						Message:  "account lockout flow regressed",
					}},
				},
			},
		},
	}
	if err := cleanr.WriteScenarioDatasetFile(datasetPath, dataset); err != nil {
		return fmt.Errorf("write preview dataset: %w", err)
	}

	return r.runInteractiveCommand(ctx, nil, "go",
		"run", "./cmd/cleanr",
		"dataset", "review",
		"-interactive",
		"-input", datasetPath,
		"-base-config", configPath,
		"-output", outputPath,
		"-merge-output", mergePath,
	)
}
