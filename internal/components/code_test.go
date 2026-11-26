package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/components/complete"
)

func typeIn(c CodeEditor, keys ...any) CodeEditor {
	for _, k := range keys {
		switch v := k.(type) {
		case string:
			for _, r := range v {
				c, _ = c.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
		case tea.KeyPressMsg:
			c, _ = c.Update(v)
		}
	}
	return c
}

var (
	enter     = tea.KeyPressMsg{Code: tea.KeyEnter}
	tab       = tea.KeyPressMsg{Code: tea.KeyTab}
	shiftTab  = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	backspace = tea.KeyPressMsg{Code: tea.KeyBackspace}
	altF      = tea.KeyPressMsg{Code: 'f', Mod: tea.ModAlt}
)

func TestCodeEditorPairsAndIndent(t *testing.T) {
	c := NewCodeEditor(LangJSON, "", "")
	c.SetSize(60, 12)
	c = typeIn(c, "{", enter, "\"a", tab)
	// { → {}, enter between braces opens an indented line, " → "".
	if got := c.Value(); got != "{\n  \"a  \"\n}" {
		t.Errorf("after typing: %q", got)
	}
	c = NewCodeEditor(LangJSON, "", "")
	c.SetSize(60, 12)
	c = typeIn(c, "[1", "]") // typing the closer moves over it
	if c.Value() != "[1]" {
		t.Errorf("closer skip: %q", c.Value())
	}
	c = NewCodeEditor(LangJSON, "", "")
	c = typeIn(c, "(", backspace)
	if c.Value() != "" {
		t.Errorf("backspace removes the pair: %q", c.Value())
	}
	c = NewCodeEditor(LangJS, "", "  x")
	c = typeIn(c, shiftTab)
	if c.Value() != "x" {
		t.Errorf("shift+tab dedents: %q", c.Value())
	}
	c = NewCodeEditor(LangJS, "", "")
	c = typeIn(c, "don", "'", "t") // no pair inside a word
	if c.Value() != "don't" {
		t.Errorf("quote in a word: %q", c.Value())
	}
}

func TestCodeEditorFormatAndValidate(t *testing.T) {
	c := NewCodeEditor(LangJSON, "", `{"b":1,"a":[1,2]}`)
	c.SetSize(60, 12)
	c = typeIn(c, altF)
	if c.Value() != "{\n  \"b\": 1,\n  \"a\": [\n    1,\n    2\n  ]\n}" {
		t.Errorf("format: %q", c.Value())
	}
	if c.Problem() != nil {
		t.Errorf("valid JSON: %+v", c.Problem())
	}
	c.SetValue("{\n  \"a\": 1,\n  \"b\": \n}")
	if p := c.Problem(); p == nil || p.Line != 3 {
		t.Errorf("problem = %+v", p)
	}
	c.SetValue(`{"token": {{env.TOKEN}}, "name": "{{env.NAME}}"}`)
	if p := c.Problem(); p != nil {
		t.Errorf("templates count as values: %+v", p)
	}
	out := ansi.Strip(c.View().Content)
	if !strings.Contains(out, "JSON") || !strings.Contains(out, "✓ valid") || !strings.Contains(out, " 1 ") {
		t.Errorf("view:\n%s", out)
	}
}

func TestCodeEditorManyLines(t *testing.T) {
	big := strings.Repeat("x\n", 300) + "end"
	c := NewCodeEditor(LangText, "", big)
	if c.Value() != big {
		t.Fatalf("lost lines: %d of 301", strings.Count(c.Value(), "\n")+1)
	}
	c.SetSize(40, 10)
	c = typeIn(c, tea.KeyPressMsg{Code: tea.KeyDown})
	for i := 0; i < 299; i++ {
		c = typeIn(c, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if out := ansi.Strip(c.View().Content); !strings.Contains(out, "301") || !strings.Contains(out, "end") {
		t.Errorf("scrolling to the last line:\n%s", out)
	}
}

type fakeVars struct{}

func (fakeVars) Vars() []complete.Var { return []complete.Var{{Ref: "env.TOKEN", Value: "x"}} }
func (fakeVars) Has(ref string) bool  { return ref == "env.TOKEN" }

func TestCodeEditorCompletions(t *testing.T) {
	TemplateSource = fakeVars{}
	defer func() { TemplateSource = nil }()

	c := NewCodeEditor(LangJS, "", "")
	c.SetSize(70, 14)
	c = typeIn(c, "expect(response.status).toBe")
	if !c.Completing() {
		t.Fatal("matcher completion expected")
	}
	c = typeIn(c, tab)
	if c.Value() != "expect(response.status).toBe()" {
		t.Errorf("accept: %q", c.Value())
	}
	c = typeIn(c, "200")
	if c.Value() != "expect(response.status).toBe(200)" {
		t.Errorf("cursor inside (): %q", c.Value())
	}

	j := NewCodeEditor(LangJSON, "", "")
	j.SetSize(70, 14)
	j = typeIn(j, "{", "\"t\": \"$TO")
	if !j.Completing() || !strings.Contains(ansi.Strip(j.View().Content), "env.TOKEN") {
		t.Fatalf("variable menu:\n%s", ansi.Strip(j.View().Content))
	}
	j = typeIn(j, tab)
	if j.Value() != `{"t": "{{env.TOKEN}}"}` {
		t.Errorf("$ shortcut: %q", j.Value())
	}
	j = typeIn(j, "$TO", tea.KeyPressMsg{Code: tea.KeyEscape})
	if j.Completing() {
		t.Error("esc closes the menu")
	}
}

func TestCodeEditorUndoPasteAndMoves(t *testing.T) {
	c := NewCodeEditor(LangText, "", "")
	c.SetSize(60, 12)
	c = typeIn(c, "abc")
	c, _ = c.Update(tea.PasteMsg{Content: "1\r\n2\n3"})
	if c.Value() != "abc1\n2\n3" || c.row != 2 || c.col != 1 {
		t.Fatalf("paste: %q at %d,%d", c.Value(), c.row, c.col)
	}
	c = typeIn(c, tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
	if c.Value() != "abc" {
		t.Errorf("undo paste: %q", c.Value())
	}
	c = typeIn(c, tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
	if c.Value() != "abc1\n2\n3" {
		t.Errorf("redo: %q", c.Value())
	}
	// up keeps the wanted column; home goes to the indentation first.
	c.SetValue("    long line\nx\n    other line")
	c = typeIn(c, tea.KeyPressMsg{Code: tea.KeyEnd}, tea.KeyPressMsg{Code: tea.KeyDown}, tea.KeyPressMsg{Code: tea.KeyDown})
	if c.row != 2 || c.col != 13 {
		t.Errorf("wanted column: %d,%d", c.row, c.col)
	}
	c = typeIn(c, tea.KeyPressMsg{Code: tea.KeyHome})
	if c.col != 4 {
		t.Errorf("home → indentation, got %d", c.col)
	}
	c = typeIn(c, tea.KeyPressMsg{Code: tea.KeyHome})
	if c.col != 0 {
		t.Errorf("home twice → 0, got %d", c.col)
	}
	c = typeIn(c, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if c.Value() != "    long line\nx    other line" {
		t.Errorf("backspace joins lines: %q", c.Value())
	}
}

func TestCodeEditorTypingThroughMenus(t *testing.T) {
	c := NewCodeEditor(LangJS, "", "")
	c.SetSize(80, 14)
	src := `test("ok", () => expect(response.json().id).toBe(1));`
	for _, r := range src {
		c, _ = c.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if c.Value() != src {
		t.Errorf("typed %q\n got  %q", src, c.Value())
	}
}

type memClipboard struct{ text string }

func (m *memClipboard) WriteAll(s string) error  { m.text = s; return nil }
func (m *memClipboard) ReadAll() (string, error) { return m.text, nil }

func k(s string) tea.KeyPressMsg {
	m := tea.KeyPressMsg{}
	for {
		switch {
		case strings.HasPrefix(s, "shift+"):
			m.Mod |= tea.ModShift
			s = s[6:]
			continue
		case strings.HasPrefix(s, "alt+"):
			m.Mod |= tea.ModAlt
			s = s[4:]
			continue
		case strings.HasPrefix(s, "ctrl+"):
			m.Mod |= tea.ModCtrl
			s = s[5:]
			continue
		}
		break
	}
	switch s {
	case "left":
		m.Code = tea.KeyLeft
	case "right":
		m.Code = tea.KeyRight
	case "down":
		m.Code = tea.KeyDown
	case "end":
		m.Code = tea.KeyEnd
	case "home":
		m.Code = tea.KeyHome
	case "tab":
		m.Code = tea.KeyTab
	case "esc":
		m.Code = tea.KeyEscape
	case "backspace":
		m.Code = tea.KeyBackspace
	default:
		m.Code = []rune(s)[0]
	}
	return m
}

func TestCodeEditorSelection(t *testing.T) {
	clip := &memClipboard{}
	Clipboard = clip
	defer func() { Clipboard = SystemClipboard{} }()

	c := NewCodeEditor(LangText, "", "hello world\nsecond line")
	c.SetSize(60, 12)
	c = typeIn(c, k("shift+end"))
	if c.SelectedText() != "hello world" {
		t.Fatalf("shift+end: %q", c.SelectedText())
	}
	c = typeIn(c, k("shift+left"), k("shift+left"))
	if c.SelectedText() != "hello wor" {
		t.Errorf("shrink: %q", c.SelectedText())
	}
	c, cmd := c.Update(k("ctrl+c"))
	if cmd == nil {
		t.Fatal("ctrl+c returns the copy")
	}
	if msg, ok := cmd().(ShowNotificationMsg); !ok || clip.text != "hello wor" {
		t.Errorf("copy: %q (%v)", clip.text, msg)
	}
	if clip.text != "hello wor" {
		t.Errorf("copy: %q", clip.text)
	}
	if !c.HasSelection() {
		t.Error("copy keeps the selection")
	}
	c = typeIn(c, "X")
	if c.Value() != "Xld\nsecond line" || c.HasSelection() {
		t.Errorf("typing replaces: %q", c.Value())
	}

	// A quote wraps the selection; esc clears it.
	c.SetValue("abc")
	c = typeIn(c, k("alt+a"), "\"")
	if c.Value() != "\"abc\"" {
		t.Errorf("wrap: %q", c.Value())
	}
	c = typeIn(c, k("shift+left"), k("esc"))
	if c.HasSelection() {
		t.Error("esc clears the selection")
	}

	// Multi-line cut and paste.
	c.SetValue("one\ntwo\nthree")
	c = typeIn(c, k("shift+down"), k("shift+end"))
	c, cmd = c.Update(k("alt+x"))
	cmd()
	if c.Value() != "\nthree" || clip.text != "one\ntwo" {
		t.Errorf("cut: %q / %q", c.Value(), clip.text)
	}
	for _, paste := range []string{"alt+v", "ctrl+v"} {
		_, cmd = c.Update(k(paste))
		msg, ok := cmd().(tea.PasteMsg)
		if !ok || msg.Content != "one\ntwo" {
			t.Fatalf("%s reads the clipboard as a paste: %#v", paste, msg)
		}
	}
	c, _ = c.Update(tea.PasteMsg{Content: "one\ntwo"})
	if c.Value() != "one\ntwo\nthree" {
		t.Errorf("paste: %q", c.Value())
	}
	// A paste replaces the selection.
	c = typeIn(c, k("alt+a"))
	c, _ = c.Update(tea.PasteMsg{Content: "new"})
	if c.Value() != "new" {
		t.Errorf("paste over a selection: %q", c.Value())
	}

	// tab / shift+tab indent the selected lines.
	c.SetValue("a\nb\nc")
	c = typeIn(c, k("shift+down"), k("shift+end"), k("tab"))
	if c.Value() != "  a\n  b\nc" || !c.HasSelection() {
		t.Errorf("indent: %q", c.Value())
	}
	c = typeIn(c, k("shift+tab"))
	if c.Value() != "a\nb\nc" {
		t.Errorf("dedent: %q", c.Value())
	}
	c = typeIn(c, k("backspace"))
	if c.Value() != "\nc" { // "a\nb" was selected; the last line break stays
		t.Errorf("backspace deletes the selection: %q", c.Value())
	}
	// A plain movement drops the selection.
	c.SetValue("xy")
	c = typeIn(c, k("shift+right"))
	if !strings.Contains(c.View().Content, "48;2;88;91;112") {
		t.Error("selected text has the selection background")
	}
	c = typeIn(c, k("right"))
	if c.HasSelection() {
		t.Error("moving without shift clears the selection")
	}
}

func TestCodeEditorScriptDiagnostics(t *testing.T) {
	c := NewCodeEditor(LangJS, "", "test(\"a\", () => {\n  expect(1).toBe(1);\n}")
	c.SetSize(80, 12)
	p := c.Problem()
	if p == nil || p.Line != 2 {
		t.Fatalf("missing ) → %+v", p)
	}
	c.SetValue("const x: number = 1;\ntest(\"a\", () => expect(x).toBe(1));")
	if p := c.Problem(); p != nil {
		t.Errorf("TypeScript is valid: %+v", p)
	}
	if out := ansi.Strip(c.View().Content); !strings.Contains(out, "✓ syntax ok") {
		t.Errorf("status:\n%s", out)
	}
	c.SetValue("")
	c = typeIn(c, "lazyapi.send(")
	if out := ansi.Strip(c.View().Content); !strings.Contains(out, "ƒ lazyapi.send(target") {
		t.Errorf("signature:\n%s", out)
	}
}

func TestCodeEditorMouse(t *testing.T) {
	Z.Clear()
	c := NewCodeEditor(LangText, "", "first line\nsecond line\nthird")
	c.SetSize(40, 8)
	Z.Scan(c.View().Content)
	z := Z.Get(c.zone)
	gw := c.gutterWidth()
	// Click on "second" (row 1, col 3), then drag to row 2 col 2.
	c, _ = c.Update(tea.MouseClickMsg{X: z.StartX + 1 + gw + 3, Y: z.StartY + 2, Button: tea.MouseLeft})
	if c.row != 1 || c.col != 3 {
		t.Fatalf("click: %d,%d", c.row, c.col)
	}
	c, _ = c.Update(tea.MouseMotionMsg{X: z.StartX + 1 + gw + 2, Y: z.StartY + 3, Button: tea.MouseLeft})
	c, _ = c.Update(tea.MouseReleaseMsg{X: z.StartX + 1 + gw + 2, Y: z.StartY + 3, Button: tea.MouseLeft})
	if !c.HasSelection() || c.row != 2 || c.col != 2 {
		t.Errorf("drag selects: %d,%d sel=%v", c.row, c.col, c.HasSelection())
	}
}
