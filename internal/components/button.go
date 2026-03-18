package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/config"
)

type Button struct {
	tea.Model
	config config.Config
	Label  string
}

func (b Button) View() string {
	return b.Label
}

func (b Button) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return b, nil
}
