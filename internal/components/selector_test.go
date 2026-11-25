package components

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func keyMsg(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

func send(s Selector, keys ...string) Selector {
	for _, k := range keys {
		m, _ := s.Update(keyMsg(k))
		s = m.(Selector)
	}
	return s
}

func TestSelectorKeyboard(t *testing.T) {
	s := Selector{Labels: []string{"POST", "GET", "PATCH", "PUT", "DELETE", "OPTIONS", "HEAD"}, Cursor: 4}
	if s = send(s, "enter"); !s.Open {
		t.Fatal("enter opens")
	}
	if s = send(s, "p"); s.Value() != "POST" {
		t.Errorf("p from DELETE: %s", s.Value())
	}
	if s = send(s, "p"); s.Value() != "PATCH" {
		t.Errorf("p again cycles: %s", s.Value())
	}
	if s = send(s, "u"); s.Value() != "PUT" {
		t.Errorf("pu: %s", s.Value())
	}
	if s = send(s, "end"); s.Value() != "HEAD" {
		t.Errorf("end: %s", s.Value())
	}
	if s = send(s, "esc"); s.Open || s.Value() != "DELETE" {
		t.Errorf("esc restores: %s open=%v", s.Value(), s.Open)
	}
	if s = send(s, "enter", "down", "enter"); s.Open || s.Value() != "OPTIONS" {
		t.Errorf("down+enter picks: %s", s.Value())
	}
}

func TestSelectorMouse(t *testing.T) {
	Z.Clear()
	s := Selector{Labels: []string{"a", "b", "c"}, ID: "t"}
	Z.Scan(s.View().Content)
	m, _ := s.Update(click(1, 1))
	s = m.(Selector)
	if !s.Open {
		t.Fatal("a click opens")
	}
	Z.Scan(s.DropDownView())
	m, _ = s.Update(click(2, 3)) // border, a, b, c
	if s = m.(Selector); s.Open || s.Value() != "c" {
		t.Errorf("click picks: %s open=%v", s.Value(), s.Open)
	}
	m, _ = s.Update(tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseLeft})
	s = m.(Selector)
	m, _ = s.Update(click(50, 50))
	if s = m.(Selector); s.Open || s.Value() != "c" {
		t.Errorf("click outside cancels: %s", s.Value())
	}
}
