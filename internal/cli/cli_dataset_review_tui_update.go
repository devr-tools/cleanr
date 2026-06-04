package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m datasetReviewTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.inputMode != reviewInputNone {
		return m.updateInputMode(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	default:
		return m, nil
	}
}

func (m datasetReviewTUIModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Quit):
		m.abort = true
		return m, tea.Quit
	case keyMatches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		return m, nil
	case keyMatches(msg, m.keys.Up):
		return m.moveSelection(-1), nil
	case keyMatches(msg, m.keys.Down):
		return m.moveSelection(1), nil
	case keyMatches(msg, m.keys.Approve):
		m.applyDecision("approved")
		return m.afterScenarioAction("Approved " + m.current().Entry.Scenario.Name)
	case keyMatches(msg, m.keys.Reject):
		m.applyDecision("rejected")
		return m.afterScenarioAction("Rejected " + m.current().Entry.Scenario.Name)
	case keyMatches(msg, m.keys.Pending):
		m.applyDecision("pending")
		return m.afterScenarioAction("Marked " + m.current().Entry.Scenario.Name + " pending")
	case keyMatches(msg, m.keys.Stable):
		m.applyDecision("approved")
		addInteractiveTag(m.current(), "stable")
		return m.afterScenarioAction("Promoted " + m.current().Entry.Scenario.Name + " to stable")
	case keyMatches(msg, m.keys.Regression):
		m.applyDecision("approved")
		addInteractiveTag(m.current(), "regression")
		return m.afterScenarioAction("Promoted " + m.current().Entry.Scenario.Name + " to regression")
	case keyMatches(msg, m.keys.Tag):
		return m.startInput(reviewInputTag)
	case keyMatches(msg, m.keys.Tags):
		return m.startInput(reviewInputTags)
	case keyMatches(msg, m.keys.Metadata):
		return m.startInput(reviewInputMetadata)
	default:
		return m, nil
	}
}

func keyMatches(msg tea.KeyMsg, binding interface{ Keys() []string }) bool {
	for _, candidate := range binding.Keys() {
		if msg.String() == candidate {
			return true
		}
	}
	return false
}

func (m datasetReviewTUIModel) moveSelection(delta int) datasetReviewTUIModel {
	nextIndex := m.index + delta
	if nextIndex < 0 || nextIndex >= len(m.reviewed.Scenarios) {
		return m
	}
	m.index = nextIndex
	return m.withMessage("Moved to " + m.current().Entry.Scenario.Name)
}

func (m datasetReviewTUIModel) updateInputMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.updateInputKey(msg)
	default:
		return m, nil
	}
}

func (m datasetReviewTUIModel) updateInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.inputMode = reviewInputNone
		m.inputValue = ""
		m.message = "Cancelled input."
		return m, nil
	case keyMatches(msg, m.keys.Submit):
		return m.applyInputValue()
	case msg.Type == tea.KeyBackspace || msg.Type == tea.KeyCtrlH:
		runes := []rune(m.inputValue)
		if len(runes) > 0 {
			m.inputValue = string(runes[:len(runes)-1])
		}
		return m, nil
	case msg.Type == tea.KeyRunes:
		m.inputValue += string(msg.Runes)
		return m, nil
	default:
		return m, nil
	}
}

func (m datasetReviewTUIModel) applyInputValue() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.inputValue)
	current := m.current()

	switch m.inputMode {
	case reviewInputTag:
		if value == "" {
			m.message = "Tag input cannot be empty."
			return m, nil
		}
		addInteractiveTag(current, value)
		m.message = fmt.Sprintf("Added tag %q to %s", value, current.Entry.Scenario.Name)
	case reviewInputTags:
		tags := splitInteractiveList(value)
		if len(tags) == 0 {
			m.message = "Tags input cannot be empty."
			return m, nil
		}
		setInteractiveTags(current, tags)
		m.message = "Replaced tags."
	case reviewInputMetadata:
		key, fieldValue, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(key) == "" {
			m.message = "Metadata input must be key=value."
			return m, nil
		}
		setInteractiveMetadata(current, strings.TrimSpace(key), strings.TrimSpace(fieldValue))
		m.message = fmt.Sprintf("Updated metadata %s.", strings.TrimSpace(key))
	}

	m.inputMode = reviewInputNone
	m.inputValue = ""
	return m, nil
}

func (m datasetReviewTUIModel) startInput(mode reviewInputMode) (tea.Model, tea.Cmd) {
	m.inputMode = mode
	m.inputValue = ""
	return m, nil
}
