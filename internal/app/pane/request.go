// Package pane implements pages for request, response and others.
package pane

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
)

type CloseRequestPaneMsg int

func Close() tea.Cmd {
	return func() tea.Msg {
		return CloseRequestPaneMsg(1)
	}
}

type field int

const (
	method field = iota
	uri
)

func (f field) next() field {
	switch f {
	case method:
		return uri
	case uri:
		return method
	}

	return method
}

type RequestPane struct {
	tea.Model
	config       config.Config
	currentField field

	URI    components.Field
	Method model.Method
}

func (rp RequestPane) View() string {
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		rp.Method.Label(),
		rp.URI.View(),
	)
}

func (rp RequestPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			rp.currentField = rp.currentField.next()
			return rp, nil
		case "esc":
			return rp, Close()
		}
	}

	var (
		cmd   tea.Cmd
		model tea.Model
	)
	switch rp.currentField {
	case method:
		// model, cmd = rp.Method.Update(msg)
		// rp.URI = model.(?)
	case uri:
		model, cmd = rp.URI.Update(msg)
		rp.URI, _ = model.(components.Field)
	}

	return rp, cmd
}
