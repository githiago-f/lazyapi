// Package pane implements pages for request, response and others.
package pane

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
)

type RequestPane struct {
	tea.Model
	config config.Config

	URL    components.Field
	Method string
	Send   components.Button
}

func (rp RequestPane) View() string {
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		rp.Method,
		rp.URL.View(),
		rp.Send.View(),
	)
}

func (rp RequestPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return rp, nil
}
