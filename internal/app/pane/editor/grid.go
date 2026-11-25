package editor

import (
	"sort"
	"strconv"
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
	zone  string // mouse zone prefix
}

func newGrid() kvGrid {
	return kvGrid{rows: []paramField{createParam()}, zone: components.Z.NewPrefix()}
}

func (g kvGrid) cellZone(cell int) string { return g.zone + strconv.Itoa(cell) }
func (g kvGrid) boxZone(row int) string   { return g.zone + "chk" + strconv.Itoa(row) }

// checkWidth is the checkbox column left of each row.
const checkWidth = 4

var (
	checkOn  = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Green)).Bold(true)
	checkOff = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay0))
)

// checkbox draws a row's on/off box, vertically centered on the field.
func checkbox(on bool) string {
	box, st := "☐", checkOff
	if on {
		box, st = "☑", checkOn
	}
	return lipgloss.NewStyle().Width(checkWidth).PaddingTop(1).PaddingLeft(1).Render(st.Render(box))
}

// toggle switches the row of a cell on or off.
func (g *kvGrid) toggle(cell int) {
	if cell >= 0 && cell < g.cells() {
		g.rows[cell/2].enabled = !g.rows[cell/2].enabled
	}
}

// click returns the cell under a mouse click (placing its cursor at the
// clicked column), or -1. A click on a checkbox switches its row and
// returns the row's name cell.
func (g *kvGrid) click(msg tea.MouseClickMsg) int {
	for row := range g.rows {
		if components.Z.Get(g.boxZone(row)).InBounds(msg) {
			g.rows[row].enabled = !g.rows[row].enabled
			return row * 2
		}
	}
	for cell := range g.cells() {
		z := components.Z.Get(g.cellZone(cell))
		if !z.InBounds(msg) {
			continue
		}
		f := &g.rows[cell/2].name
		if cell%2 == 1 {
			f = &g.rows[cell/2].value
		}
		x, _ := z.Pos(msg)
		f.ClickAt(x)
		return cell
	}
	return -1
}

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

// setMaps replaces the rows with the enabled and the disabled ones, sorted
// by name.
func (g *kvGrid) setMaps(enabled, disabled map[string]string) {
	g.setMap(enabled)
	if len(disabled) == 0 {
		return
	}
	rows := g.rows
	if len(enabled) == 0 {
		rows = nil // drop the blank row
	}
	for k, v := range disabled {
		r := createParamWith(k, v)
		r.enabled = false
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].name.Value() < rows[j].name.Value() })
	g.setRows(rows)
}

// toMap returns the enabled named rows (later rows win for repeated names).
func (g kvGrid) toMap() map[string]string { return paramFieldsToMap(g.rows, true) }

// disabledMap returns the switched-off named rows (nil when none).
func (g kvGrid) disabledMap() map[string]string {
	m := paramFieldsToMap(g.rows, false)
	if len(m) == 0 {
		return nil
	}
	return m
}

func (g kvGrid) cells() int { return len(g.rows) * 2 }

// add appends a blank row and returns its name cell.
func (g *kvGrid) add() int {
	g.rows = append(g.rows, createParam())
	g.setWidth(g.width)
	return len(g.rows)*2 - 2
}

func (g *kvGrid) setWidth(w int) {
	g.width = w
	total := max(0, w-2-checkWidth)
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
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top,
			components.Z.Mark(g.boxZone(i), checkbox(r.enabled)),
			components.Z.Mark(g.cellZone(i*2), r.name.View().Content),
			components.Z.Mark(g.cellZone(i*2+1), r.value.View().Content)))
		if focus == i*2 || focus == i*2+1 {
			f, indent := r.name, checkWidth
			if focus == i*2+1 {
				f, indent = r.value, checkWidth+lipgloss.Width(r.name.View().Content)
			}
			if menu := f.MenuView(max(24, g.width-indent)); menu != "" {
				lines = append(lines, lipgloss.NewStyle().PaddingLeft(indent).Render(menu))
			}
		}
	}
	return strings.Join(lines, "\n")
}
