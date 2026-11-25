package components

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/config"
)

type ButtonClickedMsg struct{}

// Button is a bordered action button. It is drawn like the fields around
// it while idle, filled with the accent while focused, flashes green when
// pressed and dims while Busy.
type Button struct {
	Label  string
	Active bool // false: disabled
	// Focused draws the focus highlight (the parent sets it).
	Focused bool
	// Busy replaces the label (e.g. "Sending") and dims the button.
	Busy      bool
	BusyLabel string

	Clicked bool

	// Style adds to the base style (e.g. a width).
	Style lipgloss.Style
	// ID names the mouse zone (default: the label).
	ID     string
	zoneID string
}

func (b Button) zone() string {
	if b.zoneID != "" {
		return b.zoneID
	}
	if b.ID != "" {
		return "btn-" + b.ID
	}
	return "btn-" + b.Label
}

var buttonBase = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color(config.Border)).
	Padding(0, 2).
	Bold(true).
	Foreground(lipgloss.Color(config.Go))

func (b Button) Init() tea.Cmd {
	return nil
}

// style is the button's look for its current state. States are applied on
// top of the base (lipgloss Inherit drops padding).
func (b Button) style() lipgloss.Style {
	st := buttonBase
	if w := b.Style.GetWidth(); w > 0 {
		st = st.Width(w)
	}
	switch {
	case b.Clicked:
		return st.BorderForeground(lipgloss.Color(config.Go)).
			Background(lipgloss.Color(config.Go)).Foreground(lipgloss.Color(config.OnAccent))
	case !b.Active || b.Busy:
		st = st.Foreground(lipgloss.Color(config.Overlay0))
		if b.Focused {
			st = st.BorderForeground(lipgloss.Color(config.Accent))
		}
		return st
	case b.Focused:
		return st.BorderForeground(lipgloss.Color(config.Accent)).
			Background(lipgloss.Color(config.Accent)).Foreground(lipgloss.Color(config.OnAccent))
	}
	return st
}

// Zone is where the button was drawn.
func (b Button) Zone() Zone { return Z.Get(b.zone()) }

func (b Button) View() tea.View {
	label := b.Label
	if b.Busy && b.BusyLabel != "" {
		label = b.BusyLabel
	}
	return tea.NewView(Z.Mark(b.zone(), b.style().Render(label)))
}

func (b Button) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ButtonClickedMsg:
		b.Clicked = false
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Select) && b.Active && !b.Busy:
			b.Clicked = true
			return b, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
				return ButtonClickedMsg{}
			})
		}
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft || !b.Active || b.Busy {
			break
		}
		if b.Zone().InBounds(msg) {
			b.Clicked = true
			return b, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
				return ButtonClickedMsg{}
			})
		}
	}
	return b, nil
}
