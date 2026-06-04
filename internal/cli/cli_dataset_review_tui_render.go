package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m datasetReviewTUIModel) View() string {
	if len(m.reviewed.Scenarios) == 0 {
		return "\n  No review candidates.\n"
	}

	width := m.width
	if width <= 0 {
		width = 110
	}
	if width < 90 {
		return m.renderNarrow(width)
	}
	return m.renderWide(width)
}

func (m datasetReviewTUIModel) renderWide(width int) string {
	leftWidth := 34
	if width < 120 {
		leftWidth = 30
	}

	rightWidth := width - leftWidth - 5
	header := m.renderHeader(width)
	queue := m.renderQueue(leftWidth, 18)
	details := m.renderDetails(rightWidth, 18)
	body := lipgloss.JoinHorizontal(lipgloss.Top, queue, details)
	footer := m.renderFooter(width)
	lower := m.styles.doc.Render(lipgloss.JoinVertical(lipgloss.Left, body, footer))

	return lipgloss.JoinVertical(lipgloss.Left, header, lower)
}

func (m datasetReviewTUIModel) renderNarrow(width int) string {
	header := m.renderHeader(width)
	queue := m.renderQueue(width, 8)
	details := m.renderDetails(width, 14)
	footer := m.renderFooter(width)
	lower := m.styles.doc.Render(lipgloss.JoinVertical(lipgloss.Left, queue, details, footer))

	return lipgloss.JoinVertical(lipgloss.Left, header, lower)
}

func (m datasetReviewTUIModel) renderHeader(width int) string {
	if width < 40 {
		width = 40
	}

	lines := []string{
		m.styles.title.Render("cleanr dataset review"),
		m.styles.subtle.Render(fmt.Sprintf("candidate %d/%d", m.index+1, len(m.reviewed.Scenarios))),
		m.styles.subtle.Render(fmt.Sprintf("%d approved   %d rejected   %d pending", m.reviewed.ApprovedScenarios, m.reviewed.RejectedScenarios, m.reviewed.PendingScenarios)),
		m.styles.subtle.Render("policy: " + nonEmpty(strings.TrimSpace(m.reviewed.PolicyPath), "manual or none")),
	}

	rendered := make([]string, 0, len(lines))
	rowStyle := m.styles.headerBar.Width(width)
	for _, line := range lines {
		rendered = append(rendered, rowStyle.Render(line))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rendered...)
}

func (m datasetReviewTUIModel) renderQueue(width, height int) string {
	items := make([]string, 0, len(m.reviewed.Scenarios))
	for i, item := range m.reviewed.Scenarios {
		label := fmt.Sprintf("%s %s", decisionGlyph(item.Decision.Status), item.Entry.Scenario.Name)
		line := lipgloss.NewStyle().Width(width - 4).MaxWidth(width - 4).Render(label)
		if i == m.index {
			line = m.styles.listActive.Render(" " + line + " ")
		} else {
			line = m.styles.listItem.Render(" " + line + " ")
		}
		items = append(items, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.panel.Width(width).Height(height).Render(
		lipgloss.JoinVertical(lipgloss.Left, m.styles.panelTitle.Render("Queue"), content),
	)
}

func (m datasetReviewTUIModel) renderDetails(width, height int) string {
	current := m.current()
	lines := []string{
		m.renderField("Name", current.Entry.Scenario.Name, false),
		m.renderField("Decision", current.Decision.Status, true),
		m.renderField("Diff", current.Diff.Status, false),
		m.renderField("Score", fmt.Sprintf("%d", current.Analysis.UsefulnessScore), false),
		m.renderField("Severity", nonEmpty(current.Analysis.HighestSeverity, "none"), false),
		m.renderField("Stable", nonEmpty(current.Analysis.StableSuitability, "low"), false),
	}
	if len(current.Entry.Scenario.Tags) > 0 {
		lines = append(lines, m.renderWrappedField("Tags", strings.Join(current.Entry.Scenario.Tags, ", "), width-6))
	}
	if len(current.Diff.Summary) > 0 {
		lines = append(lines, m.renderWrappedField("Changes", strings.Join(current.Diff.Summary, "; "), width-6))
	}
	if len(current.Entry.Scenario.Metadata) > 0 {
		lines = append(lines, m.renderWrappedField("Metadata", formatMetadata(current.Entry.Scenario.Metadata), width-6))
	}
	if len(current.Decision.PolicyRules) > 0 {
		lines = append(lines, m.renderWrappedField("Rules", strings.Join(current.Decision.PolicyRules, ", "), width-6))
	}
	if current.Diff.DuplicateOf != "" {
		lines = append(lines, m.renderWrappedField("Duplicate", current.Diff.DuplicateOf, width-6))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	if m.inputMode != reviewInputNone {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			"",
			m.styles.inputBox.Width(width-4).Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					m.styles.panelTitle.Render(m.inputPromptTitle()),
					m.styles.value.Render("> "+m.inputValue),
				),
			),
		)
	}

	return m.styles.panel.Width(width).Height(height).Render(
		lipgloss.JoinVertical(lipgloss.Left, m.styles.panelTitle.Render("Candidate"), content),
	)
}

func (m datasetReviewTUIModel) renderFooter(width int) string {
	content := []string{
		m.styles.message.Render(m.message),
		m.styles.helpText.Render("a approve  r reject  p pending  s stable  g regression  t tag  e set tags  m metadata  ↑/↓ move  ? help  q quit"),
	}
	if m.showHelp {
		content = append(
			content,
			m.styles.helpText.Render("Quick actions: a approve, r reject, p pending, s stable, g regression"),
			m.styles.helpText.Render("Edit current candidate: t add tag, e replace tags, m set metadata, esc cancel input"),
			m.styles.helpText.Render("Navigation: ↑/k previous, ↓/j next, enter apply input, q quit"),
		)
	}

	return m.styles.panel.Width(width - 2).Render(lipgloss.JoinVertical(lipgloss.Left, content...))
}

func (m datasetReviewTUIModel) renderField(label, value string, colorize bool) string {
	valueStyle := m.styles.value
	if colorize {
		valueStyle = m.decisionStyle(value)
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.styles.fieldLabel.Render(strings.ToUpper(label)),
		valueStyle.Render(nonEmpty(value, "pending")),
	)
}

func (m datasetReviewTUIModel) renderWrappedField(label, value string, width int) string {
	lines := wrapTextForTUI(value, width-12)
	rendered := make([]string, 0, len(lines))
	for i, line := range lines {
		fieldLabel := ""
		if i == 0 {
			fieldLabel = strings.ToUpper(label)
		}
		rendered = append(
			rendered,
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				m.styles.fieldLabel.Render(fieldLabel),
				m.styles.value.Render(line),
			),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rendered...)
}

func (m datasetReviewTUIModel) inputPromptTitle() string {
	switch m.inputMode {
	case reviewInputTag:
		return "Add tag"
	case reviewInputTags:
		return "Replace tags"
	case reviewInputMetadata:
		return "Set metadata"
	default:
		return "Input"
	}
}

func wrapTextForTUI(value string, width int) []string {
	if width < 12 {
		return []string{value}
	}

	words := strings.Fields(value)
	if len(words) == 0 {
		return []string{""}
	}

	lines := []string{words[0]}
	for _, word := range words[1:] {
		current := lines[len(lines)-1]
		if lipgloss.Width(current)+1+lipgloss.Width(word) <= width {
			lines[len(lines)-1] = current + " " + word
			continue
		}
		lines = append(lines, word)
	}

	return lines
}
