package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/devr-tools/cleanr/cleanr"
)

type reviewInputMode string

const (
	reviewInputNone     reviewInputMode = ""
	reviewInputTag      reviewInputMode = "tag"
	reviewInputTags     reviewInputMode = "tags"
	reviewInputMetadata reviewInputMode = "metadata"
)

type reviewTUIKeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Approve    key.Binding
	Reject     key.Binding
	Pending    key.Binding
	Stable     key.Binding
	Regression key.Binding
	Tag        key.Binding
	Tags       key.Binding
	Metadata   key.Binding
	Submit     key.Binding
	Cancel     key.Binding
	Quit       key.Binding
	Help       key.Binding
}

func defaultReviewTUIKeyMap() reviewTUIKeyMap {
	return reviewTUIKeyMap{
		Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Approve:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "approve")),
		Reject:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reject")),
		Pending:    key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pending")),
		Stable:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stable")),
		Regression: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "regression")),
		Tag:        key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tag")),
		Tags:       key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "set tags")),
		Metadata:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "metadata")),
		Submit:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply")),
		Cancel:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	}
}

type datasetReviewTUIModel struct {
	reviewed   cleanr.ReviewedScenarioDataset
	index      int
	width      int
	height     int
	message    string
	showHelp   bool
	inputMode  reviewInputMode
	inputValue string
	keys       reviewTUIKeyMap
	abort      bool
	styles     reviewTUIStyles
}

type reviewTUIStyles struct {
	doc            lipgloss.Style
	headerBar      lipgloss.Style
	title          lipgloss.Style
	banner         lipgloss.Style
	subtle         lipgloss.Style
	status         lipgloss.Style
	panel          lipgloss.Style
	panelTitle     lipgloss.Style
	listItem       lipgloss.Style
	listActive     lipgloss.Style
	decisionOK     lipgloss.Style
	decisionReject lipgloss.Style
	decisionWait   lipgloss.Style
	fieldLabel     lipgloss.Style
	value          lipgloss.Style
	message        lipgloss.Style
	helpText       lipgloss.Style
	inputBox       lipgloss.Style
}

func newReviewTUIStyles() reviewTUIStyles {
	border := lipgloss.RoundedBorder()
	return reviewTUIStyles{
		doc:            lipgloss.NewStyle().Padding(0, 1),
		headerBar:      lipgloss.NewStyle().Background(lipgloss.Color("#243B53")).Foreground(lipgloss.Color("#F7FAFC")).Padding(0, 1),
		title:          lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F7FAFC")),
		banner:         lipgloss.NewStyle().Foreground(lipgloss.Color("#E8F1F2")).UnsetWidth().MaxWidth(0).Inline(true),
		subtle:         lipgloss.NewStyle().Foreground(lipgloss.Color("#A0AEC0")),
		status:         lipgloss.NewStyle().Foreground(lipgloss.Color("#C6F6D5")),
		panel:          lipgloss.NewStyle().Border(border).BorderForeground(lipgloss.Color("#4A5568")).Padding(0, 1),
		panelTitle:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0")),
		listItem:       lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E0")),
		listActive:     lipgloss.NewStyle().Foreground(lipgloss.Color("#F7FAFC")).Background(lipgloss.Color("#2D3748")).Bold(true),
		decisionOK:     lipgloss.NewStyle().Foreground(lipgloss.Color("#38A169")).Bold(true),
		decisionReject: lipgloss.NewStyle().Foreground(lipgloss.Color("#E53E3E")).Bold(true),
		decisionWait:   lipgloss.NewStyle().Foreground(lipgloss.Color("#D69E2E")).Bold(true),
		fieldLabel:     lipgloss.NewStyle().Foreground(lipgloss.Color("#A0AEC0")).Width(10),
		value:          lipgloss.NewStyle().Foreground(lipgloss.Color("#F7FAFC")),
		message:        lipgloss.NewStyle().Foreground(lipgloss.Color("#F6E05E")),
		helpText:       lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E0")),
		inputBox:       lipgloss.NewStyle().Border(border).BorderForeground(lipgloss.Color("#2B6CB0")).Padding(0, 1),
	}
}

func newDatasetReviewTUIModel(reviewed cleanr.ReviewedScenarioDataset) datasetReviewTUIModel {
	return datasetReviewTUIModel{
		reviewed: reviewed,
		message:  "Review candidates and promote only the ones worth keeping.",
		keys:     defaultReviewTUIKeyMap(),
		styles:   newReviewTUIStyles(),
	}
}

func (m datasetReviewTUIModel) Init() tea.Cmd {
	return nil
}

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
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.abort = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.index > 0 {
				m.index--
			}
			return m.withMessage("Moved to " + m.current().Entry.Scenario.Name), nil
		case key.Matches(msg, m.keys.Down):
			if m.index < len(m.reviewed.Scenarios)-1 {
				m.index++
			}
			return m.withMessage("Moved to " + m.current().Entry.Scenario.Name), nil
		case key.Matches(msg, m.keys.Approve):
			m.applyDecision("approved")
			return m.afterScenarioAction("Approved " + m.current().Entry.Scenario.Name)
		case key.Matches(msg, m.keys.Reject):
			m.applyDecision("rejected")
			return m.afterScenarioAction("Rejected " + m.current().Entry.Scenario.Name)
		case key.Matches(msg, m.keys.Pending):
			m.applyDecision("pending")
			return m.afterScenarioAction("Marked " + m.current().Entry.Scenario.Name + " pending")
		case key.Matches(msg, m.keys.Stable):
			m.applyDecision("approved")
			addInteractiveTag(m.current(), "stable")
			return m.afterScenarioAction("Promoted " + m.current().Entry.Scenario.Name + " to stable")
		case key.Matches(msg, m.keys.Regression):
			m.applyDecision("approved")
			addInteractiveTag(m.current(), "regression")
			return m.afterScenarioAction("Promoted " + m.current().Entry.Scenario.Name + " to regression")
		case key.Matches(msg, m.keys.Tag):
			return m.startInput(reviewInputTag)
		case key.Matches(msg, m.keys.Tags):
			return m.startInput(reviewInputTags)
		case key.Matches(msg, m.keys.Metadata):
			return m.startInput(reviewInputMetadata)
		}
	}
	return m, nil
}

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

func (m datasetReviewTUIModel) updateInputMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Cancel):
			m.inputMode = reviewInputNone
			m.inputValue = ""
			m.message = "Cancelled input."
			return m, nil
		case key.Matches(msg, m.keys.Submit):
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
		}
	}
	return m, nil
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

func (m datasetReviewTUIModel) current() *cleanr.ReviewedScenarioEntry {
	return &m.reviewed.Scenarios[m.index]
}

func (m datasetReviewTUIModel) applyDecision(status string) {
	m.current().Decision.Status = status
}

func (m datasetReviewTUIModel) afterScenarioAction(message string) (tea.Model, tea.Cmd) {
	m.reviewed = cleanr.FinalizeReviewedScenarioDataset(m.reviewed)
	m.message = message
	if m.index >= len(m.reviewed.Scenarios)-1 {
		return m, tea.Quit
	}
	m.index++
	return m, nil
}

func (m datasetReviewTUIModel) withMessage(message string) datasetReviewTUIModel {
	m.message = message
	return m
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
	lineWidth := width
	lines := []string{
		m.styles.title.Render("cleanr dataset review"),
		m.styles.subtle.Render(fmt.Sprintf("candidate %d/%d", m.index+1, len(m.reviewed.Scenarios))),
		m.styles.subtle.Render(fmt.Sprintf("%d approved   %d rejected   %d pending", m.reviewed.ApprovedScenarios, m.reviewed.RejectedScenarios, m.reviewed.PendingScenarios)),
		m.styles.subtle.Render("policy: " + nonEmpty(strings.TrimSpace(m.reviewed.PolicyPath), "manual or none")),
	}
	rendered := make([]string, 0, len(lines))
	rowStyle := m.styles.headerBar.Width(lineWidth)
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
		content = lipgloss.JoinVertical(lipgloss.Left,
			content,
			"",
			m.styles.inputBox.Width(width-4).Render(
				lipgloss.JoinVertical(lipgloss.Left,
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
		content = append(content,
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
	return lipgloss.JoinHorizontal(lipgloss.Top, m.styles.fieldLabel.Render(strings.ToUpper(label)), valueStyle.Render(nonEmpty(value, "pending")))
}

func (m datasetReviewTUIModel) renderWrappedField(label, value string, width int) string {
	lines := wrapTextForTUI(value, width-12)
	rendered := make([]string, 0, len(lines))
	for i, line := range lines {
		fieldLabel := ""
		if i == 0 {
			fieldLabel = strings.ToUpper(label)
		}
		rendered = append(rendered, lipgloss.JoinHorizontal(lipgloss.Top, m.styles.fieldLabel.Render(fieldLabel), m.styles.value.Render(line)))
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

func (m datasetReviewTUIModel) decisionStyle(status string) lipgloss.Style {
	switch strings.TrimSpace(status) {
	case "approved":
		return m.styles.decisionOK
	case "rejected":
		return m.styles.decisionReject
	default:
		return m.styles.decisionWait
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

func decisionGlyph(status string) string {
	switch strings.TrimSpace(status) {
	case "approved":
		return "✓"
	case "rejected":
		return "✕"
	default:
		return "•"
	}
}
