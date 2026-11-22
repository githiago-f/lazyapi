package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestButtonStates(t *testing.T) {
	Z.NewGlobal()
	b := Button{Label: "▶ Send", BusyLabel: "◌ Sending", Active: true}
	idle := b.View().Content
	if !strings.Contains(ansi.Strip(idle), "  ▶ Send  ") {
		t.Errorf("idle keeps its padding: %q", ansi.Strip(idle))
	}
	if strings.Contains(idle, "48;2;203;166;247") {
		t.Error("an unfocused button must not be filled with the accent")
	}

	b.Focused = true
	focused := b.View().Content
	if !strings.Contains(focused, "48;2;203;166;247") || !strings.Contains(ansi.Strip(focused), "  ▶ Send  ") {
		t.Errorf("focused = %q", focused)
	}

	b.Focused = false
	if b.View().Content != idle {
		t.Error("losing the focus must restore the idle look")
	}

	b.Busy = true
	if !strings.Contains(ansi.Strip(b.View().Content), "◌ Sending") {
		t.Error("busy label")
	}
	if m, _ := b.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); m.(Button).Clicked {
		t.Error("a busy button cannot be pressed")
	}
	b.Busy = false
	m, _ := b.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.(Button).Clicked || !strings.Contains(m.(Button).View().Content, "48;2;166;227;161") {
		t.Error("pressing flashes green")
	}
}
