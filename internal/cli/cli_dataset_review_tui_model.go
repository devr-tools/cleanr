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
	subtle         lipgloss.Style
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
		subtle:         lipgloss.NewStyle().Foreground(lipgloss.Color("#A0AEC0")),
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

func RunDatasetReviewTUISequenceForTest(reviewed cleanr.ReviewedScenarioDataset, msgs []tea.Msg) (cleanr.ReviewedScenarioDataset, int, string, error) {
	model := newDatasetReviewTUIModel(reviewed)
	for _, msg := range msgs {
		next, _ := model.Update(msg)
		cast, ok := next.(datasetReviewTUIModel)
		if !ok {
			return cleanr.ReviewedScenarioDataset{}, 0, "", fmt.Errorf("unexpected review UI model type %T", next)
		}
		model = cast
	}
	return model.reviewed, model.index, model.View(), nil
}

func RenderDatasetReviewTUIViewForTest(reviewed cleanr.ReviewedScenarioDataset, width, height int) string {
	model := newDatasetReviewTUIModel(reviewed)
	model.width = width
	model.height = height
	return model.View()
}
