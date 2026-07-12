package components

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

func (b Button) View() tea.View {
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
	return tea.NewView(Z.Mark(b.zoneID, v))
}

func (b Button) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ButtonClickedMsg:
		b.Clicked = false
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Select) && b.Active:
			b.Clicked = true
			return b, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
				return ButtonClickedMsg{}
			})
		}
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft || !b.Active {
			break
		}
		if b.zoneID == "" {
			b.zoneID = "btn-" + b.Label
		}
		if Z.Get(b.zoneID).InBounds(msg) {
			b.Clicked = true
			return b, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
				return ButtonClickedMsg{}
			})
		}
	}
	return b, nil
}
