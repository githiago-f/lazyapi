package requesteditor

import tea "github.com/charmbracelet/bubbletea"

type header struct{}

func HeaderTab() header {
	return header{}
}

func (h header) Init() tea.Cmd {
	return nil
}

func (h header) View() string {
	return "Header"
}

func (h header) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return h, nil
}
