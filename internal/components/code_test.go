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
