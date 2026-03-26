package requesteditor

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
)

type auth struct {
	active bool
}

// SetActive implements [tabs.StatefulInputBase].
func (a auth) SetActive(b bool) tabs.StatefulInputBase {
	a.active = b
	return a
}

func AuthorizeTab() auth {
	return auth{}
}

func (a auth) Init() tea.Cmd {
	return nil
}

func (a auth) View() string {
	return "Authorize"
}

func (a auth) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return a, nil
}
