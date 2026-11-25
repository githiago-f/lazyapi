package responses

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/jq"
	"github.com/githiago-f/lazyapi/internal/runner"
)

// Tab is a section of the response view.
type Tab int

const (
	BodyTab Tab = iota
	HeadersTab
	CookiesTab
	TimelineTab
	ChecksTab
	ConsoleTab
)

var tabNames = []string{
	config.T("response.tabs.body"), config.T("response.tabs.headers"), config.T("response.tabs.cookies"),
	config.T("response.tabs.timeline"), config.T("response.tabs.checks"), config.T("response.tabs.console"),
}

var (
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Subtext0))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1))
	keyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender))
	tabStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1)).Padding(0, 1)
	tabActive     = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Accent)).Bold(true).Underline(true).Padding(0, 1)
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
	FilterKey  = key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "jq filter"))
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

	filtering bool // the jq input is open
	filter    textinput.Model
	jq        string // applied jq filter ("" = none)
	jqOut     string
	jqErr     string

	placeholder string
}

// New returns an empty response view.
func New(placeholder string) Model {
	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	vp.SoftWrap = true
	vp.HighlightStyle = lipgloss.NewStyle().Background(lipgloss.Color(config.Surface2))
	vp.SelectedHighlightStyle = lipgloss.NewStyle().Background(lipgloss.Color(config.Accent)).Foreground(lipgloss.Color(config.OnAccent))

	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = config.T("response.search_placeholder")

	fi := textinput.New()
	fi.Prompt = "jq "
	fi.Placeholder = ".items[] | {id, name}"

	return Model{
		vp:          vp,
		spin:        spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		search:      ti,
		filter:      fi,
		placeholder: placeholder,
	}
}

// SetSize sets the outer size of the view.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = max(0, w), max(0, h)
	m.vp.SetWidth(m.width)
	m.vp.SetHeight(max(0, m.height-m.chromeHeight()))
	m.search.SetWidth(max(1, m.width-2))
	m.filter.SetWidth(max(1, m.width-4))
	m.refresh()
}

func (m Model) chromeHeight() int {
	h := 2 // status line + tab bar
	if m.searching || m.query != "" {
		h++
	}
	if m.filtering || m.jq != "" {
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
	m.filtering = false
	m.filter.Blur()
}

// Focused reports whether the view receives keys.
func (m Model) Focused() bool { return m.focused }

// Searching reports whether the search or jq input is open (it captures keys).
func (m Model) Searching() bool { return m.searching || m.filtering }

// Filter returns the applied jq filter ("" when none).
func (m Model) Filter() string { return m.jq }

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
	m.runFilter()
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
	m.jq, m.jqOut, m.jqErr = "", "", ""
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
	if m.filtering {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply (empty clears)")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	}
	if m.searching {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "find")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	}
	b := []key.Binding{NextTabKey, RawKey, SearchKey, FilterKey}
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
		if m.filtering {
			return m.updateFilter(msg)
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
		case len(msg.String()) == 1 && msg.String() >= "1" && msg.String() <= fmt.Sprint(len(tabNames)):
			m.setTab(Tab(msg.String()[0] - '1'))
			return m, nil
		case key.Matches(msg, RawKey) && m.tab == BodyTab:
			m.raw = !m.raw
			m.refresh()
			return m, nil
		case key.Matches(msg, FilterKey) && m.result != nil && m.result.Response != nil:
			m.filtering = true
			m.filter.SetValue(m.jq)
			m.filter.CursorEnd()
			m.setTab(BodyTab)
			m.SetSize(m.width, m.height)
			return m, m.filter.Focus()
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

func (m Model) updateFilter(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filter.Blur()
		m.SetSize(m.width, m.height)
		return m, nil
	case "enter":
		m.filtering = false
		m.filter.Blur()
		m.SetFilter(m.filter.Value())
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	return m, cmd
}

// SetFilter applies a jq filter to the body ("" removes it). It stays applied
// across responses, so re-sending shows the same slice of the new body.
func (m *Model) SetFilter(filter string) {
	m.jq = strings.TrimSpace(filter)
	m.runFilter()
	m.vp.GotoTop()
	m.SetSize(m.width, m.height)
}

func (m *Model) runFilter() {
	m.jqOut, m.jqErr = "", ""
	if m.jq == "" || m.result == nil || m.result.Response == nil {
		return
	}
	out, err := jq.Apply(m.jq, m.result.Response.Body)
	if err != nil {
		m.jqErr = err.Error()
	}
	m.jqOut = out
}

func (m *Model) setTab(t Tab) {
	m.tab = t
	m.vp.GotoTop()
	m.refresh()
}

// refresh re-renders the active tab into the viewport and re-applies the
// search.
func (m *Model) refresh() {
	m.vp.SetContent(m.content())
	m.applySearch()
}

// applySearch highlights every case-insensitive match of the query. It
// searches the text without colors (so a query never matches inside an
// escape sequence) and maps the matches back to the colored content.
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
	raw := m.vp.GetContent()
	matches := rawOffsets(raw, re.FindAllStringIndex(ansi.Strip(raw), -1))
	m.hits = len(matches)
	m.vp.SetHighlights(matches)
}

// rawOffsets converts byte ranges of ansi.Strip(raw) into byte ranges of
// raw. Ranges must be sorted and must not overlap.
func rawOffsets(raw string, plain [][]int) [][]int {
	if len(plain) == 0 {
		return nil
	}
	out := make([][]int, len(plain))
	bounds := make([]int, 0, len(plain)*2) // start, end, start, end, ...
	for _, m := range plain {
		bounds = append(bounds, m[0], m[1])
	}
	mapped := make([]int, len(bounds))
	next := 0
	plainPos, rawPos := 0, 0
	var state byte
	for next < len(bounds) {
		// A start maps to the raw position of the first text byte at or after
		// it; an end maps to the raw position right after the last text byte.
		for next < len(bounds) && bounds[next] == plainPos && next%2 == 1 {
			mapped[next] = rawPos
			next++
		}
		if rawPos >= len(raw) {
			break
		}
		seq, _, n, newState := ansi.DecodeSequence(raw[rawPos:], state, nil)
		state = newState
		isText := seq != "" && seq[0] != '\x1b' && seq[0] != 0x9b
		if isText {
			for next < len(bounds) && bounds[next] == plainPos && next%2 == 0 {
				mapped[next] = rawPos
				next++
			}
			plainPos += n
		}
		rawPos += n
	}
	for ; next < len(bounds); next++ {
		mapped[next] = len(raw)
	}
	for i := range out {
		out[i] = []int{mapped[2*i], mapped[2*i+1]}
	}
	return out
}

func (m Model) content() string {
	r := m.result
	if r == nil {
		return ""
	}
	switch m.tab {
	case BodyTab:
		if r.Kind == runner.KindFlow {
			return flowSteps(r)
		}
		if r.Response == nil {
			return errorText(r)
		}
		if r.Response.Body == "" {
			return dimStyle.Render(config.T("response.empty_body"))
		}
		ct := r.Response.Headers["Content-Type"]
		if m.jq != "" {
			if m.jqErr != "" {
				return failStyle.Render(m.jqErr)
			}
			if m.jqOut == "" {
				return dimStyle.Render(config.T("response.jq_empty"))
			}
			return Highlight("application/json", m.jqOut)
		}
		if m.raw {
			return r.Response.Body
		}
		return Highlight(ct, Pretty(ct, r.Response.Body))

	case HeadersTab:
		var b strings.Builder
		if r.Request != nil {
			b.WriteString(labelStyle.Render(config.T("response.request")) + "\n")
			fmt.Fprintf(&b, "  %s %s\n", r.Request.Method, r.Request.URL)
			writeHeaders(&b, r.Request.Headers)
			b.WriteString("\n")
		}
		if r.Response != nil {
			b.WriteString(labelStyle.Render(config.T("response.response")) + "\n")
			writeHeaders(&b, r.Response.Headers)
		}
		return strings.TrimRight(b.String(), "\n")

	case CookiesTab:
		if r.Response == nil {
			return errorText(r)
		}
		if len(r.Response.Cookies) == 0 {
			return dimStyle.Render(config.T("response.no_cookies"))
		}
		var b strings.Builder
		for _, c := range r.Response.Cookies {
			fmt.Fprintf(&b, "%s %s\n", keyStyle.Render(c.Name+"="), c.Value)
			var attrs []string
			for _, a := range []struct{ k, v string }{{"domain", c.Domain}, {"path", c.Path}, {"expires", c.Expires}, {"sameSite", c.SameSite}} {
				if a.v != "" {
					attrs = append(attrs, a.k+"="+a.v)
				}
			}
			if c.MaxAge > 0 {
				attrs = append(attrs, fmt.Sprintf("maxAge=%d", c.MaxAge))
			}
			if c.HTTPOnly {
				attrs = append(attrs, "HttpOnly")
			}
			if c.Secure {
				attrs = append(attrs, "Secure")
			}
			if len(attrs) > 0 {
				b.WriteString("  " + dimStyle.Render(strings.Join(attrs, " · ")) + "\n")
			}
		}
		return strings.TrimRight(b.String(), "\n")

	case TimelineTab:
		if r.Response == nil {
			return errorText(r)
		}
		return timeline(r.Response.Timing, r.Response.DurationMs, m.width)

	case ChecksTab:
		if r.Response == nil {
			return errorText(r)
		}
		if len(r.Checks) == 0 {
			return dimStyle.Render(config.T("response.no_checks"))
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
			b.WriteString("\n" + dimStyle.Render(config.T("response.no_script", "run", config.DefaultKeyMap.RunTests.Help().Key)))
		}
		return strings.TrimRight(b.String(), "\n")

	case ConsoleTab:
		if len(r.Logs) == 0 {
			return dimStyle.Render(config.T("response.no_console"))
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
	if r.Outcome == runner.Skipped || r.Outcome == runner.Planned {
		return dimStyle.Render(config.T("response.not_sent", "reason", r.Reason))
	}
	return errorBoxStyle.Render("✗ "+r.Reason) + "\n\n" +
		dimStyle.Render(config.T("response.nothing"))
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
		return m.spin.View() + " " + labelStyle.Render(config.T("response.sending", "cancel", "esc"))
	}
	r := m.result
	if r == nil {
		return dimStyle.Render(m.placeholder)
	}
	if r.Kind == runner.KindFlow {
		return m.flowStatusLine()
	}
	if r.Response == nil && (r.Outcome == runner.Skipped || r.Outcome == runner.Planned) {
		return badgeBase.Background(lipgloss.Color(config.Overlay1)).Render(strings.ToUpper(string(r.Outcome))) + " " + dimStyle.Render(r.Reason)
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
			parts = append(parts, passStyle.Render(config.Plural("response.checks_passed", n)))
		} else {
			parts = append(parts, failStyle.Render(config.T("response.checks_failed", "failed", strconv.Itoa(failed), "n", strconv.Itoa(n))))
		}
	}
	if r.Attempts > 1 {
		parts = append(parts, warnStyle.Render(config.T("response.attempts", "n", strconv.Itoa(r.Attempts))))
	}
	return strings.Join(parts, dimStyle.Render(" · "))
}

func (m Model) flowStatusLine() string {
	r := m.result
	color := config.Green
	switch r.Outcome {
	case runner.Failed, runner.Errored:
		color = config.Red
	case runner.Skipped:
		color = config.Overlay1
	}
	parts := []string{
		badgeBase.Background(lipgloss.Color(color)).Render("FLOW " + strings.ToUpper(string(r.Outcome))),
		labelStyle.Render(config.Plural("response.steps", len(r.Steps))),
	}
	failed := 0
	for _, c := range r.Checks {
		if !c.Passed {
			failed++
		}
	}
	if n := len(r.Checks); n > 0 {
		if failed == 0 {
			parts = append(parts, passStyle.Render(config.Plural("response.checks_passed", n)))
		} else {
			parts = append(parts, failStyle.Render(config.T("response.checks_failed", "failed", strconv.Itoa(failed), "n", strconv.Itoa(n))))
		}
	}
	if r.Reason != "" {
		parts = append(parts, failStyle.Render(r.Reason))
	}
	return strings.Join(parts, dimStyle.Render(" · "))
}

// flowSteps lists the requests a flow sent.
func flowSteps(r *runner.OpResult) string {
	if len(r.Steps) == 0 {
		if r.Reason != "" {
			return failStyle.Render("✗ " + r.Reason)
		}
		return dimStyle.Render(config.T("response.flow_empty"))
	}
	var b strings.Builder
	for i, st := range r.Steps {
		status := dimStyle.Render("—")
		if st.Status > 0 {
			status = StatusBadge(st.Status, "")
		}
		fmt.Fprintf(&b, "%2d. %s %s %s %s\n", i+1, status, keyStyle.Render(st.Method), st.Path, dimStyle.Render(fmt.Sprintf("%d ms", st.DurationMs)))
		if st.Error != "" {
			b.WriteString("    " + failStyle.Render(st.Error) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
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
			case CookiesTab:
				if m.result.Response != nil && len(m.result.Response.Cookies) > 0 {
					label += fmt.Sprintf(" %d", len(m.result.Response.Cookies))
				}
			case BodyTab:
				if m.raw {
					label += config.T("response.raw")
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
		bar += dimStyle.Render("  " + config.T("response.switch_hint"))
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
	if m.filtering {
		lines = append(lines, m.filter.View())
	} else if m.jq != "" {
		state := config.T("response.jq_status", "filter", m.jq)
		if m.jqErr != "" {
			state = config.T("response.jq_error") + state
		}
		lines = append(lines, dimStyle.Render(ansi.Truncate(state, m.width, "…")))
	}
	if m.searching {
		lines = append(lines, m.search.View())
	} else if m.query != "" {
		lines = append(lines, dimStyle.Render(config.T("response.search_status", "query", m.query, "matches", config.Plural("response.matches", m.hits))))
	}
	lines = append(lines, m.vp.View())
	return lipgloss.NewStyle().Width(m.width).Height(m.height).MaxHeight(m.height).Render(strings.Join(lines, "\n"))
}

// timeline draws the request phases as proportional bars.
func timeline(t *runner.TimingInfo, durationMs int64, width int) string {
	if t == nil {
		return dimStyle.Render(config.T("response.timing.no_phases", "ms", strconv.FormatInt(durationMs, 10)))
	}
	type phase struct {
		name  string
		ms    float64
		color string
	}
	transfer := t.TotalMs - t.DNSMs - t.ConnectMs - t.TLSMs - t.TTFBMs
	phases := []phase{
		{config.T("response.timing.dns"), t.DNSMs, config.Teal},
		{config.T("response.timing.connect"), t.ConnectMs, config.Blue},
		{config.T("response.timing.tls"), t.TLSMs, config.Mauve},
		{config.T("response.timing.ttfb"), t.TTFBMs, config.Peach},
		{config.T("response.timing.download"), max(0, transfer), config.Green},
	}
	barWidth := max(10, width-34)
	var b strings.Builder
	offset := 0.0
	for _, p := range phases {
		start := int(offset / max(t.TotalMs, 0.001) * float64(barWidth))
		length := int(p.ms / max(t.TotalMs, 0.001) * float64(barWidth))
		if p.ms > 0 && length == 0 {
			length = 1
		}
		start = min(start, barWidth-length)
		bar := strings.Repeat(" ", max(0, start)) + lipgloss.NewStyle().Foreground(lipgloss.Color(p.color)).Render(strings.Repeat("█", length))
		fmt.Fprintf(&b, "%-15s %9.2f ms  %s\n", p.name, p.ms, bar)
		offset += p.ms
	}
	fmt.Fprintf(&b, "%-15s %9.2f ms\n", config.T("response.timing.total"), t.TotalMs)
	if t.Reused {
		b.WriteString("\n" + dimStyle.Render(config.T("response.timing.reused")))
	}
	return strings.TrimRight(b.String(), "\n")
}
