package components

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/inmath"
	"github.com/githiago-f/lazyapi/internal/model"
)

type MethodSelector struct {
	Cursor int
	Style  lipgloss.Style
}

var methodSelectorStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	Padding(0, 2).
	Width(9).
	Align(lipgloss.Center)

func (ms MethodSelector) Value() model.Method {
	return model.Method(ms.Cursor)
}

func (ms MethodSelector) Init() tea.Cmd {
	return nil
}

func (ms MethodSelector) View() string {
	method := model.Method(ms.Cursor).Label()
	return ms.Style.Inherit(methodSelectorStyle).Render(method)
}

func (ms MethodSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Up):
			ms.Cursor = inmath.Cicle(ms.Cursor-1, 0, int(model.LastMethod))
		case key.Matches(msg, config.DefaultKeyMap.Down):
			ms.Cursor = inmath.Cicle(ms.Cursor+1, 0, int(model.LastMethod))
		}
	}
	return ms, nil
}
