package scrollable

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

type lines int

func (n lines) Init() tea.Cmd                       { return nil }
func (n lines) Update(tea.Msg) (tea.Model, tea.Cmd) { return n, nil }
func (n lines) View() tea.View {
	out := make([]string, int(n))
	for i := range out {
		out[i] = fmt.Sprintf("line %d", i)
	}
	return tea.NewView(strings.Join(out, "\n"))
}

func TestScroll(t *testing.T) {
	var m tea.Model = New(lines(10))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 20, Height: 4})
	first := func() string { return strings.Split(m.View().Content, "\n")[0] }

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if first() != "line 4" {
		t.Errorf("pgdown → %q", first())
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if first() != "line 6" { // clamped: the last page fills the view
		t.Errorf("clamped at the end → %q", first())
	}
	m, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if first() != "line 3" {
		t.Errorf("wheel up → %q", first())
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	if first() != "line 0" || len(strings.Split(m.View().Content, "\n")) != 4 {
		t.Errorf("top → %q", m.View().Content)
	}

	short := tea.Model(New(lines(2)))
	short, _ = short.Update(tea.WindowSizeMsg{Width: 20, Height: 4})
	short, _ = short.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if short.View().Content != "line 0\nline 1" {
		t.Errorf("short content → %q", short.View().Content)
	}
}
