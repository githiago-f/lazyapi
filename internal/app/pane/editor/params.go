package editor

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
)

type params struct {
	active bool
	width  int

	focusPos int // linear index across all sub-fields; -1 = none
	params   []paramField
	query    []paramField
}

func (p *params) SetActive(b bool) {
	p.active = b
	if b {
		p.focusPos = 0
	} else {
		p.focusPos = -1
	}
}

func (p *params) IsActive() bool {
	return p.active
}

func (p *params) totalFields() int {
	return len(p.query)*2 + len(p.params)*2
}

func ParamsTab() *params {
	return &params{
		focusPos: -1,
		query:    []paramField{createParam()},
		params:   []paramField{createParam()},
	}
}

var titleStyle = lipgloss.NewStyle().
	Bold(true).
	Align(lipgloss.Center).
	BorderStyle(lipgloss.NormalBorder()).
	BorderBottom(true)

func (p *params) SetValue(query map[string]string, pathParams map[string]string) {
	p.query = mapToParamFields(query)
	p.params = mapToParamFields(pathParams)
	if len(p.query) == 0 {
		p.query = []paramField{createParam()}
	}
	if len(p.params) == 0 {
		p.params = []paramField{createParam()}
	}
	p.updateFieldWidths()
}

func (p *params) QueryValue() map[string]string {
	return paramFieldsToMap(p.query)
}

func (p *params) ParamsValue() map[string]string {
	return paramFieldsToMap(p.params)
}

func (p *params) Reset() {
	p.query = []paramField{createParam()}
	p.params = []paramField{createParam()}
}

func (p params) View() tea.View {
	customParams := titleStyle.Width(p.width).Render("Query")

	activeColor := config.DefaultConfig.PrimaryColor()

	for i := range p.query {
		p.query[i].name.Style = p.query[i].name.Style.UnsetBorderForeground()
		p.query[i].value.Style = p.query[i].value.Style.UnsetBorderForeground()
	}
	for i := range p.params {
		p.params[i].name.Style = p.params[i].name.Style.UnsetBorderForeground()
		p.params[i].value.Style = p.params[i].value.Style.UnsetBorderForeground()
	}

	if p.active && p.focusPos >= 0 && p.focusPos < len(p.query)*2 {
		row := p.focusPos / 2
		col := p.focusPos % 2
		if row < len(p.query) {
			if col == 0 {
				p.query[row].name.Style = p.query[row].name.Style.BorderForeground(activeColor)
			} else {
				p.query[row].value.Style = p.query[row].value.Style.BorderForeground(activeColor)
			}
		}
	}
	if p.active && p.focusPos >= len(p.query)*2 {
		offset := p.focusPos - len(p.query)*2
		row := offset / 2
		col := offset % 2
		if row < len(p.params) {
			if col == 0 {
				p.params[row].name.Style = p.params[row].name.Style.BorderForeground(activeColor)
			} else {
				p.params[row].value.Style = p.params[row].value.Style.BorderForeground(activeColor)
			}
		}
	}

	for i := range p.query {
		customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, p.query[i].View().Content)
	}

	customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, titleStyle.Render("Path params"))

	for i := range p.params {
		customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, p.params[i].View().Content)
	}

	return tea.NewView(customParams)
}

func (p *params) updateFieldWidths() {
	totalWidth := p.width - 2
	nameWidth := totalWidth / 2
	valueWidth := totalWidth - nameWidth
	for i := range p.query {
		p.query[i].name.Style = lipgloss.NewStyle().Width(nameWidth)
		p.query[i].name.TextInput.SetWidth(max(0, nameWidth-2))
		p.query[i].value.Style = lipgloss.NewStyle().Width(valueWidth)
		p.query[i].value.TextInput.SetWidth(max(0, valueWidth-2))
	}
	for i := range p.params {
		p.params[i].name.Style = lipgloss.NewStyle().Width(nameWidth)
		p.params[i].name.TextInput.SetWidth(max(0, nameWidth-2))
		p.params[i].value.Style = lipgloss.NewStyle().Width(valueWidth)
		p.params[i].value.TextInput.SetWidth(max(0, valueWidth-2))
	}
}

func (p params) Init() tea.Cmd {
	return nil
}

func (p params) HelpBindings() []key.Binding {
	return []key.Binding{
		config.DefaultKeyMap.Next,
		config.DefaultKeyMap.Prev,
		config.DefaultKeyMap.AddQueryParam,
		config.DefaultKeyMap.AddPathParam,
		config.DefaultKeyMap.Back,
	}
}

func (p params) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.updateFieldWidths()

	case tabs.SetActiveTabMsg:
		p.SetActive(msg.Active)
		return &p, nil

	case tea.KeyPressMsg:
		if !p.active {
			return &p, nil
		}

		// Shortcuts intercepted before field processing
		if key.Matches(msg, config.DefaultKeyMap.AddQueryParam) {
			p.query = append(p.query, createParam())
			p.updateFieldWidths()
			return &p, nil
		}
		if key.Matches(msg, config.DefaultKeyMap.AddPathParam) {
			p.params = append(p.params, createParam())
			p.updateFieldWidths()
			return &p, nil
		}

		total := p.totalFields()

		switch {
		case key.Matches(msg, config.DefaultKeyMap.Next):
			if total > 0 {
				p.focusPos = (p.focusPos + 1) % total
			}

		case key.Matches(msg, config.DefaultKeyMap.Prev):
			if total > 0 {
				p.focusPos = (p.focusPos - 1 + total) % total
			}

		default:
			if total == 0 {
				return &p, nil
			}

			qc := len(p.query) * 2
			if p.focusPos < qc {
				row := p.focusPos / 2
				col := p.focusPos % 2
				if col == 0 {
					m, cmd := p.query[row].name.Update(msg)
					p.query[row].name = m.(components.Field)
					return &p, cmd
				} else {
					m, cmd := p.query[row].value.Update(msg)
					p.query[row].value = m.(components.Field)
					return &p, cmd
				}
			} else {
				offset := p.focusPos - qc
				row := offset / 2
				col := offset % 2
				if col == 0 {
					m, cmd := p.params[row].name.Update(msg)
					p.params[row].name = m.(components.Field)
					return &p, cmd
				} else {
					m, cmd := p.params[row].value.Update(msg)
					p.params[row].value = m.(components.Field)
					return &p, cmd
				}
			}
		}
	}

	return &p, nil
}
