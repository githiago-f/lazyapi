package requesteditor

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
)

type params struct {
	active bool
}

// SetActive implements [tabs.StatefulInputBase].
func (p params) SetActive(b bool) tabs.StatefulInputBase {
	p.active = b
	return p
}

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
