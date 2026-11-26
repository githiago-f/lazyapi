package app

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/githiago-f/lazyapi/internal/components"
)

// Some terminals (Windows Terminal among them) send a file dropped on the
// window as typed text, not as a bracketed paste: the program gets one key
// press per character of the path, which would type it, or fire shortcuts
// outside text fields. Printable keys are therefore held until burstWindow
// passes without another one; a burst that spells existing files is
// delivered as a tea.PasteMsg (a drop), anything else is replayed as the
// keys it was. People leave tens of milliseconds between keys (key repeat
// too); a terminal writing a drop does not.

// burstWindow is how long printable keys are held (0 turns it off; tests
// that quit the program from outside, which drops held keys, do).
var burstWindow = 12 * time.Millisecond

// maxBurst caps the held keys (a long paste is flushed in pieces).
const maxBurst = 8192

type burstFlushMsg struct{ id int }

// burstable reports whether a key can be part of a burst: printable text
// (shift allowed), and line breaks between dropped paths.
func burstable(k tea.KeyPressMsg, pending bool) bool {
	if k.Mod&^tea.ModShift != 0 {
		return false
	}
	return k.Text != "" || (pending && k.Code == tea.KeyEnter)
}

// holdKey buffers a printable key; ok is false when msg is not one.
func (t Tui) holdKey(msg tea.Msg) (Tui, tea.Cmd, bool) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok || t.replaying || burstWindow <= 0 || !burstable(k, len(t.burst) > 0) {
		return t, nil, false
	}
	t.burst = append(t.burst, k)
	if len(t.burst) >= maxBurst {
		m, cmd := t.flushBurst()
		return m.(Tui), cmd, true
	}
	t.burstID++ // each key restarts the wait: earlier flushes go stale
	id := t.burstID
	return t, tea.Tick(burstWindow, func(time.Time) tea.Msg { return burstFlushMsg{id: id} }), true
}

// interrupts reports whether msg must come after the held keys (other keys,
// pastes and clicks keep their order; ticks and mouse motion need not).
func interrupts(msg tea.Msg) bool {
	switch msg.(type) {
	case tea.KeyPressMsg, tea.PasteMsg, tea.MouseClickMsg, tea.MouseReleaseMsg:
		return true
	}
	return false
}

// flushBurst delivers the held keys: as a paste when they spell dropped
// files, else one by one.
func (t Tui) flushBurst() (tea.Model, tea.Cmd) {
	keys := t.burst
	t.burst = nil
	if len(keys) == 0 {
		return t, nil
	}
	if len(keys) > 1 {
		var text strings.Builder
		for _, k := range keys {
			if k.Code == tea.KeyEnter {
				text.WriteByte('\n')
			} else {
				text.WriteString(k.Text)
			}
		}
		if components.DroppedFiles(text.String()) != nil {
			return t.Update(tea.PasteMsg{Content: text.String()})
		}
	}
	t.replaying = true
	var (
		m    tea.Model = t
		cmds []tea.Cmd
	)
	for _, k := range keys {
		var cmd tea.Cmd
		m, cmd = m.Update(k)
		cmds = append(cmds, cmd)
	}
	out := m.(Tui)
	out.replaying = false
	return out, tea.Batch(cmds...)
}

// frame caches the last rendered view (shared by the Tui's copies).
type frame struct {
	last  tea.View
	drawn bool
	reuse bool
}

// unchanged tells View that this update changed nothing on screen.
func (t Tui) unchanged() {
	if t.frame != nil {
		t.frame.reuse = true
	}
}
