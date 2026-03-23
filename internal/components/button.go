package components

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
)

type buttonColorMsg int

type Button struct {
	Label   string
	Active  bool
	Hovered bool

	clicked bool

	Style        lipgloss.Style
	HoverStyle   lipgloss.Style
	ClickedStyle lipgloss.Style
}

func (b Button) Init() tea.Cmd {
	return nil
}

func (b Button) View() string {
	switch {
	case b.Hovered:
		return b.HoverStyle.Render(b.Label)
	case !b.Active:
		return b.Style.
			Background(lipgloss.Color(config.Overlay1)).
			Render(b.Label)
	case b.clicked:
		return b.ClickedStyle.Render(b.Label)
	default:
		return b.Style.Render(b.Label)
	}
}

func (b Button) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case buttonColorMsg:
		b.clicked = false
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Select) && b.Active:
			b.clicked = true
			return b, tea.Tick(300, func(t time.Time) tea.Msg {
				return buttonColorMsg(0)
			})
		}
	}
	return b, nil
}
