package editor

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
)

// params is the Params tab: query parameters, then path parameters. The
// focus is one index across both grids.
type params struct {
	active bool
	width  int

	focusPos int // cell across query then path; -1 = none
	query    kvGrid
	params   kvGrid
}

func (p *params) SetActive(b bool) {
	p.active = b
	p.focusPos = -1
	if b {
		p.focusPos = 0
	}
}

func (p *params) IsActive() bool { return p.active }

func (p *params) totalFields() int { return p.query.cells() + p.params.cells() }

func ParamsTab() *params {
	return &params{focusPos: -1, query: newGrid(), params: newGrid()}
}

var titleStyle = lipgloss.NewStyle().
	Bold(true).
	Align(lipgloss.Center).
	BorderStyle(lipgloss.NormalBorder()).
	BorderBottom(true)

func (p *params) SetValue(query map[string]string, pathParams map[string]string) {
	p.query.setMap(query)
	p.params.setMap(pathParams)
}

func (p *params) QueryValue() map[string]string { return p.query.toMap() }

func (p *params) ParamsValue() map[string]string { return p.params.toMap() }

func (p *params) Reset() {
	p.query.setRows(nil)
	p.params.setRows(nil)
}

func (p params) View() tea.View {
	qFocus, pFocus := -1, -1
	if p.active {
		if p.focusPos < p.query.cells() {
			qFocus = p.focusPos
		} else {
			pFocus = p.focusPos - p.query.cells()
		}
	}
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Top,
		titleStyle.Width(p.width).Render("Query"),
		p.query.view(qFocus),
		titleStyle.Render("Path params"),
		p.params.view(pFocus),
	))
}

func (p params) Init() tea.Cmd { return nil }

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
		p.query.setWidth(msg.Width)
		p.params.setWidth(msg.Width)

	case tabs.SetActiveTabMsg:
		p.SetActive(msg.Active)

	case tea.KeyPressMsg:
		if !p.active {
			return &p, nil
		}
		if q := p.query.cells(); p.focusPos < q && p.query.completing(p.focusPos) && menuKey(msg) {
			return &p, p.query.update(p.focusPos, msg)
		} else if p.focusPos >= q && p.params.completing(p.focusPos-q) && menuKey(msg) {
			return &p, p.params.update(p.focusPos-q, msg)
		}
		total := p.totalFields()
		switch {
		case key.Matches(msg, config.DefaultKeyMap.AddQueryParam):
			p.focusPos = p.query.add()
		case key.Matches(msg, config.DefaultKeyMap.AddPathParam):
			p.focusPos = p.query.cells() + p.params.add()
		case key.Matches(msg, config.DefaultKeyMap.Next):
			p.focusPos = (p.focusPos + 1) % total
		case key.Matches(msg, config.DefaultKeyMap.Prev):
			p.focusPos = (p.focusPos - 1 + total) % total
		default:
			if p.focusPos < p.query.cells() {
				return &p, p.query.update(p.focusPos, msg)
			}
			return &p, p.params.update(p.focusPos-p.query.cells(), msg)
		}
	}
	return &p, nil
}

// Completing reports whether the focused field has its menu open.
func (p params) Completing() bool {
	if !p.active {
		return false
	}
	if q := p.query.cells(); p.focusPos >= q {
		return p.params.completing(p.focusPos - q)
	}
	return p.query.completing(p.focusPos)
}
