package responses

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/runner"
)

// Tab is a section of the response view.
type Tab int

const (
	BodyTab Tab = iota
	HeadersTab
	ChecksTab
	ConsoleTab
)

var tabNames = []string{"Body", "Headers", "Checks", "Console"}

var (
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Subtext0))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1))
	keyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender))
	tabStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1)).Padding(0, 1)
	tabActive     = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Peach)).Bold(true).Underline(true).Padding(0, 1)
	passStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Green))
	failStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Red))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Yellow))
	errorBoxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Red)).Bold(true)
	badgeBase     = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(lipgloss.Color(config.Crust))
)

// Keys used inside the response view.
var (
	NextTabKey = key.NewBinding(key.WithKeys("]"), key.WithHelp("[/]", "response tab"))
	PrevTabKey = key.NewBinding(key.WithKeys("["), key.WithHelp("[", "previous tab"))
	RawKey     = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "raw/pretty"))
	SearchKey  = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search"))
	NextHitKey = key.NewBinding(key.WithKeys("n"), key.WithHelp("n/N", "next/prev match"))
	PrevHitKey = key.NewBinding(key.WithKeys("N"))
)

// Model shows the outcome of one request.
type Model struct {
	result  *runner.OpResult
	sending bool
	spin    spinner.Model

	tab     Tab
	raw     bool
	vp      viewport.Model
	width   int
	height  int
	focused bool

	searching bool
	search    textinput.Model
	query     string
	hits      int

	placeholder string
}

// New returns an empty response view.
func New(placeholder string) Model {
	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	vp.SoftWrap = true
	vp.HighlightStyle = lipgloss.NewStyle().Background(lipgloss.Color(config.Surface2))
	vp.SelectedHighlightStyle = lipgloss.NewStyle().Background(lipgloss.Color(config.Peach)).Foreground(lipgloss.Color(config.Crust))

	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = "search the response"

	return Model{
		vp:          vp,
		spin:        spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		search:      ti,
		placeholder: placeholder,
	}
}

// SetSize sets the outer size of the view.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = max(0, w), max(0, h)
	m.vp.SetWidth(m.width)
	m.vp.SetHeight(max(0, m.height-m.chromeHeight()))
	m.search.SetWidth(max(1, m.width-2))
	m.refresh()
}

func (m Model) chromeHeight() int {
	h := 2 // status line + tab bar
	if m.searching || m.query != "" {
		h++
	}
	return h
}

// Focus and Blur control whether keys are handled.
func (m *Model) Focus() { m.focused = true }
func (m *Model) Blur() {
	m.focused = false
	m.searching = false
	m.search.Blur()
}

// Focused reports whether the view receives keys.
func (m Model) Focused() bool { return m.focused }

// Searching reports whether the search input is open (it captures keys).
func (m Model) Searching() bool { return m.searching }

// Sending reports whether a request is in flight.
func (m Model) Sending() bool { return m.sending }

// StartSending shows the spinner until SetResult is called.
func (m *Model) StartSending() tea.Cmd {
	m.sending = true
	return m.spin.Tick
}

// SetResult shows a finished exchange. It jumps to the Checks tab when a
// check failed or the request errored, otherwise keeps the current tab.
func (m *Model) SetResult(res runner.OpResult) {
	m.sending = false
	m.result = &res
	if res.Outcome == runner.Failed && m.tab == BodyTab {
		m.tab = ChecksTab
	}
	m.vp.GotoTop()
	m.refresh()
}

// Clear removes the current result.
func (m *Model) Clear() {
	m.result = nil
	m.sending = false
	m.query = ""
	m.tab = BodyTab
	m.refresh()
}

// Result returns the shown result, if any.
func (m Model) Result() *runner.OpResult { return m.result }

// Body returns the raw response body ("" when there is none).
func (m Model) Body() string {
	if m.result == nil || m.result.Response == nil {
		return ""
	}
	return m.result.Response.Body
}

// HelpBindings are the keys available while the view is focused.
func (m Model) HelpBindings() []key.Binding {
	if m.searching {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "find")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	}
	b := []key.Binding{NextTabKey, RawKey, SearchKey}
	if m.query != "" {
		b = append(b, NextHitKey)
	}
	return b
}

// Update handles keys (when focused), mouse wheel and the spinner.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.sending {
			m.spin, cmd = m.spin.Update(msg)
		}
		return m, cmd

	case tea.KeyPressMsg:
		if !m.focused {
			return m, nil
		}
		if m.searching {
			return m.updateSearch(msg)
		}
		switch {
		case key.Matches(msg, NextTabKey):
			m.setTab(Tab((int(m.tab) + 1) % len(tabNames)))
			return m, nil
		case key.Matches(msg, PrevTabKey):
			m.setTab(Tab((int(m.tab) + len(tabNames) - 1) % len(tabNames)))
			return m, nil
		case msg.String() >= "1" && msg.String() <= "4" && len(msg.String()) == 1:
			m.setTab(Tab(msg.String()[0] - '1'))
			return m, nil
		case key.Matches(msg, RawKey) && m.tab == BodyTab:
			m.raw = !m.raw
			m.refresh()
			return m, nil
		case key.Matches(msg, SearchKey):
			m.searching = true
			m.search.SetValue(m.query)
			m.SetSize(m.width, m.height)
			return m, m.search.Focus()
		case key.Matches(msg, NextHitKey) && m.query != "":
			m.vp.HighlightNext()
			return m, nil
		case key.Matches(msg, PrevHitKey) && m.query != "":
			m.vp.HighlightPrevious()
			return m, nil
		}
	}
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m Model) updateSearch(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.search.Blur()
		m.SetSize(m.width, m.height)
		return m, nil
	case "enter":
		m.searching = false
		m.search.Blur()
		m.query = m.search.Value()
		m.SetSize(m.width, m.height) // re-renders (plain while searching) and applies the search
		return m, nil
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	return m, cmd
}

func (m *Model) setTab(t Tab) {
	m.tab = t
	m.vp.GotoTop()
	m.refresh()
}

// refresh re-renders the active tab into the viewport. While a search is
// active the content is shown without colors: the viewport positions
// highlights on plain text only.
func (m *Model) refresh() {
	content := m.content()
	if m.query != "" {
		content = ansi.Strip(content)
	}
	m.vp.SetContent(content)
	m.applySearch()
}

func (m *Model) applySearch() {
	m.vp.ClearHighlights()
	m.hits = 0
	if m.query == "" {
		return
	}
	re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(m.query))
	if err != nil {
		return
	}
	matches := re.FindAllStringIndex(m.vp.GetContent(), -1)
	m.hits = len(matches)
	m.vp.SetHighlights(matches)
}

func (m Model) content() string {
	r := m.result
	if r == nil {
		return ""
	}
	switch m.tab {
	case BodyTab:
		if r.Response == nil {
			return errorText(r)
		}
		if r.Response.Body == "" {
			return dimStyle.Render("(empty body)")
		}
		ct := r.Response.Headers["Content-Type"]
		if m.raw {
			return r.Response.Body
		}
		return Highlight(ct, Pretty(ct, r.Response.Body))

	case HeadersTab:
		var b strings.Builder
		if r.Request != nil {
			b.WriteString(labelStyle.Render("Request") + "\n")
			fmt.Fprintf(&b, "  %s %s\n", r.Request.Method, r.Request.URL)
			writeHeaders(&b, r.Request.Headers)
			b.WriteString("\n")
		}
		if r.Response != nil {
			b.WriteString(labelStyle.Render("Response") + "\n")
			writeHeaders(&b, r.Response.Headers)
		}
		return strings.TrimRight(b.String(), "\n")

	case ChecksTab:
		if r.Response == nil {
			return errorText(r)
		}
		if len(r.Checks) == 0 {
			return dimStyle.Render("No checks: this request is not linked to an operation with declared responses,\nand it has no test script. Add one in the Tests tab.")
		}
		var b strings.Builder
		for _, c := range r.Checks {
			mark := passStyle.Render("✓")
			if !c.Passed {
				mark = failStyle.Render("✗")
			}
			fmt.Fprintf(&b, "%s %s %s\n", mark, dimStyle.Render(fmt.Sprintf("%-8s", c.Kind)), c.Name)
			if c.Message != "" {
				fmt.Fprintf(&b, "  %s\n", failStyle.Render(c.Message))
			}
		}
		if r.TestsFile == "" {
			b.WriteString("\n" + dimStyle.Render("No saved test script. Write one in the Tests tab (F5 runs it)."))
		}
		return strings.TrimRight(b.String(), "\n")

	case ConsoleTab:
		if len(r.Logs) == 0 {
			return dimStyle.Render("No console output. console.log() in a test script shows up here.")
		}
		var b strings.Builder
		for _, l := range r.Logs {
			style := dimStyle
			switch l.Level {
			case "warn":
				style = warnStyle
			case "error":
				style = failStyle
			}
			fmt.Fprintf(&b, "%s %s\n", style.Render(fmt.Sprintf("[%s]", l.Level)), l.Message)
		}
		return strings.TrimRight(b.String(), "\n")
	}
	return ""
}

func errorText(r *runner.OpResult) string {
	return errorBoxStyle.Render("✗ "+r.Reason) + "\n\n" +
		dimStyle.Render("Nothing came back. Check the server URL, path parameters and network.")
}

func writeHeaders(b *strings.Builder, h map[string]string) {
	names := make([]string, 0, len(h))
	for k := range h {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		fmt.Fprintf(b, "  %s %s\n", keyStyle.Render(k+":"), h[k])
	}
}

// StatusBadge renders "200 OK" colored by status class.
func StatusBadge(status int, text string) string {
	color := config.Green
	switch {
	case status >= 500:
		color = config.Red
	case status >= 400:
		color = config.Peach
	case status >= 300:
		color = config.Blue
	}
	label := strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprint(status)))
	return badgeBase.Background(lipgloss.Color(color)).Render(strings.TrimSpace(fmt.Sprintf("%d %s", status, label)))
}

func (m Model) statusLine() string {
	if m.sending {
		return m.spin.View() + " " + labelStyle.Render("Sending…  esc cancels")
	}
	r := m.result
	if r == nil {
		return dimStyle.Render(m.placeholder)
	}
	if r.Response == nil {
		return badgeBase.Background(lipgloss.Color(config.Red)).Render("ERROR") + " " + failStyle.Render(r.Reason)
	}
	parts := []string{
		StatusBadge(r.Response.Status, r.Response.StatusText),
		labelStyle.Render(fmt.Sprintf("%d ms", r.Response.DurationMs)),
		labelStyle.Render(HumanSize(len(r.Response.Body))),
	}
	if n := len(r.Checks); n > 0 {
		failed := 0
		for _, c := range r.Checks {
			if !c.Passed {
				failed++
			}
		}
		if failed == 0 {
			parts = append(parts, passStyle.Render(fmt.Sprintf("✓ %d %s", n, plural(n, "check", "checks"))))
		} else {
			parts = append(parts, failStyle.Render(fmt.Sprintf("✗ %d/%d checks failed", failed, n)))
		}
	}
	if r.Attempts > 1 {
		parts = append(parts, warnStyle.Render(fmt.Sprintf("%d attempts", r.Attempts)))
	}
	return strings.Join(parts, dimStyle.Render(" · "))
}

func (m Model) tabBar() string {
	var tabs []string
	for i, name := range tabNames {
		label := name
		if m.result != nil {
			switch Tab(i) {
			case ChecksTab:
				if n := len(m.result.Checks); n > 0 {
					label += fmt.Sprintf(" %d", n)
				}
			case ConsoleTab:
				if n := len(m.result.Logs); n > 0 {
					label += fmt.Sprintf(" %d", n)
				}
			case BodyTab:
				if m.raw {
					label += " (raw)"
				}
			}
		}
		if Tab(i) == m.tab {
			tabs = append(tabs, tabActive.Render(label))
		} else {
			tabs = append(tabs, tabStyle.Render(label))
		}
	}
	bar := strings.Join(tabs, "")
	if m.focused {
		bar += dimStyle.Render("  [ ] switch")
	}
	return ansi.Truncate(bar, m.width, "…")
}

// View renders the status line, tab bar and the active tab.
func (m Model) View() string {
	lines := []string{ansi.Truncate(m.statusLine(), m.width, "…")}
	if m.result == nil && !m.sending {
		return lipgloss.NewStyle().Width(m.width).Height(m.height).Render(strings.Join(lines, "\n"))
	}
	lines = append(lines, m.tabBar())
	if m.searching {
		lines = append(lines, m.search.View())
	} else if m.query != "" {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("/%s — %d %s · n/N next/prev · / then empty enter clears", m.query, m.hits, plural(m.hits, "match", "matches"))))
	}
	lines = append(lines, m.vp.View())
	return lipgloss.NewStyle().Width(m.width).Height(m.height).MaxHeight(m.height).Render(strings.Join(lines, "\n"))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
