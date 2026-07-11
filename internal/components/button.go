package components

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/githiago-f/lazyapi/internal/config"
)

type ButtonClickedMsg struct{}

type Button struct {
	Label  string
	Active bool

	Clicked bool

	Style   lipgloss.Style
	zoneID  string
}

func (b Button) Init() tea.Cmd {
	return nil
}

func (b Button) View() string {
	var v string
	switch {
	case !b.Active:
		v = b.Style.
			Background(lipgloss.Color(config.Overlay1)).
			Render(b.Label)
	default:
		v = b.Style.Render(b.Label)
	}
	if b.zoneID == "" {
		b.zoneID = "btn-" + b.Label
	}
	return zone.Mark(b.zoneID, v)
}

func (b Button) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ButtonClickedMsg:
		b.Clicked = false
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Select) && b.Active:
			b.Clicked = true
			return b, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
				return ButtonClickedMsg{}
			})
		}
	case tea.MouseMsg:
		if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft || !b.Active {
			break
		}
		if b.zoneID == "" {
			b.zoneID = "btn-" + b.Label
		}
		if zone.Get(b.zoneID).InBounds(msg) {
			b.Clicked = true
			return b, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
				return ButtonClickedMsg{}
			})
		}
	}
	return b, nil
}
