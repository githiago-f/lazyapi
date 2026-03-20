package components

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Field struct {
	tea.Model
	textField textinput.Model
}

func InitField(placeholder string) Field {
	ti := textinput.New()
	ti.Placeholder = placeholder

	ti.Focus()

	return Field{textField: ti}
}

func (f Field) View() string {
	return f.textField.View()
}

func (f Field) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	f.textField, cmd = f.textField.Update(msg)
	return f, cmd
}
