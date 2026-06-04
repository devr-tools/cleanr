package tests

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/cli"
)

func TestDatasetReviewTUIModelSupportsNavigationAndActions(t *testing.T) {
	reviewed, index, _, err := cli.RunDatasetReviewTUISequenceForTest(
		cleanr.ReviewedScenarioDataset{
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
		},
		[]tea.Msg{
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}},
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("owner=qa")},
			tea.KeyMsg{Type: tea.KeyEnter},
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}},
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("manual")},
			tea.KeyMsg{Type: tea.KeyEnter},
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}},
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}},
		},
	)
	if err != nil {
		t.Fatalf("run review ui sequence: %v", err)
	}
	if reviewed.ApprovedScenarios != 1 {
		t.Fatalf("expected approved count 1, got %d", reviewed.ApprovedScenarios)
	}
	if reviewed.RejectedScenarios != 1 {
		t.Fatalf("expected rejected count 1, got %d", reviewed.RejectedScenarios)
	}
	if index != 1 {
		t.Fatalf("expected selection to advance to second candidate, got %d", index)
	}
	first := reviewed.Scenarios[0]
	if first.Entry.Scenario.Metadata["owner"] != "qa" {
		t.Fatalf("expected metadata to be applied, got %+v", first.Entry.Scenario.Metadata)
	}
	if !strings.Contains(strings.Join(first.Entry.Scenario.Tags, ","), "manual") {
		t.Fatalf("expected manual tag to be added, got %+v", first.Entry.Scenario.Tags)
	}
}

func TestDatasetReviewTUIViewUsesStructuredLayout(t *testing.T) {
	view := cli.RenderDatasetReviewTUIViewForTest(cleanr.ReviewedScenarioDataset{
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
	}, 120, 32)
	for _, want := range []string{
		"cleanr dataset review",
		"candidate 1/2",
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
