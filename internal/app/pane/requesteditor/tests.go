package requesteditor

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
)

type testsPane struct {
	active bool
}

// SetActive implements [tabs.StatefulInputBase].
func (tp testsPane) SetActive(b bool) tabs.StatefulInputBase {
	tp.active = b
	return tp
}

func TestsTab() *testsPane {
	return &testsPane{}
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
