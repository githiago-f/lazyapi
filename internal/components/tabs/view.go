package tabs

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/inmath"
)

type Model struct {
	tabs   []tab
	cursor int
	Width  int
}

func New(tabs ...tab) Model {
	return Model{
		tabs:   tabs,
		cursor: 0,
	}
}

var style = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder())

func (t Model) Value() int {
	return t.cursor
}

var (
	defaultTabStyle = lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			Padding(0, 2).
			Margin(0, 1)
	activeTabStyle   = defaultTabStyle.BorderForeground(lipgloss.Color(config.Peach))
	inactiveTabStyle = defaultTabStyle.BorderForeground(lipgloss.Color(config.Overlay1))
)

func (t Model) View() string {
	tabs := ""
	for i, tab := range t.tabs {
		tab.Style = inactiveTabStyle
		if i == t.cursor {
			tab.Style = activeTabStyle
		}

		tabs = lipgloss.JoinHorizontal(lipgloss.Left, tabs, tab.View())
	}

	tabContent := t.tabs[t.cursor].content

	return style.Width(t.Width).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				tabs,
				tabContent.View(),
			),
		)
}

func (t Model) Init() tea.Cmd {
	return nil
}

func (t Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Left):
			t.cursor = inmath.Cicle(t.cursor-1, 0, len(t.tabs)-1)
		case key.Matches(msg, config.DefaultKeyMap.Right):
			t.cursor = inmath.Cicle(t.cursor+1, 0, len(t.tabs)-1)
		}
	}

	var cmd tea.Cmd
	t.tabs[t.cursor].content, cmd = t.tabs[t.cursor].content.Update(msg)
	return t, cmd
}
