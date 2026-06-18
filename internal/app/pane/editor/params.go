package editor

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
)

type params struct {
	active bool
	width  int
	prevKey rune

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

func (p params) View() string {
	width := (p.width / 2)
	titleStyle = titleStyle.Width(p.width)
	customParams := titleStyle.Render("Query")

	activeColor := config.DefaultConfig.PrimaryColor()

	for i := range p.query {
		p.query[i].name.Style = lipgloss.NewStyle().Width(width - 2)
		p.query[i].value.Style = lipgloss.NewStyle().Width(width)

		if p.active && p.focusPos >= 0 && p.focusPos < len(p.query)*2 {
			row := p.focusPos / 2
			col := p.focusPos % 2
			if row == i {
				if col == 0 {
					p.query[i].name.Style = p.query[i].name.Style.BorderForeground(activeColor)
				} else {
					p.query[i].value.Style = p.query[i].value.Style.BorderForeground(activeColor)
				}
			}
		}
		customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, p.query[i].View())
	}

	customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, titleStyle.Render("Path params"))

	for i := range p.params {
		p.params[i].name.Style = lipgloss.NewStyle().Width(width - 2)
		p.params[i].value.Style = lipgloss.NewStyle().Width(width)

		if p.active && p.focusPos >= len(p.query)*2 {
			offset := p.focusPos - len(p.query)*2
			row := offset / 2
			col := offset % 2
			if row == i {
				if col == 0 {
					p.params[i].name.Style = p.params[i].name.Style.BorderForeground(activeColor)
				} else {
					p.params[i].value.Style = p.params[i].value.Style.BorderForeground(activeColor)
				}
			}
		}
		customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, p.params[i].View())
	}

	return customParams
}

func (p params) Init() tea.Cmd {
	return nil
}

func (p params) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width

	case tabs.SetActiveTabMsg:
		p.SetActive(msg.Active)
		return &p, nil

	case tea.KeyMsg:
		if p.focusPos < 0 {
			isNewCmd := p.prevKey == 'n'
			switch {
			case key.Matches(msg, config.DefaultKeyMap.New):
				p.prevKey = 'n'
			case msg.String() == "q" && isNewCmd:
				p.query = append(p.query, createParam())
				p.prevKey = '0'
			case msg.String() == "p" && isNewCmd:
				p.params = append(p.params, createParam())
				p.prevKey = '0'
			default:
				p.prevKey = '0'
			}
		}

		if !p.active {
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
			if p.focusPos < 0 || total == 0 {
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
