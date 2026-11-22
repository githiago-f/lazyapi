// Package runview is the TUI test runner: it runs a spec, a tag or flows
// through internal/runner, streams results as they finish and lets the user
// browse them (failures first) with the response view on the right.
package runview

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/app/pane/responses"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/runner"
)

// Messages emitted to the TUI.
type (
	// CloseMsg leaves the runner view.
	CloseMsg struct{}
	// OpenOperationMsg opens an operation in the editor.
	OpenOperationMsg struct{ Ref model.OpenAPIRef }
)

// Internal messages; id ties them to one run so a rerun ignores stale ones.
type (
	resultMsg struct {
		id  int
		res runner.OpResult
	}
	doneMsg struct {
		id     int
		report runner.Report
		err    error
	}
)

// Keys of the runner view.
var (
	UpKey       = key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up"))
	DownKey     = key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down"))
	FailedKey   = key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "only failures"))
	RerunKey    = key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "run again"))
	OpenKey     = key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "open in editor"))
	DetailKey   = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "list/details"))
	BackKey     = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "stop/back"))
	ResponseKey = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details · expand flow"))
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(config.Brand))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1))
	passStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Green))
	failStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Red))
	skipStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay0))
	selStyle   = lipgloss.NewStyle().Background(lipgloss.Color(config.Surface1))
	barDone    = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Green))
	barFail    = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Red))
	barTodo    = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Surface2))
	borderOn   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(config.Accent))
	borderOff  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(config.Surface2))
)

// Run describes what to execute.
type Run struct {
	Title    string // e.g. "examples/openapi.yml · tag pets"
	Spec     *openapi3.T
	SpecPath string
	Options  runner.Options
}

// Model is the runner view.
type Model struct {
	run     Run
	id      int
	ch      chan tea.Msg
	cancel  context.CancelFunc
	running bool
	started time.Time
	elapsed time.Duration
	planned int
	results []runner.OpResult
	err     error

	onlyFailed  bool
	cursor      int          // index into visible()
	expanded    map[int]bool // flow results (by index) showing their steps
	offset      int
	detailFocus bool
	detail      responses.Model
	spin        spinner.Model

	width, height int
}

// New returns an idle runner view.
func New() Model {
	return Model{
		detail: responses.New("Select a result"),
		spin:   spinner.New(spinner.WithSpinner(spinner.MiniDot)),
	}
}

// Running reports whether a run is in progress.
func (m Model) Running() bool { return m.running }

// CapturesKeys reports whether a text input owns the keyboard.
func (m Model) CapturesKeys() bool { return m.detailFocus && m.detail.Searching() }

// Results returns the results received so far.
func (m Model) Results() []runner.OpResult { return m.results }

// Start begins a run (cancelling any previous one).
func (m *Model) Start(r Run) tea.Cmd {
	m.Stop()
	m.run = r
	m.id++
	m.results = nil
	m.expanded = nil
	m.err = nil
	m.cursor, m.offset = 0, 0
	m.detailFocus = false
	m.detail.Blur()
	m.detail.Clear()
	m.planned = runner.PlannedCount(r.Spec, r.SpecPath, r.Options)
	m.running = true
	m.started = time.Now()

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	ch := make(chan tea.Msg, m.planned+2) // the runner never blocks on the UI
	m.ch = ch
	id := m.id
	opts := r.Options
	opts.OnResult = func(res runner.OpResult) { ch <- resultMsg{id: id, res: res} }
	go func() {
		rep, err := runner.Run(ctx, r.Spec, r.SpecPath, opts)
		ch <- doneMsg{id: id, report: rep, err: err}
	}()
	return tea.Batch(wait(ch), m.spin.Tick)
}

// Stop cancels the run in progress; results so far are kept.
func (m *Model) Stop() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

func wait(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

// SetSize sets the outer size.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.layout()
}

func (m Model) listWidth() int { return max(30, m.width*2/5) }

// detailWidth is the outer width of the details box.
func (m Model) detailWidth() int { return max(12, m.width-m.listWidth()) }

func (m *Model) layout() {
	m.detail.SetSize(m.detailWidth()-2, m.listRows())
}

func (m Model) headerHeight() int { return 2 }

// listRow is a line of the list: a result, or one step of an expanded flow.
type listRow struct {
	res  int
	step int // -1: the result itself
}

// visible returns the rows shown in the list.
func (m Model) visible() []listRow {
	var out []listRow
	for i, r := range m.results {
		if m.onlyFailed && r.Outcome != runner.Failed && r.Outcome != runner.Errored {
			continue
		}
		out = append(out, listRow{res: i, step: -1})
		if m.expanded[i] {
			for s := range r.Steps {
				out = append(out, listRow{res: i, step: s})
			}
		}
	}
	return out
}

func (m Model) selectedRow() (listRow, bool) {
	v := m.visible()
	if m.cursor < 0 || m.cursor >= len(v) {
		return listRow{}, false
	}
	return v[m.cursor], true
}

func (m Model) selected() (runner.OpResult, bool) {
	row, ok := m.selectedRow()
	if !ok {
		return runner.OpResult{}, false
	}
	if row.step >= 0 {
		return stepResult(m.results[row.res], row.step), true
	}
	return m.results[row.res], true
}

// stepResult presents one step of a flow as a result of its own: its
// request, response and the automatic checks of that step.
func stepResult(flow runner.OpResult, i int) runner.OpResult {
	st := flow.Steps[i]
	r := runner.OpResult{OperationID: st.OperationID, Method: st.Method, Path: st.Path, Request: st.Request, Response: st.Response, Outcome: runner.Passed}
	prefix := fmt.Sprintf("#%d ", i+1)
	for _, c := range flow.Checks {
		if name, ok := strings.CutPrefix(c.Name, prefix); ok {
			c.Name = strings.TrimPrefix(strings.TrimPrefix(name, st.Method+" "+st.Path), " · ")
			r.Checks = append(r.Checks, c)
			if !c.Passed {
				r.Outcome = runner.Failed
			}
		}
	}
	if st.Error != "" {
		r.Outcome, r.Reason = runner.Errored, st.Error
	}
	return r
}

func (m *Model) showSelected() {
	if r, ok := m.selected(); ok {
		m.detail.SetResult(r)
	} else {
		m.detail.Clear()
	}
}

// HelpBindings are the keys shown in the help bar.
func (m Model) HelpBindings() []key.Binding {
	if m.detailFocus {
		return append(m.detail.HelpBindings(), DetailKey, BackKey)
	}
	return []key.Binding{DownKey, UpKey, ResponseKey, FailedKey, OpenKey, RerunKey, BackKey}
}

// Update handles run messages and keys.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case resultMsg:
		if msg.id != m.id {
			return m, nil
		}
		m.results = append(m.results, msg.res)
		if len(m.results) == 1 || (m.cursor == len(m.visible())-2 && !m.detailFocus) {
			// Follow the newest result until the user moves away.
			m.cursor = max(0, len(m.visible())-1)
		}
		m.clamp()
		m.showSelected()
		return m, wait(m.ch)

	case doneMsg:
		if msg.id != m.id {
			return m, nil
		}
		m.running = false
		m.cancel = nil
		m.elapsed = time.Since(m.started)
		m.err = msg.err
		// Jump to the first failure: that is what the user wants to see.
		for i, row := range m.visible() {
			if r := m.results[row.res]; row.step < 0 && (r.Outcome == runner.Failed || r.Outcome == runner.Errored) {
				m.cursor = i
				break
			}
		}
		m.clamp()
		m.showSelected()
		return m, nil

	case spinner.TickMsg:
		if m.running {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.updateKeys(msg)
	}
	if m.detailFocus {
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateKeys(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.detailFocus {
		if m.detail.Searching() {
			var cmd tea.Cmd
			m.detail, cmd = m.detail.Update(msg)
			return m, cmd
		}
		switch {
		case key.Matches(msg, DetailKey), key.Matches(msg, BackKey):
			m.detailFocus = false
			m.detail.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(msg, BackKey):
		if m.running {
			m.Stop()
			return m, nil
		}
		return m, func() tea.Msg { return CloseMsg{} }
	case key.Matches(msg, DownKey):
		m.cursor++
	case key.Matches(msg, UpKey):
		m.cursor--
	case msg.String() == "g" || msg.String() == "home":
		m.cursor = 0
	case msg.String() == "G" || msg.String() == "end":
		m.cursor = len(m.visible()) - 1
	case key.Matches(msg, FailedKey):
		current, ok := m.selectedRow()
		m.onlyFailed = !m.onlyFailed
		m.cursor = 0
		if ok {
			for i, row := range m.visible() {
				if row == current {
					m.cursor = i
				}
			}
		}
	case key.Matches(msg, RerunKey):
		if m.run.Spec != nil {
			return m, m.Start(m.run)
		}
		return m, nil
	case key.Matches(msg, OpenKey):
		if r, ok := m.selected(); ok && r.Kind != runner.KindFlow {
			ref := model.OpenAPIRef{FilePath: m.run.SpecPath, Path: r.Path, Method: r.Method}
			return m, func() tea.Msg { return OpenOperationMsg{Ref: ref} }
		}
		return m, nil
	case key.Matches(msg, ResponseKey) && m.toggleFlow():
		return m, nil
	case key.Matches(msg, ResponseKey), key.Matches(msg, DetailKey):
		if _, ok := m.selected(); ok {
			m.detailFocus = true
			m.detail.Focus()
		}
		return m, nil
	case msg.String() == "[" || msg.String() == "]" || (len(msg.String()) == 1 && msg.String() >= "1" && msg.String() <= "6"):
		m.detail.Focus()
		m.detail, _ = m.detail.Update(msg)
		m.detail.Blur()
		return m, nil
	default:
		return m, nil
	}
	m.clamp()
	m.showSelected()
	return m, nil
}

// toggleFlow expands or collapses the selected flow; false when the row is
// not a flow with steps.
func (m *Model) toggleFlow() bool {
	row, ok := m.selectedRow()
	if !ok || row.step >= 0 || m.results[row.res].Kind != runner.KindFlow || len(m.results[row.res].Steps) == 0 {
		return false
	}
	if m.expanded == nil {
		m.expanded = map[int]bool{}
	}
	m.expanded[row.res] = !m.expanded[row.res]
	return true
}

func (m *Model) clamp() {
	n := len(m.visible())
	m.cursor = max(0, min(m.cursor, n-1))
	rows := m.listRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	m.offset = max(0, min(m.offset, max(0, n-rows)))
}

// listRows is the inner height of both boxes (borders excluded).
func (m Model) listRows() int { return max(1, m.height-m.headerHeight()-2) }

// counts returns passed, failed (incl. errors) and skipped results.
func (m Model) counts() (passed, failed, skipped int) {
	for _, r := range m.results {
		switch r.Outcome {
		case runner.Passed:
			passed++
		case runner.Failed, runner.Errored:
			failed++
		default:
			skipped++
		}
	}
	return
}

func (m Model) header() string {
	passed, failed, skipped := m.counts()
	done := len(m.results)
	var state string
	switch {
	case m.running:
		state = m.spin.View() + fmt.Sprintf(" running %d/%d · %s · esc stops", done, m.planned, time.Since(m.started).Round(100*time.Millisecond))
	case m.err != nil:
		state = failStyle.Render("lockfile: " + m.err.Error())
	default:
		state = dimStyle.Render(fmt.Sprintf("finished in %s · R runs again · esc back", m.elapsed.Round(time.Millisecond)))
	}
	line1 := titleStyle.Render("▶ "+m.run.Title) + "  " + state
	counts := passStyle.Render(fmt.Sprintf("✓ %d passed", passed)) + "  " + failStyle.Render(fmt.Sprintf("✗ %d failed", failed)) +
		"  " + skipStyle.Render(fmt.Sprintf("○ %d skipped", skipped))
	if m.onlyFailed {
		counts += dimStyle.Render("  · showing failures (F)")
	}
	barWidth := max(10, m.width-lipgloss.Width(counts)-4)
	line2 := progressBar(passed+skipped, failed, m.planned, barWidth) + "  " + counts
	return ansi.Truncate(line1, m.width, "…") + "\n" + ansi.Truncate(line2, m.width, "…")
}

// progressBar draws done (green), failed (red) and pending cells.
func progressBar(ok, failed, total, width int) string {
	if total <= 0 {
		return barTodo.Render(strings.Repeat("━", width))
	}
	okCells := ok * width / total
	failCells := failed * width / total
	if failed > 0 && failCells == 0 {
		failCells = 1
	}
	todo := max(0, width-okCells-failCells)
	return barDone.Render(strings.Repeat("━", okCells)) + barFail.Render(strings.Repeat("━", failCells)) + barTodo.Render(strings.Repeat("━", todo))
}

func icon(o runner.Outcome) string {
	switch o {
	case runner.Passed:
		return passStyle.Render("✓")
	case runner.Failed:
		return failStyle.Render("✗")
	case runner.Errored:
		return failStyle.Render("!")
	default:
		return skipStyle.Render("○")
	}
}

func (m Model) row(lr listRow, width int, selected bool) string {
	r := m.results[lr.res]
	if lr.step >= 0 {
		r = stepResult(r, lr.step)
	}
	label := r.Label()
	switch {
	case lr.step >= 0:
		label = fmt.Sprintf("  ↳ %d. %s", lr.step+1, r.Label())
	case r.Kind == runner.KindFlow && len(r.Steps) > 0 && m.expanded[lr.res]:
		label = "▾ flow " + r.Path
	case r.Kind == runner.KindFlow && len(r.Steps) > 0:
		label = "▸ flow " + r.Path
	case r.Kind == runner.KindFlow:
		label = "flow " + r.Path
	}
	info := ""
	switch {
	case r.Response != nil:
		info = fmt.Sprintf("%d · %dms", r.Response.Status, r.Response.DurationMs)
	case lr.step >= 0 && r.Reason != "":
		info = "error"
	case r.Outcome == runner.Skipped:
		info = "skipped"
	case r.Kind == runner.KindFlow:
		info = fmt.Sprintf("%d steps", len(r.Steps))
	}
	left := icon(r.Outcome) + " " + label
	right := dimStyle.Render(info)
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	var line string
	if gap < 1 {
		line = ansi.Truncate(left, width, "…")
	} else {
		line = left + strings.Repeat(" ", gap) + right
	}
	if selected {
		return selStyle.Width(width).Render(line)
	}
	return line
}

func (m Model) list() string {
	w := m.listWidth() - 2 // inside the border
	v := m.visible()
	rows := m.listRows()
	var lines []string
	if len(v) == 0 {
		msg := "Waiting for the first result…"
		switch {
		case !m.running && m.onlyFailed:
			msg = "No failures."
		case !m.running:
			msg = "Nothing was selected to run."
		}
		lines = append(lines, dimStyle.Render(msg))
	}
	for i := m.offset; i < len(v) && i < m.offset+rows; i++ {
		lines = append(lines, m.row(v[i], w, i == m.cursor))
	}
	style := borderOn
	if m.detailFocus {
		style = borderOff
	}
	// lipgloss sizes include the border.
	return style.Width(m.listWidth()).Height(rows + 2).Render(strings.Join(lines, "\n"))
}

// View renders the header, the result list and the selected result.
func (m Model) View() string {
	style := borderOff
	if m.detailFocus {
		style = borderOn
	}
	detail := style.Width(m.detailWidth()).Height(m.listRows() + 2).Render(m.detail.View())
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.list(), detail)
	return m.header() + "\n" + body
}
