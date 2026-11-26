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
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Top, section(config.T("editor.headers.title"), config.T("editor.headers.hint", "add", config.DefaultKeyMap.AddHeader.Help().Key)+" · "+gridHint(), h.width), h.grid.view(focus)))
}

func (h *header) SetValue(headers, disabled map[string]string) { h.grid.setMaps(headers, disabled) }

func (h *header) Value() map[string]string { return h.grid.toMap() }

// Disabled are the switched-off headers.
func (h *header) Disabled() map[string]string { return h.grid.disabledMap() }

func (h header) HelpBindings() []key.Binding {
	return []key.Binding{
		config.DefaultKeyMap.Next,
		config.DefaultKeyMap.Prev,
		config.DefaultKeyMap.AddHeader,
		config.DefaultKeyMap.Toggle,
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

	case tea.MouseClickMsg:
		if cell := h.grid.click(msg); cell >= 0 && h.active {
			h.focusPos = cell
		}

	case tea.PasteMsg:
		if h.active {
			return &h, h.grid.update(h.focusPos, msg)
		}

	case tea.KeyPressMsg:
		if !h.active {
			return &h, nil
		}
		if h.grid.completing(h.focusPos) && menuKey(msg) {
			return &h, h.grid.update(h.focusPos, msg)
		}
		total := h.grid.cells()
		switch {
		case key.Matches(msg, config.DefaultKeyMap.AddHeader):
			h.focusPos = h.grid.add()
		case key.Matches(msg, config.DefaultKeyMap.Toggle):
			h.grid.toggle(h.focusPos)
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

// Completing reports whether the focused field has its menu open.
func (h header) Completing() bool { return h.active && h.grid.completing(h.focusPos) }

// HasSelection reports whether ctrl+c copies the focused field.
func (h header) HasSelection() bool { return h.active && h.grid.canCopy(h.focusPos) }
