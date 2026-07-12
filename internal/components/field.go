package components

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Field struct {
	tea.Model
	TextInput textinput.Model
	Focused   bool
	Style     lipgloss.Style
}

var defaultStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder())

func InitField(placeholder, value string) Field {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = ""
	ti.SetValue(value)

	ti.Focus()

	return Field{TextInput: ti}
}

func (f Field) Value() string {
	return f.TextInput.Value()
}

func (f *Field) SetValue(s string) {
	f.TextInput.SetValue(s)
}

func (f Field) Init() tea.Cmd {
	return nil
}

func (f Field) View() tea.View {
	return tea.NewView(f.Style.Inherit(defaultStyle).Render(f.TextInput.View()))
}

func (f Field) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	f.TextInput, cmd = f.TextInput.Update(msg)
	return f, cmd
}
