package editor

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
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

// section draws a list title with a hint line under it.
func section(title, hint string, width int) string {
	t := titleStyle
	if width > 0 {
		t = t.Width(width)
	}
	if width > 0 {
		hint = ansi.Truncate(hint, width, "…")
	}
	return lipgloss.JoinVertical(lipgloss.Left, t.Render(title), hintStyle.Render(hint))
}

// gridHint is the hint shared by the name/value lists.
func gridHint() string {
	k := config.DefaultKeyMap
	return config.T("editor.grid.hint", "toggle", k.Toggle.Help().Key)
}

var hintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1))

var titleStyle = lipgloss.NewStyle().
	Bold(true).
	Align(lipgloss.Center).
	BorderStyle(lipgloss.NormalBorder()).
	BorderBottom(true)

func (p *params) SetValue(query, pathParams map[string]string, disabled model.Disabled) {
	p.query.setMaps(query, disabled.Query)
	p.params.setMaps(pathParams, disabled.Params)
}

// DisabledQuery and DisabledParams are the switched-off rows.
func (p *params) DisabledQuery() map[string]string  { return p.query.disabledMap() }
func (p *params) DisabledParams() map[string]string { return p.params.disabledMap() }

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
		section(config.T("editor.params.query"), config.T("editor.params.query_hint", "add", config.DefaultKeyMap.AddQueryParam.Help().Key)+" · "+gridHint(), p.width),
		p.query.view(qFocus),
		section(config.T("editor.params.path"), config.T("editor.params.path_hint", "add", config.DefaultKeyMap.AddPathParam.Help().Key), p.width),
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
		config.DefaultKeyMap.Toggle,
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

	case tea.MouseClickMsg:
		if !p.active {
			break
		}
		if cell := p.query.click(msg); cell >= 0 {
			p.focusPos = cell
		} else if cell := p.params.click(msg); cell >= 0 {
			p.focusPos = p.query.cells() + cell
		}

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
		case key.Matches(msg, config.DefaultKeyMap.Toggle):
			if q := p.query.cells(); p.focusPos < q {
				p.query.toggle(p.focusPos)
			} else {
				p.params.toggle(p.focusPos - q)
			}
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
