// Package tabs define a tab component
package tabs

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tab struct {
	label string

	content tea.Model

	Style lipgloss.Style
}

func NewTab(label string, content tea.Model) tab {
	return tab{label: label, content: content}
}

func (t tab) View() string {
	return t.Style.Render(t.label)
}

func (t tab) Init() tea.Cmd {
	return nil
}

func (t tab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return t, nil
}
