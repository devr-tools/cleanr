package cli

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devr-tools/cleanr/cleanr"
)

func TestDatasetReviewTUIModelSupportsNavigationAndActions(t *testing.T) {
	model := newDatasetReviewTUIModel(cleanr.ReviewedScenarioDataset{
		Scenarios: []cleanr.ReviewedScenarioEntry{
			{
				Entry: cleanr.ScenarioDatasetEntry{
					Scenario: cleanr.Scenario{Name: "candidate-one", Tags: []string{"generated"}},
				},
				Diff:     cleanr.DatasetReviewDiff{Status: "new"},
				Analysis: cleanr.DatasetReviewAnalysis{UsefulnessScore: 9, HighestSeverity: "high", StableSuitability: "medium"},
				Decision: cleanr.DatasetReviewDecision{Status: "pending"},
			},
			{
				Entry: cleanr.ScenarioDatasetEntry{
					Scenario: cleanr.Scenario{Name: "candidate-two", Tags: []string{"generated"}},
				},
				Diff:     cleanr.DatasetReviewDiff{Status: "duplicate"},
				Analysis: cleanr.DatasetReviewAnalysis{UsefulnessScore: 4},
				Decision: cleanr.DatasetReviewDecision{Status: "pending"},
			},
		},
	})

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	model = next.(datasetReviewTUIModel)
	if model.inputMode != reviewInputMetadata {
		t.Fatalf("expected metadata input mode, got %q", model.inputMode)
	}
	model.inputValue = "owner=qa"
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(datasetReviewTUIModel)
	if model.current().Entry.Scenario.Metadata["owner"] != "qa" {
		t.Fatalf("expected metadata to be applied, got %+v", model.current().Entry.Scenario.Metadata)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	model = next.(datasetReviewTUIModel)
	model.inputValue = "manual"
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(datasetReviewTUIModel)
	if !containsString(model.current().Entry.Scenario.Tags, "manual") {
		t.Fatalf("expected tag to be added, got %+v", model.current().Entry.Scenario.Tags)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model = next.(datasetReviewTUIModel)
	if model.reviewed.ApprovedScenarios != 1 {
		t.Fatalf("expected approved count 1, got %d", model.reviewed.ApprovedScenarios)
	}
	if model.index != 1 {
		t.Fatalf("expected selection to advance, got %d", model.index)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model = next.(datasetReviewTUIModel)
	if model.reviewed.RejectedScenarios != 1 {
		t.Fatalf("expected rejected count 1, got %d", model.reviewed.RejectedScenarios)
	}
}

func TestDatasetReviewTUIViewUsesStructuredLayout(t *testing.T) {
	model := newDatasetReviewTUIModel(cleanr.ReviewedScenarioDataset{
		PolicyPath:        "cleanr.review.yaml",
		ApprovedScenarios: 1,
		RejectedScenarios: 1,
		PendingScenarios:  1,
		Scenarios: []cleanr.ReviewedScenarioEntry{
			{
				Entry: cleanr.ScenarioDatasetEntry{
					Scenario: cleanr.Scenario{Name: "candidate-one", Tags: []string{"generated", "stable"}, Metadata: map[string]string{"owner": "qa"}},
				},
				Diff:     cleanr.DatasetReviewDiff{Status: "modified", Summary: []string{"input changed", "metadata changed"}},
				Analysis: cleanr.DatasetReviewAnalysis{UsefulnessScore: 9, HighestSeverity: "high", StableSuitability: "medium"},
				Decision: cleanr.DatasetReviewDecision{Status: "approved", PolicyRules: []string{"promote-replay"}},
			},
			{
				Entry:    cleanr.ScenarioDatasetEntry{Scenario: cleanr.Scenario{Name: "candidate-two"}},
				Diff:     cleanr.DatasetReviewDiff{Status: "duplicate"},
				Analysis: cleanr.DatasetReviewAnalysis{UsefulnessScore: 4},
				Decision: cleanr.DatasetReviewDecision{Status: "pending"},
			},
		},
	})
	model.width = 120
	model.height = 32
	view := model.View()
	for _, want := range []string{
		"cleanr dataset review",
		"Queue",
		"Candidate",
		"policy: cleanr.review.yaml",
		"candidate-one",
		"promote-replay",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in view:\n%s", want, view)
		}
	}
}
