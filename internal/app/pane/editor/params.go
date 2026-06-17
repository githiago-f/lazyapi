package editor

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
)

type params struct {
	active        bool
	selectedField int

	width int

	prevKey rune

	params []paramField
	query  []paramField
}

func (p *params) SetActive(b bool) {
	p.active = b
}

func (p *params) IsActive() bool {
	return p.active
}

func ParamsTab() *params {
	return &params{
		query:  []paramField{createParam()},
		params: []paramField{createParam()},
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

	for _, param := range p.query {
		param.SetWidth(width)
		customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, param.View())
	}

	customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, titleStyle.Render("Path params"))

	for _, param := range p.params {
		param.SetWidth(width)
		customParams = lipgloss.JoinVertical(lipgloss.Top, customParams, param.View())
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
		p.active = msg.Active
		return &p, nil
	case tea.KeyMsg:
		if !p.active {
			return &p, nil
		}

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

	return &p, nil
}
