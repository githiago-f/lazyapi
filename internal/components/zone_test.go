package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

func click(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}
}

func TestZoneScanLayout(t *testing.T) {
	m := NewManager()
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	a := m.Mark("a", box.Render("one\ntwo")) // 5x4
	b := m.Mark("b", box.Render("three"))    // 7x3
	frame := lipgloss.JoinVertical(lipgloss.Left, "title", lipgloss.JoinHorizontal(lipgloss.Top, a, " ", b))
	out := m.Scan(frame)
	if strings.Contains(out, "\x1b[0z") || strings.Contains(out, ";1z") {
		t.Fatalf("markers left in output: %q", out)
	}
	if lipgloss.Width(out) != lipgloss.Width(frame) {
		t.Fatalf("width changed")
	}
	za, zb := m.Get("a"), m.Get("b")
	if !za.InBounds(click(0, 1)) || !za.InBounds(click(4, 4)) || za.InBounds(click(5, 1)) || za.InBounds(click(0, 0)) {
		t.Fatalf("zone a = %+v", za)
	}
	if !zb.InBounds(click(6, 1)) || !zb.InBounds(click(12, 3)) || zb.InBounds(click(13, 1)) || zb.InBounds(click(6, 4)) {
		t.Fatalf("zone b = %+v", zb)
	}
	if m.Get("missing").InBounds(click(0, 0)) {
		t.Fatal("missing zone in bounds")
	}
}

func TestZoneUnderOverlay(t *testing.T) {
	m := NewManager()
	bg := strings.Repeat(".", 20) + "\n" + "...." + m.Mark("x", "XXXX") + "............"
	fg := m.Mark("pop", "POP")
	out := m.Scan(overlay.Composite(fg, bg, overlay.Left, overlay.Top, 2, 1))
	if got := m.Get("pop"); !got.InBounds(click(2, 1)) || !got.InBounds(click(4, 1)) || got.InBounds(click(5, 1)) {
		t.Fatalf("pop zone = %+v (%q)", got, out)
	}
}
