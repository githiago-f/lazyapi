package components

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

func (t *Text) SetValue(s string) {
	t.TextArea.SetValue(s)
}

func (t *Text) Focus() {
	t.TextArea.Focus()
}

func (t *Text) Blur() {
	t.TextArea.Blur()
}

func (t Text) View() tea.View {
	return tea.NewView(t.Style.Inherit(defaultTextStyle).Render(t.TextArea.View()))
}

func (t Text) Init() tea.Cmd {
	return nil
}

func (t Text) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	t.TextArea, cmd = t.TextArea.Update(msg)
	return t, cmd
}
