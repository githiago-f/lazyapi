package components

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/inmath"
)

type Selector struct {
	Cursor int
	Labels []string
	Style  lipgloss.Style
	Width  int
}

var selectorStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	Padding(0, 2).
	Align(lipgloss.Center)

func (s Selector) Value() string {
	if s.Cursor < 0 || s.Cursor >= len(s.Labels) {
		return ""
	}
	return s.Labels[s.Cursor]
}

func (s Selector) Init() tea.Cmd {
	return nil
}

func (s Selector) View() string {
	label := s.Value()
	style := s.Style.Inherit(selectorStyle)
	if s.Width > 0 {
		style = style.Width(s.Width)
	}
	return style.Render(label)
}

func (s Selector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Up):
			s.Cursor = inmath.Cicle(s.Cursor-1, 0, len(s.Labels)-1)
		case key.Matches(msg, config.DefaultKeyMap.Down):
			s.Cursor = inmath.Cicle(s.Cursor+1, 0, len(s.Labels)-1)
		}
	}
	return s, nil
}
