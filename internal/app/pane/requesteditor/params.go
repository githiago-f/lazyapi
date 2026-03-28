package requesteditor

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
)

type paramField struct {
	enabled bool
	name    components.Field
	value   components.Field
}

type params struct {
	active        bool
	selectedField int

	width int

	cmdBuffer string

	params []paramField
	query  []paramField
}

// SetActive implements [tabs.StatefulInputBase].
func (p params) SetActive(b bool) tabs.StatefulInputBase {
	p.active = b
	return p
}

func ParamsTab() params {
	return params{
		query: []paramField{{
			enabled: true,
			name:    components.InitField("Name", ""),
			value:   components.InitField("Value", ""),
		}},
		params: []paramField{{
			enabled: true,
			name:    components.InitField("Name", ""),
			value:   components.InitField("Value", ""),
		}},
	}
}

var titleStyle = lipgloss.NewStyle().
	Bold(true).
	Align(lipgloss.Center)

func (p params) View() string {
	customParams := titleStyle.Render("Query")

	for _, param := range p.query {
		customParam := lipgloss.JoinHorizontal(lipgloss.Top, param.name.View(), param.value.View())
		customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, customParam)
	}

	customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, titleStyle.Render("Path params"))

	for _, param := range p.params {
		customParam := lipgloss.JoinHorizontal(lipgloss.Top, param.name.View(), param.value.View())
		customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, customParam)
	}

	return customParams
}

func (p params) Init() tea.Cmd {
	return nil
}

func (p params) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	width := (p.width / 2) - 4
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width

		for i := range p.params {
			p.params[i].name.Style = p.params[i].name.Style.Width(width)
			p.params[i].value.Style = p.params[i].value.Style.Width(width)
		}
		for i := range p.query {
			p.query[i].name.Style = p.query[i].name.Style.Width(width)
			p.query[i].value.Style = p.query[i].value.Style.Width(width)
		}

		return p, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.New):
			p.cmdBuffer = "n"
		case msg.String() == "q" && p.cmdBuffer[0] == 'n':
			query := paramField{
				enabled: true,
				name:    components.InitField("Name", ""),
				value:   components.InitField("Value", ""),
			}
			query.name.Style = query.name.Style.Width(width)
			query.value.Style = query.value.Style.Width(width)
			p.query = append(p.query, query)

		case msg.String() == "p" && p.cmdBuffer[0] == 'n':
			param := paramField{
				enabled: true,
				name:    components.InitField("Name", ""),
				value:   components.InitField("Value", ""),
			}
			param.name.Style = param.name.Style.Width(width)
			param.value.Style = param.value.Style.Width(width)
			p.params = append(p.params, param)

		default:
			p.cmdBuffer = ""
		}
	}

	return p, nil
}
