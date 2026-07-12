package editor

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
)

type header struct {
	active   bool
	width    int

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

func (h header) View() tea.View {
	customParams := titleStyle.Width(h.width).Render("Headers")
	activeColor := config.DefaultConfig.PrimaryColor()

	for i := range h.headers {
		h.headers[i].name.Style = h.headers[i].name.Style.UnsetBorderForeground()
		h.headers[i].value.Style = h.headers[i].value.Style.UnsetBorderForeground()
	}

	if h.active && h.focusPos >= 0 {
		row := h.focusPos / 2
		col := h.focusPos % 2
		if row < len(h.headers) {
			if col == 0 {
				h.headers[row].name.Style = h.headers[row].name.Style.BorderForeground(activeColor)
			} else {
				h.headers[row].value.Style = h.headers[row].value.Style.BorderForeground(activeColor)
			}
		}
	}

	for i := range h.headers {
		customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, h.headers[i].View().Content)
	}

	return tea.NewView(customParams)
}

func (h *header) updateFieldWidths() {
	totalWidth := h.width - 2
	nameWidth := totalWidth / 2
	valueWidth := totalWidth - nameWidth
	for i := range h.headers {
		h.headers[i].name.Style = lipgloss.NewStyle().Width(nameWidth)
		h.headers[i].name.TextInput.SetWidth(max(0, nameWidth-2))
		h.headers[i].value.Style = lipgloss.NewStyle().Width(valueWidth)
		h.headers[i].value.TextInput.SetWidth(max(0, valueWidth-2))
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
	h.updateFieldWidths()
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
		h.updateFieldWidths()

	case tabs.SetActiveTabMsg:
		h.SetActive(msg.Active)
		return &h, nil

	case tea.KeyPressMsg:
		if !h.active {
			return &h, nil
		}

		// Shortcuts intercepted before field processing
		if key.Matches(msg, config.DefaultKeyMap.AddHeader) {
			h.headers = append(h.headers, createParam())
			h.updateFieldWidths()
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
			if total == 0 {
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
