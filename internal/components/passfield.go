package components

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type PassField struct {
	Field
	ShowContent bool
}

var passToggleKey = key.NewBinding(key.WithKeys("ctrl+p"))

func InitPassField(placeholder, value string, showContent bool) PassField {
	f := InitField(placeholder, value)
	if !showContent {
		f.TextInput.EchoMode = textinput.EchoPassword
	}
	return PassField{Field: f, ShowContent: showContent}
}

func (p PassField) ToggleVisibility() PassField {
	p.ShowContent = !p.ShowContent
	if p.ShowContent {
		p.TextInput.EchoMode = textinput.EchoNormal
	} else {
		p.TextInput.EchoMode = textinput.EchoPassword
	}
	return p
}

func (p PassField) View() string {
	indicator := "○"
	if !p.ShowContent {
		indicator = "●"
	}
	return p.Style.Inherit(defaultStyle).Render(
		p.TextInput.View() + " " + indicator,
	)
}

func (p PassField) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, passToggleKey) {
			return p.ToggleVisibility(), nil
		}
	}
	m, cmd := p.Field.Update(msg)
	p.Field = m.(Field)
	return p, cmd
}
