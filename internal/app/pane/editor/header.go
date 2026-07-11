package editor

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
)

type header struct {
	active    bool
	width     int
	cmdBuffer rune

	focusPos int
	headers  []paramField
}

func (h *header) SetActive(b bool) {
	h.active = b
	if b {
		h.focusPos = 0
	} else {
		h.focusPos = -1
	}
}

func (h *header) IsActive() bool {
	return h.active
}

func HeaderTab() *header {
	return &header{
		focusPos: -1,
		headers:  []paramField{createParam()},
	}
}

func (h header) Init() tea.Cmd {
	return nil
}

func (h header) View() string {
	customParams := titleStyle.Render("Headers")
	activeColor := config.DefaultConfig.PrimaryColor()

	for i := range h.headers {
		if h.active && h.focusPos >= 0 {
			row := h.focusPos / 2
			col := h.focusPos % 2
			if row == i {
				if col == 0 {
					h.headers[i].name.Style = h.headers[i].name.Style.BorderForeground(activeColor)
				} else {
					h.headers[i].value.Style = h.headers[i].value.Style.BorderForeground(activeColor)
				}
			}
		}
		customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, h.headers[i].View())
	}

	return customParams
}

func (h *header) updateFieldWidths() {
	for i := range h.headers {
		h.headers[i].name.Style = lipgloss.NewStyle().Width(h.width/2 - 2)
		h.headers[i].value.Style = lipgloss.NewStyle().Width(h.width / 2)
	}
}

func (h *header) SetValue(headers map[string]string) {
	h.headers = make([]paramField, 0, len(headers))
	for name, value := range headers {
		pf := createParamWith(name, value)
		h.headers = append(h.headers, pf)
	}
	if len(h.headers) == 0 {
		h.headers = []paramField{createParam()}
	}
}

func (h *header) Value() map[string]string {
	result := make(map[string]string, len(h.headers))
	for _, pf := range h.headers {
		name := pf.name.Value()
		value := pf.value.Value()
		if name != "" {
			result[name] = value
		}
	}
	return result
}

func (h header) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = msg.Width
		h.updateFieldWidths()

	case tabs.SetActiveTabMsg:
		h.SetActive(msg.Active)
		return &h, nil

	case tea.KeyMsg:
		if h.focusPos < 0 {
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

		if !h.active {
			return &h, nil
		}

		total := len(h.headers) * 2

		switch {
		case key.Matches(msg, config.DefaultKeyMap.Next):
			if total > 0 {
				h.focusPos = (h.focusPos + 1) % total
			}

		case key.Matches(msg, config.DefaultKeyMap.Prev):
			if total > 0 {
				h.focusPos = (h.focusPos - 1 + total) % total
			}

		default:
			if h.focusPos < 0 || total == 0 {
				return &h, nil
			}

			row := h.focusPos / 2
			col := h.focusPos % 2
			if col == 0 {
				m, cmd := h.headers[row].name.Update(msg)
				h.headers[row].name = m.(components.Field)
				return &h, cmd
			} else {
				m, cmd := h.headers[row].value.Update(msg)
				h.headers[row].value = m.(components.Field)
				return &h, cmd
			}
		}
	}

	return &h, nil
}
