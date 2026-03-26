package requesteditor

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
)

type body struct {
	active bool
}

// SetActive implements [tabs.StatefulInputBase].
func (b body) SetActive(active bool) tabs.StatefulInputBase {
	b.active = active
	return b
}

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
