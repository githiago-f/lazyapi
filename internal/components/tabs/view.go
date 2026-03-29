package tabs

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/inmath"
)

type Model struct {
	Tabs   []Tab
	Cursor int
	Width  int

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
		tab.Style = inactiveTabStyle
		if i == t.Cursor {
			tab.Style = activeTabStyle
		}

		tabs = lipgloss.JoinHorizontal(lipgloss.Left, tabs, tab.View())
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
	var (
		model tea.Model
		cmd   tea.Cmd
	)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		for i := range t.Tabs {
			model, cmd = t.Tabs[i].Update(msg)
			if cmd != nil {
				return t, cmd
			}
			t.Tabs[i] = model.(Tab)
		}
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Select):
			t.Tabs[t.Cursor].Active = true
			t.Tabs[t.Cursor].Content = t.Tabs[t.Cursor].Content.SetActive(true)
		case key.Matches(msg, config.DefaultKeyMap.Back):
			t.Tabs[t.Cursor].Active = false
			t.Tabs[t.Cursor].Content = t.Tabs[t.Cursor].Content.SetActive(false)
		case key.Matches(msg, config.DefaultKeyMap.Left) && !t.Tabs[t.Cursor].Active:
			t.Cursor = inmath.Cicle(t.Cursor-1, 0, len(t.Tabs)-1)
		case key.Matches(msg, config.DefaultKeyMap.Right) && !t.Tabs[t.Cursor].Active:
			t.Cursor = inmath.Cicle(t.Cursor+1, 0, len(t.Tabs)-1)
		}
	}

	model, cmd = t.Tabs[t.Cursor].Update(msg)
	t.Tabs[t.Cursor], _ = model.(Tab)
	return t, cmd
}
