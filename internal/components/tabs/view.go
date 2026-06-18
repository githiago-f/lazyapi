package tabs

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/inmath"
)

type SetActiveTabMsg struct {
	Active bool
}

type Model struct {
	selected bool
	Tabs     []Tab
	Cursor   int
	Width    int

	Style lipgloss.Style
}

func New(tabs ...Tab) Model {
	return Model{
		Tabs:   tabs,
		Cursor: 0,
	}
}

var style = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder())

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
	for i, tab := range t.Tabs {
		tabStyle := inactiveTabStyle
		if i == t.Cursor {
			tabStyle = activeTabStyle
		}

		tabs = lipgloss.JoinHorizontal(lipgloss.Left, tabs, tabStyle.Render(tab.label))
	}

	tabContent := t.Tabs[t.Cursor].Content

	return t.Style.
		Inherit(style).
		Width(t.Width).
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
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		for i := range t.Tabs {
			t.Tabs[i].Content, cmd = t.Tabs[i].Content.Update(msg)
			if cmd != nil {
				return t, cmd
			}
		}
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Select) && !t.selected:
			t.selected = true

			activeMsg := SetActiveTabMsg{Active: true}
			t.Tabs[t.Cursor].Content, cmd = t.Tabs[t.Cursor].Content.Update(activeMsg)

			return t, cmd

		case key.Matches(msg, config.DefaultKeyMap.Back):
			t.selected = false

			activeMsg := SetActiveTabMsg{Active: false}
			t.Tabs[t.Cursor].Content, cmd = t.Tabs[t.Cursor].Content.Update(activeMsg)

			return t, cmd

		case key.Matches(msg, config.DefaultKeyMap.Left) && !t.selected:
			t.Cursor = inmath.Circle(t.Cursor-1, 0, len(t.Tabs)-1)
		case key.Matches(msg, config.DefaultKeyMap.Right) && !t.selected:
			t.Cursor = inmath.Circle(t.Cursor+1, 0, len(t.Tabs)-1)
		}
	}

	t.Tabs[t.Cursor].Content, cmd = t.Tabs[t.Cursor].Content.Update(msg)
	return t, cmd
}
