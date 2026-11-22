package editor

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
)

type header struct {
	active   bool
	width    int
	focusPos int // cell of grid; -1 = none
	grid     kvGrid
}

func (h *header) SetActive(b bool) {
	h.active = b
	h.focusPos = -1
	if b {
		h.focusPos = 0
	}
}

func (h *header) IsActive() bool { return h.active }

func HeaderTab() *header {
	return &header{focusPos: -1, grid: newGrid()}
}

func (h header) Init() tea.Cmd { return nil }

func (h header) View() tea.View {
	focus := -1
	if h.active {
		focus = h.focusPos
	}
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Top, titleStyle.Width(h.width).Render("Headers"), h.grid.view(focus)))
}

func (h *header) SetValue(headers map[string]string) { h.grid.setMap(headers) }

func (h *header) Value() map[string]string { return h.grid.toMap() }

func (h header) HelpBindings() []key.Binding {
	return []key.Binding{
		config.DefaultKeyMap.Next,
		config.DefaultKeyMap.Prev,
		config.DefaultKeyMap.AddHeader,
		config.DefaultKeyMap.Back,
	}
}

func (h header) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = msg.Width
		h.grid.setWidth(msg.Width)

	case tabs.SetActiveTabMsg:
		h.SetActive(msg.Active)

	case tea.KeyPressMsg:
		if !h.active {
			return &h, nil
		}
		total := h.grid.cells()
		switch {
		case key.Matches(msg, config.DefaultKeyMap.AddHeader):
			h.focusPos = h.grid.add()
		case key.Matches(msg, config.DefaultKeyMap.Next):
			h.focusPos = (h.focusPos + 1) % total
		case key.Matches(msg, config.DefaultKeyMap.Prev):
			h.focusPos = (h.focusPos - 1 + total) % total
		default:
			return &h, h.grid.update(h.focusPos, msg)
		}
	}
	return &h, nil
}
