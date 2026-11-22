package editor

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
)

// kvGrid is a list of name/value rows (headers, query and path params, form
// fields). Cells are numbered row*2 (name) and row*2+1 (value). View never
// changes the grid: focus styling is applied to copies.
type kvGrid struct {
	rows  []paramField
	width int
}

func newGrid() kvGrid { return kvGrid{rows: []paramField{createParam()}} }

// setMap replaces the rows with m, sorted by name (one blank row if empty).
func (g *kvGrid) setMap(m map[string]string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([]paramField, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, createParamWith(k, m[k]))
	}
	g.setRows(rows)
}

// setRows replaces the rows as given (one blank row if empty).
func (g *kvGrid) setRows(rows []paramField) {
	if len(rows) == 0 {
		rows = []paramField{createParam()}
	}
	g.rows = rows
	g.setWidth(g.width)
}

// toMap returns the named rows (later rows win for repeated names).
func (g kvGrid) toMap() map[string]string { return paramFieldsToMap(g.rows) }

func (g kvGrid) cells() int { return len(g.rows) * 2 }

// add appends a blank row and returns its name cell.
func (g *kvGrid) add() int {
	g.rows = append(g.rows, createParam())
	g.setWidth(g.width)
	return len(g.rows)*2 - 2
}

func (g *kvGrid) setWidth(w int) {
	g.width = w
	total := max(0, w-2)
	nameW := total / 2
	valueW := total - nameW
	for i := range g.rows {
		g.rows[i].name.Style = lipgloss.NewStyle().Width(nameW)
		g.rows[i].name.TextInput.SetWidth(max(0, nameW-2))
		g.rows[i].value.Style = lipgloss.NewStyle().Width(valueW)
		g.rows[i].value.TextInput.SetWidth(max(0, valueW-2))
	}
}

// completing reports whether the cell's field has its completion menu
// open (then tab/esc/arrows go to the field, not to navigation).
func (g kvGrid) completing(cell int) bool {
	if cell < 0 || cell >= g.cells() {
		return false
	}
	if cell%2 == 0 {
		return g.rows[cell/2].name.Completing()
	}
	return g.rows[cell/2].value.Completing()
}

// update sends msg to a cell.
func (g *kvGrid) update(cell int, msg tea.Msg) tea.Cmd {
	if cell < 0 || cell >= g.cells() {
		return nil
	}
	row := cell / 2
	f := &g.rows[row].name
	if cell%2 == 1 {
		f = &g.rows[row].value
	}
	m, cmd := f.Update(msg)
	*f = m.(components.Field)
	return cmd
}

// view renders the rows; focus is the highlighted cell (-1 for none).
func (g kvGrid) view(focus int) string {
	active := config.DefaultConfig.PrimaryColor()
	lines := make([]string, 0, len(g.rows))
	for i, r := range g.rows { // r is a copy
		if focus == i*2 {
			r.name.Style = r.name.Style.BorderForeground(active)
		}
		if focus == i*2+1 {
			r.value.Style = r.value.Style.BorderForeground(active)
		}
		lines = append(lines, r.View().Content)
		if focus == i*2 || focus == i*2+1 {
			f, indent := r.name, 0
			if focus == i*2+1 {
				f, indent = r.value, lipgloss.Width(r.name.View().Content)
			}
			if menu := f.MenuView(max(24, g.width-indent)); menu != "" {
				lines = append(lines, lipgloss.NewStyle().PaddingLeft(indent).Render(menu))
			}
		}
	}
	return strings.Join(lines, "\n")
}
