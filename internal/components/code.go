package components

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/evanw/esbuild/pkg/api"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/components/complete"
	"github.com/githiago-f/lazyapi/internal/components/syntax"
	"github.com/githiago-f/lazyapi/internal/config"
)

// Languages of the code editor.
const (
	LangJSON     = "json"
	LangJS       = "javascript"
	LangText     = "text"
	LangMarkdown = "markdown"
)

// CodeEditor is a multi-line editor for bodies and scripts: syntax colors,
// line numbers, auto-closed brackets and quotes, auto-indent, tab indents,
// alt+f formats JSON, live JSON validation and a completion menu
// (variables everywhere, the script API in JavaScript). The textarea holds
// the text and the cursor; this type draws it.
type CodeEditor struct {
	Lang        string
	Placeholder string

	buf      [][]rune // never empty: at least one (empty) line
	row, col int      // cursor
	wantCol  int      // column kept while moving up and down
	anchor   *textPos // selection: from anchor to the cursor (nil: none)

	width, height int // outer size, border included
	top, left     int // scroll
	focused       bool

	undo []snapshot
	redo []snapshot

	menu      *Completion
	dismissed string

	// cache of the highlighted text
	cacheValue string
	cacheLines []syntax.Line
	problem    *Problem
}

type textPos struct{ row, col int }

func (p textPos) before(o textPos) bool { return p.row < o.row || p.row == o.row && p.col < o.col }

// Clipboard is how the editor copies and pastes (replaced in tests).
var Clipboard interface {
	WriteAll(string) error
	ReadAll() (string, error)
} = SystemClipboard{}

// SystemClipboard uses the OS clipboard.
type SystemClipboard struct{}

func (SystemClipboard) WriteAll(s string) error  { return clipboard.WriteAll(s) }
func (SystemClipboard) ReadAll() (string, error) { return clipboard.ReadAll() }

type snapshot struct {
	text     string
	row, col int
}

const maxUndo = 200

// Problem is a validation error at a line (0-based).
type Problem struct {
	Line    int
	Message string
}

const indentUnit = "  "

var (
	codeBox        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(config.Surface2))
	gutterStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Surface2))
	gutterCurrent  = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender)).Bold(true)
	gutterError    = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Red)).Bold(true)
	statusStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1))
	statusOK       = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Green))
	statusBad      = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Red))
	signatureStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender))
	langBadge      = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Crust)).Background(lipgloss.Color(config.Mauve)).Bold(true).Padding(0, 1)
)

// NewCodeEditor returns an editor for lang.
func NewCodeEditor(lang, placeholder, value string) CodeEditor {
	c := CodeEditor{Lang: lang, Placeholder: placeholder, focused: true}
	c.SetValue(value)
	return c
}

// Value is the text.
func (c CodeEditor) Value() string {
	parts := make([]string, len(c.buf))
	for i, l := range c.buf {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

// SetValue replaces the text, moves the cursor to the start and forgets the
// undo history.
func (c *CodeEditor) SetValue(s string) {
	c.buf = splitRunes(s)
	c.row, c.col, c.wantCol = 0, 0, 0
	c.top, c.left, c.menu = 0, 0, nil
	c.undo, c.redo = nil, nil
	c.anchor = nil
}

// selection returns the selected range in order (ok false when empty).
func (c CodeEditor) selection() (from, to textPos, ok bool) {
	if c.anchor == nil {
		return textPos{}, textPos{}, false
	}
	a, b := *c.anchor, textPos{c.row, c.col}
	if b.before(a) {
		a, b = b, a
	}
	return a, b, a != b
}

// HasSelection reports whether text is selected (ctrl+c then copies it
// instead of quitting).
func (c CodeEditor) HasSelection() bool {
	_, _, ok := c.selection()
	return ok
}

// SelectedText returns the selected text.
func (c CodeEditor) SelectedText() string {
	from, to, ok := c.selection()
	if !ok {
		return ""
	}
	if from.row == to.row {
		return string(c.buf[from.row][from.col:to.col])
	}
	parts := []string{string(c.buf[from.row][from.col:])}
	for r := from.row + 1; r < to.row; r++ {
		parts = append(parts, string(c.buf[r]))
	}
	parts = append(parts, string(c.buf[to.row][:to.col]))
	return strings.Join(parts, "\n")
}

// deleteSelection removes the selected text (no checkpoint).
func (c *CodeEditor) deleteSelection() bool {
	from, to, ok := c.selection()
	c.anchor = nil
	if !ok {
		return false
	}
	head := append([]rune{}, c.buf[from.row][:from.col]...)
	tail := c.buf[to.row][to.col:]
	c.buf[from.row] = append(head, tail...)
	c.buf = append(c.buf[:from.row+1], c.buf[to.row+1:]...)
	c.row, c.col = from.row, from.col
	return true
}

// shiftLines indents (+1) or dedents (-1) the lines of the selection.
func (c *CodeEditor) shiftLines(dir int) {
	from, to, _ := c.selection()
	last := to.row
	if to.col == 0 && to.row > from.row {
		last-- // a selection ending at column 0 does not include that line
	}
	for r := from.row; r <= last; r++ {
		if dir > 0 {
			c.buf[r] = append([]rune(indentUnit), c.buf[r]...)
			continue
		}
		n := min(len(indentUnit), len(leadingSpaces(c.buf[r])))
		c.buf[r] = c.buf[r][n:]
	}
	shift := func(p *textPos) {
		if p.row >= from.row && p.row <= last {
			if dir > 0 {
				p.col += len(indentUnit)
			} else {
				p.col = max(0, p.col-len(indentUnit))
			}
		}
	}
	cur := textPos{c.row, c.col}
	shift(&cur)
	shift(c.anchor)
	c.row, c.col = cur.row, cur.col
	c.clampCursor()
}

func (c *CodeEditor) Focus() { c.focused = true }
func (c *CodeEditor) Blur()  { c.focused = false; c.menu = nil }

// SetSize sets the outer size (border included).
func (c *CodeEditor) SetSize(w, h int) { c.width, c.height = w, h }

// Completing reports whether the editor needs esc/tab/arrows itself: a
// completion menu is open or text is selected (esc clears the selection,
// tab indents it).
func (c CodeEditor) Completing() bool { return c.menu != nil || c.HasSelection() }

// Problem returns the current validation error, if any.
func (c *CodeEditor) Problem() *Problem {
	c.highlight()
	return c.problem
}

func (c CodeEditor) Init() tea.Cmd { return nil }

// checkpoint records the text for ctrl+z before a change.
func (c *CodeEditor) checkpoint() {
	c.undo = append(c.undo, snapshot{c.Value(), c.row, c.col})
	if len(c.undo) > maxUndo {
		c.undo = c.undo[1:]
	}
	c.redo = nil
}

func (c *CodeEditor) restore(from, to *[]snapshot) {
	if len(*from) == 0 {
		return
	}
	last := (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	*to = append(*to, snapshot{c.Value(), c.row, c.col})
	c.buf = splitRunes(last.text)
	c.row, c.col = last.row, last.col
	c.clampCursor()
}

func (c *CodeEditor) clampCursor() {
	c.row = max(0, min(c.row, len(c.buf)-1))
	c.col = max(0, min(c.col, len(c.buf[c.row])))
}

// insert puts text at the cursor (it may contain newlines).
func (c *CodeEditor) insert(text string) {
	parts := splitRunes(text)
	line := c.buf[c.row]
	head, tail := append([]rune{}, line[:c.col]...), append([]rune{}, line[c.col:]...)
	if len(parts) == 1 {
		c.buf[c.row] = append(append(head, parts[0]...), tail...)
		c.col += len(parts[0])
		return
	}
	newLines := make([][]rune, 0, len(parts))
	newLines = append(newLines, append(head, parts[0]...))
	newLines = append(newLines, parts[1:len(parts)-1]...)
	last := parts[len(parts)-1]
	newLines = append(newLines, append(append([]rune{}, last...), tail...))
	c.buf = append(c.buf[:c.row], append(newLines, c.buf[c.row+1:]...)...)
	c.row += len(parts) - 1
	c.col = len(last)
}

// deleteBack removes n runes before the cursor (joining lines at col 0).
func (c *CodeEditor) deleteBack() {
	switch {
	case c.col > 0:
		line := c.buf[c.row]
		c.buf[c.row] = append(append([]rune{}, line[:c.col-1]...), line[c.col:]...)
		c.col--
	case c.row > 0:
		prev := c.buf[c.row-1]
		c.col = len(prev)
		c.buf[c.row-1] = append(append([]rune{}, prev...), c.buf[c.row]...)
		c.buf = append(c.buf[:c.row], c.buf[c.row+1:]...)
		c.row--
	}
}

func (c *CodeEditor) deleteForward() {
	line := c.buf[c.row]
	switch {
	case c.col < len(line):
		c.buf[c.row] = append(append([]rune{}, line[:c.col]...), line[c.col+1:]...)
	case c.row < len(c.buf)-1:
		c.buf[c.row] = append(append([]rune{}, line...), c.buf[c.row+1]...)
		c.buf = append(c.buf[:c.row+1], c.buf[c.row+2:]...)
	}
}

// wordLeft/wordRight find the next word boundary on the current line.
func (c CodeEditor) wordLeft() int {
	l, i := c.buf[c.row], c.col
	for i > 0 && !isWordRune(l[i-1]) {
		i--
	}
	for i > 0 && isWordRune(l[i-1]) {
		i--
	}
	return i
}

func (c CodeEditor) wordRight() int {
	l, i := c.buf[c.row], c.col
	for i < len(l) && !isWordRune(l[i]) {
		i++
	}
	for i < len(l) && isWordRune(l[i]) {
		i++
	}
	return i
}

// move handles cursor keys; false if k is not one.
func (c *CodeEditor) move(k string) bool {
	vertical := false
	switch k {
	case "left", "ctrl+b":
		if c.col > 0 {
			c.col--
		} else if c.row > 0 {
			c.row--
			c.col = len(c.buf[c.row])
		}
	case "right", "ctrl+f":
		if c.col < len(c.buf[c.row]) {
			c.col++
		} else if c.row < len(c.buf)-1 {
			c.row++
			c.col = 0
		}
	case "up":
		c.row--
		vertical = true
	case "down":
		c.row++
		vertical = true
	case "pgup":
		c.row -= c.textRows()
		vertical = true
	case "pgdown":
		c.row += c.textRows()
		vertical = true
	case "home", "ctrl+a":
		// first press: after the indentation; second: column 0
		if ind := len(leadingSpaces(c.buf[c.row])); c.col != ind {
			c.col = ind
		} else {
			c.col = 0
		}
	case "end", "ctrl+e":
		c.col = len(c.buf[c.row])
	case "ctrl+home":
		c.row, c.col = 0, 0
	case "ctrl+end":
		c.row = len(c.buf) - 1
		c.col = len(c.buf[c.row])
	case "alt+left", "ctrl+left", "alt+b":
		c.col = c.wordLeft()
	case "alt+right", "ctrl+right":
		c.col = c.wordRight()
	default:
		return false
	}
	if vertical {
		c.row = max(0, min(c.row, len(c.buf)-1))
		c.col = min(c.wantCol, len(c.buf[c.row]))
	} else {
		c.clampCursor()
		c.wantCol = c.col
	}
	return true
}

var pairs = map[rune]rune{'{': '}', '[': ']', '(': ')', '"': '"', '\'': '\'', '`': '`'}

func isClosing(r rune) bool {
	return r == '}' || r == ']' || r == ')' || r == '"' || r == '\'' || r == '`'
}

func isWordRune(r rune) bool {
	return r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func leadingSpaces(l []rune) string {
	i := 0
	for i < len(l) && (l[i] == ' ' || l[i] == '\t') {
		i++
	}
	return string(l[:i])
}

// selectionKey handles selecting (shift+movement, alt+a), the clipboard
// (ctrl+c copy, alt+x cut, alt+v paste) and keys that act on a selection.
func (c *CodeEditor) selectionKey(k tea.KeyPressMsg) (bool, tea.Cmd) {
	s := k.String()
	if strings.Contains(s, "shift+") && s != "shift+tab" {
		base := strings.Replace(s, "shift+", "", 1)
		if c.anchor == nil {
			c.anchor = &textPos{c.row, c.col}
		}
		if c.move(base) {
			return true, nil
		}
		if !c.HasSelection() {
			c.anchor = nil
		}
		return false, nil
	}
	switch s {
	case "alt+a":
		c.anchor = &textPos{0, 0}
		c.row = len(c.buf) - 1
		c.col = len(c.buf[c.row])
		return true, nil
	case "alt+v":
		text, err := Clipboard.ReadAll()
		if err != nil {
			return true, notifyCmd("Clipboard unavailable: "+err.Error(), Error)
		}
		c.checkpoint()
		c.deleteSelection()
		c.insert(strings.ReplaceAll(text, "\r\n", "\n"))
		return true, nil
	}
	if !c.HasSelection() {
		return false, nil
	}
	switch s {
	case "ctrl+c", "alt+x":
		if err := Clipboard.WriteAll(c.SelectedText()); err != nil {
			return true, notifyCmd("Clipboard unavailable: "+err.Error(), Error)
		}
		if s == "alt+x" {
			c.checkpoint()
			c.deleteSelection()
			return true, notifyCmd("✓ Cut", Success)
		}
		return true, notifyCmd("✓ Copied", Success)
	case "esc":
		c.anchor = nil
		return true, nil
	case "tab", "shift+tab":
		c.checkpoint()
		if s == "tab" {
			c.shiftLines(1)
		} else {
			c.shiftLines(-1)
		}
		return true, nil
	case "backspace", "ctrl+h", "delete", "ctrl+d":
		c.checkpoint()
		c.deleteSelection()
		return true, nil
	}
	// Typing replaces the selection; an opening bracket or quote wraps it.
	if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
		c.checkpoint()
		r := []rune(k.Text)
		if closer, ok := pairs[r[0]]; ok && len(r) == 1 {
			text := c.SelectedText()
			c.deleteSelection()
			c.insert(string(r[0]) + text + string(closer))
			return true, nil
		}
		c.deleteSelection()
		c.insert(k.Text)
		return true, nil
	}
	if s == "enter" {
		c.checkpoint()
		c.deleteSelection()
		return false, nil // the newline itself is a normal edit
	}
	return false, nil
}

func notifyCmd(msg string, kind NotificationType) tea.Cmd {
	return func() tea.Msg { return ShowNotificationMsg{Message: msg, Type: kind} }
}

// edit applies an editing key; false if k is not one.
func (c *CodeEditor) edit(k tea.KeyPressMsg) bool {
	line := c.buf[c.row]
	prev, next := rune(0), rune(0)
	if c.col > 0 {
		prev = line[c.col-1]
	}
	if c.col < len(line) {
		next = line[c.col]
	}

	switch s := k.String(); s {
	case "ctrl+z":
		c.restore(&c.undo, &c.redo)
		return true
	case "ctrl+y", "ctrl+shift+z":
		c.restore(&c.redo, &c.undo)
		return true
	case "alt+f":
		c.Format()
		return true
	case "tab":
		c.checkpoint()
		c.insert(indentUnit)
		return true
	case "shift+tab":
		if n := min(len(indentUnit), len(leadingSpaces(line))); n > 0 {
			c.checkpoint()
			c.buf[c.row] = line[n:]
			c.col = max(0, c.col-n)
		}
		return true
	case "enter":
		c.checkpoint()
		indent := leadingSpaces(line)
		if closer, ok := pairs[prev]; ok && (prev == '{' || prev == '[' || prev == '(') {
			if next == closer { // {|} → {\n  |\n}
				c.insert("\n" + indent + indentUnit + "\n" + indent)
				c.row--
				c.col = len(indent) + len(indentUnit)
			} else {
				c.insert("\n" + indent + indentUnit)
			}
			return true
		}
		c.insert("\n" + indent)
		return true
	case "backspace", "ctrl+h":
		c.checkpoint()
		if prev != 0 && next != 0 && pairs[prev] == next {
			c.deleteForward()
		}
		c.deleteBack()
		return true
	case "delete", "ctrl+d":
		c.checkpoint()
		c.deleteForward()
		return true
	case "alt+backspace", "ctrl+w":
		c.checkpoint()
		to := c.wordLeft()
		c.buf[c.row] = append(append([]rune{}, line[:to]...), line[c.col:]...)
		c.col = to
		return true
	case "ctrl+u":
		c.checkpoint()
		c.buf[c.row] = line[c.col:]
		c.col = 0
		return true
	}

	text := k.Text
	if text == "" || k.Mod&(tea.ModCtrl|tea.ModAlt) != 0 {
		return false
	}
	c.checkpoint()
	if r := []rune(text); len(r) == 1 {
		ch := r[0]
		// Typing a closer that is already there moves over it.
		if isClosing(ch) && next == ch {
			c.col++
			return true
		}
		if closer, ok := pairs[ch]; ok {
			quote := ch == '"' || ch == '\'' || ch == '`'
			// No pair next to a word: quotes inside words (don't), brackets before one.
			nextToWord := isWordRune(next) || quote && isWordRune(prev)
			if !nextToWord {
				c.insert(string(ch) + string(closer))
				c.col--
				return true
			}
		}
	}
	c.insert(text)
	return true
}

// Format re-indents JSON (keys keep their order). Other languages and
// invalid JSON are left alone.
func (c *CodeEditor) Format() bool {
	if c.Lang != LangJSON {
		return false
	}
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(strings.TrimSpace(c.Value())), "", indentUnit); err != nil {
		return false
	}
	c.checkpoint()
	c.buf = splitRunes(out.String())
	c.row, c.col, c.top, c.left = 0, 0, 0, 0
	return true
}

func (c CodeEditor) Update(msg tea.Msg) (CodeEditor, tea.Cmd) {
	if !c.focused {
		return c, nil
	}
	if paste, ok := msg.(tea.PasteMsg); ok {
		c.checkpoint()
		c.insert(strings.ReplaceAll(paste.Content, "\r\n", "\n"))
		c.after(true)
		return c, nil
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return c, nil
	}
	if c.menu != nil {
		switch k.String() {
		case "up", "ctrl+p":
			c.menu.Move(-1)
			return c, nil
		case "down", "ctrl+n":
			c.menu.Move(1)
			return c, nil
		case "tab", "enter":
			c.checkpoint()
			text, cur := c.menu.Apply([]rune(c.Value()))
			c.menu = nil
			c.buf = splitRunes(string(text))
			c.row, c.col = position(c.buf, cur)
			c.after(true)
			return c, nil
		case "esc":
			c.menu = nil
			c.dismissed = c.Value()
			return c, nil
		}
	}
	before := c.Value()
	if handled, cmd := c.selectionKey(k); handled {
		c.after(c.Value() != before)
		return c, cmd
	}
	if c.move(k.String()) {
		c.anchor = nil
		c.after(false)
		return c, nil
	}
	if c.edit(k) {
		if len(c.undo) > 0 && c.undo[len(c.undo)-1].text == c.Value() {
			c.undo = c.undo[:len(c.undo)-1] // nothing changed
		}
		c.wantCol = c.col
		c.after(c.Value() != before)
	}
	return c, nil
}

func splitRunes(s string) [][]rune {
	parts := strings.Split(s, "\n")
	out := make([][]rune, len(parts))
	for i, p := range parts {
		out[i] = []rune(p)
	}
	return out
}

// after keeps the cursor on screen and refreshes the menu.
func (c *CodeEditor) after(changed bool) {
	if changed {
		c.dismissed = ""
	}
	rows, cols := c.textRows(), c.textCols()
	if c.row < c.top {
		c.top = c.row
	}
	if c.row >= c.top+rows {
		c.top = c.row - rows + 1
	}
	if c.col < c.left {
		c.left = c.col
	}
	if c.col >= c.left+cols {
		c.left = c.col - cols + 1
	}
	c.menu = nil
	if !changed || c.dismissed != "" && c.dismissed == c.Value() {
		return
	}
	text := []rune(c.Value())
	cur := offset(c.buf, c.row, c.col)
	if res, ok := complete.Templates(text, cur, templateVars()); ok {
		c.menu = &Completion{Result: res}
		return
	}
	if c.Lang == LangJS {
		if res, ok := complete.Script(text, cur); ok {
			c.menu = &Completion{Result: res}
		}
	}
}

func (c CodeEditor) gutterWidth() int {
	n := len(c.buf)
	return max(2, len(fmt.Sprint(n))) + 2 // digits, marker, space
}

func (c CodeEditor) textRows() int { return max(1, c.height-2-1) } // border, status line
func (c CodeEditor) textCols() int { return max(1, c.width-2-c.gutterWidth()) }

var templateRef = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// highlight caches the colored lines and the JSON problem.
func (c *CodeEditor) highlight() []syntax.Line {
	v := c.Value()
	if c.cacheLines != nil && v == c.cacheValue {
		return c.cacheLines
	}
	var lines []syntax.Line
	if c.Lang == LangText {
		lines = syntax.PlainLines(v)
	} else {
		lines = syntax.Code(c.Lang, v)
	}
	syntax.MarkTemplates(lines, Resolver())
	c.cacheValue, c.cacheLines = v, lines
	c.problem = nil
	switch {
	case strings.TrimSpace(v) == "":
	case c.Lang == LangJSON:
		c.problem = jsonProblem(v)
	case c.Lang == LangJS:
		c.problem = scriptProblem(v)
	}
	return lines
}

// scriptProblem is the first syntax error esbuild reports for a script
// (JavaScript or TypeScript; imports are not resolved here).
func scriptProblem(v string) *Problem {
	res := api.Transform(v, api.TransformOptions{Loader: api.LoaderTS, Format: api.FormatESModule, LogLevel: api.LogLevelSilent})
	if len(res.Errors) == 0 {
		return nil
	}
	e := res.Errors[0]
	line := 0
	if e.Location != nil {
		line = e.Location.Line - 1
	}
	return &Problem{Line: line, Message: e.Text}
}

// jsonProblem validates JSON, treating {{templates}} as values.
func jsonProblem(v string) *Problem {
	probe := templateRef.ReplaceAllString(v, "0")
	var x any
	err := json.Unmarshal([]byte(probe), &x)
	if err == nil {
		return nil
	}
	var se *json.SyntaxError
	if errors.As(err, &se) {
		line := strings.Count(probe[:min(int(se.Offset), len(probe))], "\n")
		return &Problem{Line: line, Message: se.Error()}
	}
	return &Problem{Line: 0, Message: err.Error()}
}

func (c CodeEditor) View() tea.View {
	lines := c.highlight()
	row, col := c.row, c.col
	rows, cols := c.textRows(), c.textCols()
	gw := c.gutterWidth()
	top := max(0, min(c.top, max(0, len(lines)-1)))

	var out []string
	if c.Value() == "" && c.Placeholder != "" {
		cur := ""
		if c.focused {
			cur = syntax.Render(syntax.Line{}, 0, 1, 0)
		}
		out = append(out, gutterCurrent.Render(fmt.Sprintf("%*d ", gw-1, 1))+" "+cur+placeholderStyle.Render(c.Placeholder))
	} else {
		for i := top; i < top+rows && i < len(lines); i++ {
			num := fmt.Sprintf("%*d", gw-2, i+1)
			mark := " "
			gs := gutterStyle
			if c.problem != nil && c.problem.Line == i {
				mark, gs = "●", gutterError
			} else if i == row && c.focused {
				gs = gutterCurrent
			}
			cur := -1
			if i == row && c.focused {
				cur = col
			}
			selStart, selEnd := -1, -1
			if from, to, ok := c.selection(); ok && i >= from.row && i <= to.row {
				selStart, selEnd = 0, lines[i].Len()+1
				if i == from.row {
					selStart = from.col
				}
				if i == to.row {
					selEnd = to.col
				}
			}
			out = append(out, gs.Render(num+mark)+" "+syntax.RenderSelection(lines[i], c.left, cols, cur, selStart, selEnd))
		}
	}
	for len(out) < rows {
		out = append(out, gutterStyle.Render(strings.Repeat(" ", gw-2)+"~"))
	}
	if c.menu != nil && c.focused {
		// text starts at column gw; "❯ " and the border sit left of the item
		out = overlayMenu(out, c.menu.View(min(56, c.width-2)), row-top+1, gw+max(0, col-c.left)-3, c.width-2)
	}
	out = append(out, c.statusLine(len(lines), row, col))
	box := codeBox
	if c.focused {
		box = box.BorderForeground(lipgloss.Color(config.Mauve))
	}
	box = box.Width(max(4, c.width)).Height(max(3, c.height))
	return tea.NewView(box.Render(strings.Join(out, "\n")))
}

func (c CodeEditor) statusLine(n, row, col int) string {
	lang := map[string]string{LangJSON: "JSON", LangJS: "JS", LangText: "TEXT", LangMarkdown: "MD"}[c.Lang]
	if lang == "" {
		lang = strings.ToUpper(c.Lang)
	}
	parts := []string{langBadge.Render(lang), statusStyle.Render(fmt.Sprintf("Ln %d, Col %d · %d lines", row+1, col+1, n))}
	if c.Lang == LangJS && c.focused {
		if sig := complete.Signature([]rune(c.Value()), offset(c.buf, row, col)); sig != "" {
			// What the cursor is typing arguments for beats the rest.
			return ansi.Truncate(strings.Join(append(parts, signatureStyle.Render("ƒ "+sig)), " "), max(1, c.width-2), "…")
		}
	}
	switch {
	case c.problem != nil:
		parts = append(parts, statusBad.Render(fmt.Sprintf("✗ %d: %s", c.problem.Line+1, c.problem.Message)))
	case c.Lang == LangJSON && strings.TrimSpace(c.Value()) != "":
		parts = append(parts, statusOK.Render("✓ valid")+statusStyle.Render(" · alt+f format"))
	case c.Lang == LangJS && strings.TrimSpace(c.Value()) != "":
		parts = append(parts, statusOK.Render("✓ syntax ok")+statusStyle.Render(" · F5 run"))
	}
	return ansi.Truncate(strings.Join(parts, " "), max(1, c.width-2), "…")
}

// overlayMenu draws menu over lines starting at (row, col).
func overlayMenu(lines []string, menu string, row, col, width int) []string {
	mlines := strings.Split(menu, "\n")
	mw := lipgloss.Width(menu)
	col = max(0, min(col, width-mw))
	if row+len(mlines) > len(lines) { // not enough room below: open above
		row = max(0, row-len(mlines)-1)
	}
	for i, ml := range mlines {
		r := row + i
		if r >= len(lines) {
			break
		}
		base := lines[r]
		left := ansi.Truncate(base, col, "")
		if pad := col - lipgloss.Width(left); pad > 0 {
			left += strings.Repeat(" ", pad)
		}
		right := ansi.TruncateLeft(base, col+mw, "")
		lines[r] = left + ml + right
	}
	return lines
}

// offset converts (row, col) into an index of the whole text in runes.
func offset(lines [][]rune, row, col int) int {
	n := 0
	for i := 0; i < row && i < len(lines); i++ {
		n += len(lines[i]) + 1
	}
	return n + col
}

// position converts a text index back into (row, col).
func position(lines [][]rune, off int) (int, int) {
	for i, l := range lines {
		if off <= len(l) {
			return i, off
		}
		off -= len(l) + 1
	}
	last := len(lines) - 1
	return last, len(lines[last])
}
