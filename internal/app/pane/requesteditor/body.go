package requesteditor

import tea "github.com/charmbracelet/bubbletea"

type body struct{}

func BodyTab() body {
	return body{}
}

func (b body) Init() tea.Cmd {
	return nil
}

func (b body) View() string {
	return "Body"
}

func (b body) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return b, nil
}
