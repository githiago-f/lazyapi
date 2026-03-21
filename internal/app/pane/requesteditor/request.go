// Package requesteditor implements editor page for request data
package requesteditor

import (
	"github.com/charmbracelet/bubbles/key"
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

const fieldsLen = 2

func (f field) next() field {
	return field((int(f) + 1) % fieldsLen)
}

func (f field) prev() field {
	prev := int(f) - 1
	if prev < 0 {
		return field(fieldsLen - 1)
	}
	return field(prev)
}

type RequestPane struct {
	tea.Model
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
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Next):
			rp.currentField = rp.currentField.next()
			return rp, nil
		case key.Matches(msg, config.DefaultKeyMap.Prev):
			rp.currentField = rp.currentField.prev()
			return rp, nil
		case key.Matches(msg, config.DefaultKeyMap.Back):
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
