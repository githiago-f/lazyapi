package components

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Text struct {
	Style    lipgloss.Style
	TextArea textarea.Model
}

var defaultTextStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder())

func NewTextarea(placeholder, value string) Text {
	t := textarea.New()
	t.Placeholder = placeholder
	t.Prompt = ""
	t.ShowLineNumbers = false
	t.SetValue(value)
	t.Focus()

	return Text{
		TextArea: t,
	}
}

func (t Text) Value() string {
	return t.TextArea.Value()
}

func (t Text) SetValue(s string) {
	t.TextArea.SetValue(s)
}

func (t Text) View() string {
	return t.Style.Inherit(defaultTextStyle).Render(t.TextArea.View())
}

func (t Text) Init() tea.Cmd {
	return nil
}

func (t Text) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	t.TextArea, cmd = t.TextArea.Update(msg)
	return t, cmd
}
