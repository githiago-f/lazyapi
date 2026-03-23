package requesteditor

import tea "github.com/charmbracelet/bubbletea"

type testsPane struct{}

func TestsTab() testsPane {
	return testsPane{}
}

func (tp testsPane) Init() tea.Cmd {
	return nil
}

func (tp testsPane) View() string {
	return "Tests"
}

func (tp testsPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return tp, nil
}
