package editor

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/config"
)

type testsPane struct {
	active bool
}

func (tp *testsPane) SetActive(b bool) {
	tp.active = b
}

func TestsTab() *testsPane {
	return &testsPane{}
}

func (tp testsPane) HelpBindings() []key.Binding {
	return []key.Binding{
		config.DefaultKeyMap.Back,
	}
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
