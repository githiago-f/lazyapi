package requests

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/model"
)

// longList has n requests in two tag groups, sized to height rows.
func longList(n, height int) RequestList {
	items := make([]RequestItem, n)
	for i := range items {
		tag := "pets"
		if i >= n/2 {
			tag = "users"
		}
		items[i] = RequestItem{Method: model.GET, URI: fmt.Sprintf("/r%02d", i), Tags: []string{tag}}
	}
	rl := NewRequestList()
	rl.SetSize(60, height)
	return rl.SetItems(items)
}

func press(rl RequestList, keys ...string) RequestList {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "end":
			msg = tea.KeyPressMsg{Code: tea.KeyEnd}
		case "home":
			msg = tea.KeyPressMsg{Code: tea.KeyHome}
		case "ctrl+d":
			msg = tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
		case "ctrl+u":
			msg = tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
		}
		m, _ := rl.Update(msg)
		rl = m.(RequestList)
	}
	return rl
}

func screen(rl RequestList) string {
	return ansi.Strip(components.Z.Scan(rl.View().Content))
}

func TestLongListFitsAndSaysWhatIsHidden(t *testing.T) {
	rl := longList(40, 12)
	out := screen(rl)
	if h := strings.Count(out, "\n") + 1; h != 12 {
		t.Fatalf("height %d, want 12:\n%s", h, out)
	}
	if !strings.Contains(out, "/r00") || strings.Contains(out, "/r39") {
		t.Errorf("first page:\n%s", out)
	}
	last := out[strings.LastIndex(out, "\n")+1:]
	if !strings.Contains(last, "1/40") || !strings.Contains(last, "↓") || strings.Contains(last, "↑") {
		t.Errorf("position line = %q", last)
	}
}

func TestCursorStaysOnScreen(t *testing.T) {
	rl := longList(40, 12)
	for i := 0; i < 25; i++ {
		rl = press(rl, "down")
		if out := screen(rl); !strings.Contains(out, fmt.Sprintf("/r%02d", rl.cursor)) {
			t.Fatalf("cursor %d is off screen:\n%s", rl.cursor, out)
		}
	}
	if out := screen(rl); !strings.Contains(out, "↑") || !strings.Contains(out, "26/40") {
		t.Errorf("after scrolling:\n%s", out)
	}

	rl = press(rl, "end")
	out := screen(rl)
	if rl.cursor != 39 || !strings.Contains(out, "/r39") || strings.Contains(out, "↓") {
		t.Errorf("end (cursor %d):\n%s", rl.cursor, out)
	}
	rl = press(rl, "home")
	if out := screen(rl); rl.cursor != 0 || !strings.Contains(out, "── pets (20) ──") {
		t.Errorf("home shows the first tag header (cursor %d):\n%s", rl.cursor, out)
	}
}

func TestPageKeys(t *testing.T) {
	rl := longList(40, 12)
	rl = press(rl, "ctrl+d")
	if rl.cursor == 0 || !strings.Contains(screen(rl), fmt.Sprintf("/r%02d", rl.cursor)) {
		t.Fatalf("ctrl+d: cursor %d\n%s", rl.cursor, screen(rl))
	}
	rl = press(rl, "ctrl+u")
	if rl.cursor != 0 {
		t.Errorf("ctrl+u: cursor %d", rl.cursor)
	}
}

// The tags overlay opens next to the selected row, also when scrolled.
func TestCursorPosYFollowsScroll(t *testing.T) {
	rl := press(longList(40, 12), "end")
	lines := strings.Split(screen(rl), "\n")
	// CursorPosY counts from the filter's border line.
	if row := lines[rl.CursorPosY()+1]; !strings.Contains(row, "/r39") {
		t.Errorf("CursorPosY %d points at %q", rl.CursorPosY(), row)
	}
}

func TestShortListHasNoPositionLine(t *testing.T) {
	out := screen(longList(4, 12))
	if strings.Contains(out, "1/4") || strings.Contains(out, "page") {
		t.Errorf("short list:\n%s", out)
	}
}

// Clicks only reach the rows on screen.
func TestClickAfterScroll(t *testing.T) {
	rl := press(longList(40, 12), "end")
	components.Z.Scan(rl.View().Content)
	z := components.Z.Get(rowZone(35))
	if z.IsZero() {
		t.Fatal("row 35 is not on screen")
	}
	m, _ := rl.Update(tea.MouseClickMsg{X: z.StartX + 1, Y: z.StartY, Button: tea.MouseLeft})
	if rl = m.(RequestList); rl.cursor != 35 {
		t.Errorf("click selected %d", rl.cursor)
	}
	if !components.Z.Get(rowZone(0)).IsZero() {
		t.Error("row 0 is off screen but still clickable")
	}
}
