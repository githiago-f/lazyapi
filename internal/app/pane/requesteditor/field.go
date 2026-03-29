package requesteditor

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
)

type activeField int

const (
	none activeField = iota
	name
	value
)

type paramField struct {
	enabled bool
	active  activeField
	name    components.Field
	value   components.Field
}

func createParam() paramField {
	return paramField{
		enabled: true,
		active:  none,
		name:    components.InitField("Name", ""),
		value:   components.InitField("Value", ""),
	}
}

func (pf *paramField) SetWidth(w int) {
	pf.name.Style = pf.name.Style.Width(w - 1)
	pf.value.Style = pf.value.Style.Width(w)
}

func (pf paramField) Init() tea.Cmd {
	return nil
}

func (pf paramField) View() string {
	active := config.DefaultConfig.PrimaryColor()
	switch pf.active {
	case name:
		pf.name.Style = pf.name.Style.BorderForeground(active)
	case value:
		pf.value.Style = pf.value.Style.BorderForeground(active)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, pf.name.View(), pf.value.View())
}

func (pf paramField) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Next):
		case key.Matches(msg, config.DefaultKeyMap.Prev):
		}
	}

	var (
		model tea.Model
		cmd   tea.Cmd
	)
	switch pf.active {
	case none:
	case name:
		model, cmd = pf.name.Update(msg)
		pf.name = model.(components.Field)
	case value:
		model, cmd = pf.value.Update(msg)
		pf.value = model.(components.Field)
	}
	return pf, cmd
}
