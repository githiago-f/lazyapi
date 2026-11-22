// Package palette is the command palette (ctrl+k): a fuzzy-searchable list
// of actions — open an operation, switch environment, run tests or flows,
// copy as cURL. Choosing one returns its message to the TUI.
package palette

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/config"
)

// Command is one palette entry.
type Command struct {
	Group string // "Open", "Run", "Environment", ...
	Title string
	Hint  string // shown dimmed on the right
	Msg   tea.Msg
}

// ChosenMsg is sent when the user picks a command (Msg is the command's).
type ChosenMsg struct{ Msg tea.Msg }

// ClosedMsg is sent when the palette is dismissed.
type ClosedMsg struct{}

var (
	boxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(config.Accent)).Background(lipgloss.Color(config.Surface0)).Padding(0, 1)
	groupStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender))
	hintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1))
	selStyle   = lipgloss.NewStyle().Background(lipgloss.Color(config.Surface2))
	matchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Peach)).Bold(true)
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay0))
)

// Model is an open palette.
type Model struct {
	input    textinput.Model
	commands []Command
	matches  []match
	cursor   int
	offset   int
	width    int
	rows     int
}

type match struct {
	idx       int
	score     int
	positions []int // matched rune positions in the searchable text
}

// New opens a palette over commands.
func New(commands []Command, width, height int) Model {
	in := textinput.New()
	in.Prompt = "› "
	in.Placeholder = "type to search operations and commands"
	in.Focus()
	m := Model{input: in, commands: commands}
	m.SetSize(width, height)
	m.filter()
	return m
}

// SetSize fits the palette in a width × height screen.
func (m *Model) SetSize(width, height int) {
	m.width = max(30, min(100, width-8))
	m.rows = max(3, min(14, height-8))
	m.input.SetWidth(m.width - 6)
}

// Init focuses the input.
func (m Model) Init() tea.Cmd { return textinput.Blink }

// Update handles keys.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "esc", "ctrl+k":
			return m, func() tea.Msg { return ClosedMsg{} }
		case "enter":
			if m.cursor < len(m.matches) {
				chosen := m.commands[m.matches[m.cursor].idx].Msg
				return m, func() tea.Msg { return ChosenMsg{Msg: chosen} }
			}
			return m, nil
		case "up", "ctrl+p", "shift+tab":
			m.move(-1)
			return m, nil
		case "down", "ctrl+n", "tab":
			m.move(1)
			return m, nil
		case "pgup":
			m.move(-m.rows)
			return m, nil
		case "pgdown":
			m.move(m.rows)
			return m, nil
		}
	}
	before := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != before {
		m.filter()
	}
	return m, cmd
}

func (m *Model) move(d int) {
	if len(m.matches) == 0 {
		return
	}
	m.cursor = max(0, min(len(m.matches)-1, m.cursor+d))
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.rows {
		m.offset = m.cursor - m.rows + 1
	}
}

func searchable(c Command) string { return c.Group + " " + c.Title }

func (m *Model) filter() {
	q := strings.TrimSpace(m.input.Value())
	m.matches = m.matches[:0]
	for i, c := range m.commands {
		if q == "" {
			m.matches = append(m.matches, match{idx: i})
			continue
		}
		if score, pos, ok := Fuzzy(q, searchable(c)); ok {
			m.matches = append(m.matches, match{idx: i, score: score, positions: pos})
		}
	}
	if q != "" {
		sort.SliceStable(m.matches, func(a, b int) bool { return m.matches[a].score > m.matches[b].score })
	}
	m.cursor, m.offset = 0, 0
}

// Fuzzy matches every word of query (space separated) as a subsequence of
// text, case-insensitively. Higher scores mean consecutive runs, word starts
// and earlier matches.
func Fuzzy(query, text string) (int, []int, bool) {
	t := []rune(strings.ToLower(text))
	orig := []rune(text)
	score := 0
	var positions []int
	used := map[int]bool{}
	for _, word := range strings.Fields(strings.ToLower(query)) {
		w := []rune(word)
		best, bestPos := -1, []int(nil)
		// Try each start so "pet" prefers "/pets" over "p…e…t".
		for start := 0; start < len(t); start++ {
			if t[start] != w[0] {
				continue
			}
			pos := []int{}
			j, s, prev := 0, 0, -2
			for i := start; i < len(t) && j < len(w); i++ {
				if t[i] != w[j] || used[i] {
					continue
				}
				pos = append(pos, i)
				switch {
				case i == prev+1:
					s += 5
				case i == 0 || !unicode.IsLetter(orig[i-1]) && !unicode.IsDigit(orig[i-1]):
					s += 3
				default:
					s++
				}
				prev = i
				j++
			}
			if j < len(w) {
				break // later starts cannot match more
			}
			s -= start / 4
			if s > best {
				best, bestPos = s, pos
			}
		}
		if best < 0 {
			return 0, nil, false
		}
		score += best
		for _, p := range bestPos {
			used[p] = true
		}
		positions = append(positions, bestPos...)
	}
	sort.Ints(positions)
	return score, positions, true
}

func highlight(text string, positions []int, offset int) string {
	if len(positions) == 0 {
		return text
	}
	set := map[int]bool{}
	for _, p := range positions {
		set[p-offset] = true
	}
	var b strings.Builder
	for i, r := range []rune(text) {
		if set[i] {
			b.WriteString(matchStyle.Render(string(r)))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// View renders the palette box (the TUI centers it over the screen).
func (m Model) View() string {
	inner := m.width - 4
	lines := []string{m.input.View(), dimStyle.Render(strings.Repeat("─", inner))}
	if len(m.matches) == 0 {
		lines = append(lines, dimStyle.Render("No matches"))
	}
	for i := m.offset; i < len(m.matches) && i < m.offset+m.rows; i++ {
		mt := m.matches[i]
		c := m.commands[mt.idx]
		groupLen := len([]rune(c.Group)) + 1
		left := groupStyle.Render(highlight(c.Group, mt.positions, 0)) + " " + highlight(c.Title, mt.positions, groupLen)
		hint := hintStyle.Render(c.Hint)
		gap := inner - lipgloss.Width(left) - lipgloss.Width(hint)
		line := left
		if gap >= 2 {
			line += strings.Repeat(" ", gap) + hint
		} else {
			line = ansi.Truncate(left, inner, "…")
		}
		if i == m.cursor {
			line = selStyle.Width(inner).Render(line)
		}
		lines = append(lines, line)
	}
	footer := "↑↓ choose · enter run · esc close"
	if n := len(m.matches); n > m.rows {
		footer += fmt.Sprintf(" · %d/%d", m.cursor+1, n)
	}
	lines = append(lines, dimStyle.Render(footer))
	return boxStyle.Width(m.width).Render(strings.Join(lines, "\n"))
}
