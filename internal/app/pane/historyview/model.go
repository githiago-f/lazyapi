// Package historyview lists the requests sent from the TUI (newest first),
// shows each response and compares it with the previous response of the
// same operation or with the example saved in the spec.
package historyview

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/app/pane/responses"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/diff"
	"github.com/githiago-f/lazyapi/internal/history"
	"github.com/githiago-f/lazyapi/internal/runner"
	"github.com/githiago-f/lazyapi/internal/store"
)

// Messages emitted to the TUI.
type (
	CloseMsg  struct{}
	ReplayMsg struct{ Entry history.Entry }
)

// Keys of the history view.
var (
	UpKey      = key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up"))
	DownKey    = key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down"))
	ReplayKey  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "replay in editor"))
	DiffKey    = key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "diff with previous"))
	ExampleKey = key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "diff with example"))
	DetailKey  = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "list/details"))
	BackKey    = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))
)

var (
	dimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1))
	passStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Green))
	failStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Red))
	hunkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender))
	titleSty  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(config.Peach))
	selStyle  = lipgloss.NewStyle().Background(lipgloss.Color(config.Surface1))
	borderOn  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(config.Peach))
	borderOff = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(config.Surface2))
)

// Model is the history view.
type Model struct {
	entries []history.Entry // newest first
	cursor  int
	offset  int

	detail      responses.Model
	detailFocus bool

	diffing  bool
	diffVP   viewport.Model
	diffHead string

	width, height int
}

// New returns an empty history view.
func New() Model {
	vp := viewport.New()
	vp.SoftWrap = true
	return Model{detail: responses.New("No requests yet: send one from the list or the editor"), diffVP: vp}
}

// SetEntries replaces the entries (newest first).
func (m *Model) SetEntries(entries []history.Entry) {
	m.entries = entries
	m.cursor, m.offset = 0, 0
	m.diffing = false
	m.showSelected()
}

// Len is the number of entries.
func (m Model) Len() int { return len(m.entries) }

// CapturesKeys reports whether a text input owns the keyboard.
func (m Model) CapturesKeys() bool { return m.detailFocus && m.detail.Searching() }

// SetSize sets the outer size.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.detail.SetSize(m.detailWidth()-2, m.rows())
	m.diffVP.SetWidth(m.detailWidth() - 2)
	m.diffVP.SetHeight(max(1, m.rows()-1))
}

func (m Model) listWidth() int   { return max(30, m.width*2/5) }
func (m Model) detailWidth() int { return max(12, m.width-m.listWidth()) }
func (m Model) rows() int        { return max(1, m.height-1-2) } // header line, borders

func (m Model) selected() (history.Entry, bool) {
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return history.Entry{}, false
	}
	return m.entries[m.cursor], true
}

func (m *Model) showSelected() {
	m.diffing = false
	if e, ok := m.selected(); ok {
		m.detail.SetResult(e.Result)
	} else {
		m.detail.Clear()
	}
}

// HelpBindings are the keys shown in the help bar.
func (m Model) HelpBindings() []key.Binding {
	if m.detailFocus && !m.diffing {
		return append(m.detail.HelpBindings(), DetailKey, BackKey)
	}
	return []key.Binding{DownKey, UpKey, ReplayKey, DiffKey, ExampleKey, DetailKey, BackKey}
}

// Update handles keys.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		if m.diffing {
			m.diffVP, cmd = m.diffVP.Update(msg)
		} else {
			m.detail, cmd = m.detail.Update(msg)
		}
		return m, cmd
	}

	if m.detailFocus {
		if m.detail.Searching() {
			var cmd tea.Cmd
			m.detail, cmd = m.detail.Update(k)
			return m, cmd
		}
		if key.Matches(k, DetailKey) || key.Matches(k, BackKey) {
			m.detailFocus = false
			m.detail.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		if m.diffing {
			m.diffVP, cmd = m.diffVP.Update(k)
		} else {
			m.detail, cmd = m.detail.Update(k)
		}
		return m, cmd
	}

	switch {
	case key.Matches(k, BackKey):
		if m.diffing {
			m.diffing = false
			return m, nil
		}
		return m, func() tea.Msg { return CloseMsg{} }
	case key.Matches(k, DownKey):
		m.move(1)
	case key.Matches(k, UpKey):
		m.move(-1)
	case k.String() == "pgdown":
		m.move(m.rows())
	case k.String() == "pgup":
		m.move(-m.rows())
	case key.Matches(k, DetailKey):
		if _, ok := m.selected(); ok {
			m.detailFocus = true
			if !m.diffing {
				m.detail.Focus()
			}
		}
	case key.Matches(k, ReplayKey):
		if e, ok := m.selected(); ok {
			return m, func() tea.Msg { return ReplayMsg{Entry: e} }
		}
	case key.Matches(k, DiffKey):
		m.diffPrevious()
	case key.Matches(k, ExampleKey):
		m.diffExample()
	case k.String() == "[" || k.String() == "]" || (len(k.String()) == 1 && k.String() >= "1" && k.String() <= "6"):
		m.detail.Focus()
		m.detail, _ = m.detail.Update(k)
		m.detail.Blur()
	}
	return m, nil
}

func (m *Model) move(d int) {
	if len(m.entries) == 0 {
		return
	}
	m.cursor = max(0, min(len(m.entries)-1, m.cursor+d))
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.rows() {
		m.offset = m.cursor - m.rows() + 1
	}
	m.showSelected()
}

// previous is the next older entry of the same operation.
func (m Model) previous() (history.Entry, bool) {
	cur, ok := m.selected()
	if !ok {
		return history.Entry{}, false
	}
	for _, e := range m.entries[m.cursor+1:] {
		if e.SameOperation(cur) && e.Result.Response != nil {
			return e, true
		}
	}
	return history.Entry{}, false
}

func bodyOf(r *runner.ResponseInfo) (string, string) {
	if r == nil {
		return "", ""
	}
	ct := r.Headers["Content-Type"]
	return ct, normalize(ct, r.Body)
}

// normalize prepares a body for diffing: JSON is re-indented with sorted
// keys, so only real changes show up (not key order).
func normalize(ct, body string) string {
	var v any
	dec := json.NewDecoder(strings.NewReader(body))
	dec.UseNumber()
	if dec.Decode(&v) == nil {
		var b strings.Builder
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if enc.Encode(v) == nil {
			return strings.TrimSuffix(b.String(), "\n")
		}
	}
	return responses.Pretty(ct, body)
}

func (m *Model) diffPrevious() {
	cur, ok := m.selected()
	if !ok || cur.Result.Response == nil {
		m.setDiff(dimStyle.Render("This request has no response to compare."), "")
		return
	}
	prev, ok := m.previous()
	if !ok {
		m.setDiff(dimStyle.Render("No earlier response of "+cur.Label()+" in the history."), "")
		return
	}
	_, a := bodyOf(prev.Result.Response)
	_, b := bodyOf(cur.Result.Response)
	head := fmt.Sprintf("%s · %s → %s · status %d → %d", cur.Label(), stamp(prev.Time), stamp(cur.Time),
		prev.Result.Response.Status, cur.Result.Response.Status)
	m.setDiff(render(diff.Lines(a, b)), head)
}

func (m *Model) diffExample() {
	cur, ok := m.selected()
	if !ok || cur.Result.Response == nil {
		m.setDiff(dimStyle.Render("This request has no response to compare."), "")
		return
	}
	if cur.Ref == nil || cur.Spec == "" {
		m.setDiff(dimStyle.Render("This request is not an operation of a spec."), "")
		return
	}
	spec, err := store.ParseSpec(cur.Spec)
	if err != nil {
		m.setDiff(failStyle.Render(err.Error()), "")
		return
	}
	status := cur.Result.Response.Status
	ct, example, ok := store.ResponseExample(store.FindOperation(spec, *cur.Ref), status)
	if !ok {
		m.setDiff(dimStyle.Render(fmt.Sprintf("The spec has no example for %s %d.\nctrl+e in the editor saves a response as the example.", cur.Label(), status)), "")
		return
	}
	_, b := bodyOf(cur.Result.Response)
	head := fmt.Sprintf("%s · example (%d) → response of %s", cur.Label(), status, stamp(cur.Time))
	m.setDiff(render(diff.Lines(normalize(ct, example), b)), head)
}

func (m *Model) setDiff(content, head string) {
	m.diffing = true
	m.diffHead = head
	m.diffVP.SetContent(content)
	m.diffVP.GotoTop()
}

// render colors a diff; identical inputs say so.
func render(lines []diff.Line) string {
	if !diff.Changed(lines) {
		return passStyle.Render("✓ Identical bodies")
	}
	added, removed := diff.Stats(lines)
	var b strings.Builder
	b.WriteString(passStyle.Render(fmt.Sprintf("+%d", added)) + " " + failStyle.Render(fmt.Sprintf("−%d", removed)) + "\n")
	for _, h := range diff.Hunks(lines, 3) {
		if h.Skipped > 0 {
			b.WriteString(hunkStyle.Render(fmt.Sprintf("@@ %d unchanged %s @@", h.Skipped, plural(h.Skipped, "line"))) + "\n")
		}
		for _, l := range h.Lines {
			switch l.Op {
			case diff.Insert:
				b.WriteString(passStyle.Render("+ "+l.Text) + "\n")
			case diff.Delete:
				b.WriteString(failStyle.Render("- "+l.Text) + "\n")
			default:
				b.WriteString(dimStyle.Render("  "+l.Text) + "\n")
			}
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func plural(n int, w string) string {
	if n == 1 {
		return w
	}
	return w + "s"
}

func stamp(t time.Time) string {
	if time.Since(t) < 20*time.Hour {
		return t.Format("15:04:05")
	}
	return t.Format("Jan 2 15:04")
}

func (m Model) row(e history.Entry, width int, selected bool) string {
	icon := passStyle.Render("✓")
	switch e.Result.Outcome {
	case runner.Failed:
		icon = failStyle.Render("✗")
	case runner.Errored:
		icon = failStyle.Render("!")
	}
	info := "error"
	if r := e.Result.Response; r != nil {
		info = fmt.Sprintf("%d · %dms", r.Status, r.DurationMs)
	}
	left := dimStyle.Render(stamp(e.Time)) + " " + icon + " " + e.Label()
	right := dimStyle.Render(info)
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	line := ansi.Truncate(left, width, "…")
	if gap >= 1 {
		line = left + strings.Repeat(" ", gap) + right
	}
	if selected {
		return selStyle.Width(width).Render(line)
	}
	return line
}

// View renders the list and the selected entry (or a diff).
func (m Model) View() string {
	head := titleSty.Render("History") + dimStyle.Render(fmt.Sprintf("  %d %s · newest first", len(m.entries), plural(len(m.entries), "request")))
	w := m.listWidth() - 2
	var lines []string
	if len(m.entries) == 0 {
		lines = append(lines, dimStyle.Render("Nothing sent yet."))
	}
	for i := m.offset; i < len(m.entries) && i < m.offset+m.rows(); i++ {
		lines = append(lines, m.row(m.entries[i], w, i == m.cursor))
	}
	listStyle, detailStyle := borderOn, borderOff
	if m.detailFocus {
		listStyle, detailStyle = borderOff, borderOn
	}
	list := listStyle.Width(m.listWidth()).Height(m.rows() + 2).Render(strings.Join(lines, "\n"))

	var right string
	if m.diffing {
		top := dimStyle.Render("esc closes the diff")
		if m.diffHead != "" {
			top = ansi.Truncate(hunkStyle.Render(m.diffHead), m.detailWidth()-2, "…")
		}
		right = top + "\n" + m.diffVP.View()
	} else {
		right = m.detail.View()
	}
	detail := detailStyle.Width(m.detailWidth()).Height(m.rows() + 2).Render(right)
	return ansi.Truncate(head, m.width, "…") + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, list, detail)
}
