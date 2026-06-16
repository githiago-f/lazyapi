package editor

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	case tea.KeyMsg:
		if !p.active {
			return p, nil
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

	return p, nil
}
