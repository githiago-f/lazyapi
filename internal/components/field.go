package components

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Field struct {
	tea.Model
	textField textinput.Model
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

	return Field{textField: ti}
}

func (f Field) Init() tea.Cmd {
	return nil
}

func (f Field) View() string {
	return f.Style.Inherit(defaultStyle).Render(f.textField.View())
}

func (f Field) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	f.textField, cmd = f.textField.Update(msg)
	return f, cmd
}
