package components

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
)

type ButtonClickedMsg struct{}

type Button struct {
	Label  string
	Active bool

	Clicked bool

	Style lipgloss.Style
}

func (b Button) Init() tea.Cmd {
	return nil
}

func (b Button) View() string {
	switch {
	case !b.Active:
		return b.Style.
			Background(lipgloss.Color(config.Overlay1)).
			Render(b.Label)
	default:
		return b.Style.Render(b.Label)
	}
}

func (b Button) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ButtonClickedMsg:
		b.Clicked = false
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Select) && b.Active:
			b.Clicked = true
			return b, tea.Tick(300, func(t time.Time) tea.Msg {
				return ButtonClickedMsg{}
			})
		}
	}
	return b, nil
}
