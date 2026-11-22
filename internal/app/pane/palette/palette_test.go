package palette

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestFuzzy(t *testing.T) {
	if _, _, ok := Fuzzy("pets get", "Open GET /pets/{id}"); !ok {
		t.Error("words match in any order")
	}
	if _, _, ok := Fuzzy("xyz", "Open GET /pets"); ok {
		t.Error("unexpected match")
	}
	a, _, _ := Fuzzy("pets", "Open GET /pets")
	b, _, _ := Fuzzy("pets", "Open GET /p/e/t/s")
	if a <= b {
		t.Errorf("consecutive match should score higher: %d vs %d", a, b)
	}
	_, pos, _ := Fuzzy("get", "Open GET /x")
	if len(pos) != 3 || pos[0] != 5 {
		t.Errorf("positions %v", pos)
	}
}

type pick struct{ name string }

func press(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

func TestPaletteChooses(t *testing.T) {
	m := New([]Command{
		{Group: "Open", Title: "GET /pets", Msg: pick{"list"}},
		{Group: "Open", Title: "POST /pets", Msg: pick{"create"}},
		{Group: "Environment", Title: "staging", Msg: pick{"env"}},
	}, 100, 30)
	for _, r := range "stag" {
		m, _ = m.Update(press(string(r)))
	}
	if len(m.matches) != 1 || !strings.Contains(ansi.Strip(m.View()), "staging") {
		t.Fatalf("filter: %d matches\n%s", len(m.matches), m.View())
	}
	_, cmd := m.Update(press("enter"))
	if got := cmd().(ChosenMsg).Msg.(pick).name; got != "env" {
		t.Errorf("chose %q", got)
	}

	m = New([]Command{{Group: "Open", Title: "a", Msg: pick{"a"}}, {Group: "Open", Title: "b", Msg: pick{"b"}}}, 100, 30)
	m, _ = m.Update(press("down"))
	_, cmd = m.Update(press("enter"))
	if got := cmd().(ChosenMsg).Msg.(pick).name; got != "b" {
		t.Errorf("down then enter chose %q", got)
	}
	if _, cmd := m.Update(press("esc")); cmd == nil {
		t.Error("esc must close")
	} else if _, ok := cmd().(ClosedMsg); !ok {
		t.Error("esc must send ClosedMsg")
	}
}
