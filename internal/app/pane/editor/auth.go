package editor

import (
	tea "github.com/charmbracelet/bubbletea"
)

type auth struct {
	active bool
}

func (a *auth) SetActive(b bool) {
	a.active = b
}

func AuthorizeTab() *auth {
	return &auth{}
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
