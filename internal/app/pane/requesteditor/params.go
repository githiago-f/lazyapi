package requesteditor

import tea "github.com/charmbracelet/bubbletea"

type params struct{}

func ParamsTab() params {
	return params{}
}

func (p params) View() string {
	return "Params"
}

func (p params) Init() tea.Cmd {
	return nil
}

func (p params) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p, nil
}
