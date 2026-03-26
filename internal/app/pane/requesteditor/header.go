package requesteditor

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
)

type header struct {
	active bool
}

// SetActive implements [tabs.StatefulInputBase].
func (h header) SetActive(b bool) tabs.StatefulInputBase {
	h.active = b
	return h
}

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
