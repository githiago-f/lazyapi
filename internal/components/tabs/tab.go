// Package tabs define a tab component
package tabs

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StatefulInputBase interface {
	tea.Model
	SetActive(b bool) StatefulInputBase
}

type StatefulInput[T any] interface {
	StatefulInputBase
	SetValue(val T) StatefulInput[T]
	Value() T
}

type Tab struct {
	label string

	Active  bool
	Content StatefulInputBase

	Style lipgloss.Style
}

func NewTab(label string, content StatefulInputBase) Tab {
	return Tab{label: label, Content: content}
}

func (t Tab) View() string {
	return t.Style.Render(t.label)
}

func (t Tab) Init() tea.Cmd {
	return nil
}

func (t Tab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		model tea.Model
		cmd   tea.Cmd
	)
	model, cmd = t.Content.Update(msg)
	t.Content, _ = model.(StatefulInputBase)
	return t, cmd
}
