package requesteditor

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
)

type header struct {
	active    bool
	width     int
	cmdBuffer rune
	headers   []paramField
}

// SetActive implements [tabs.StatefulInputBase].
func (h *header) SetActive(b bool) {
	h.active = b
}

func HeaderTab() *header {
	return &header{
		headers: []paramField{createParam()},
	}
}

func (h header) Init() tea.Cmd {
	return nil
}

func (h header) View() string {
	customParams := titleStyle.Render("Headers")

	for _, v := range h.headers {
		v.SetWidth(h.width / 2)
		customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, v.View())
	}

	return customParams
}

func (h header) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = msg.Width
	case tea.KeyMsg:
		if !h.active {
			return h, nil
		}

		isNewCmd := h.cmdBuffer == 'n'

		switch {
		case key.Matches(msg, config.DefaultKeyMap.New):
			h.cmdBuffer = 'n'

		case msg.String() == "h" && isNewCmd:
			h.headers = append(h.headers, createParam())
			h.cmdBuffer = '0'

		default:
			h.cmdBuffer = '0'
		}
	}
	return h, nil
}
