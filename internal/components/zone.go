// Package components describe all components used by the TUI
package components

import (
	"strconv"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Mouse zones: a component wraps what it renders in Z.Mark(id, s); the TUI
// passes the final frame through Z.Scan, which strips the markers and
// records where each zone ended up. Update then asks Z.Get(id).InBounds(msg).
//
// Markers are CSI sequences ("ESC [ n z" starts zone n, "ESC [ n ; 1 z" ends
// it) that terminals never see: lipgloss and ansi measure them as zero-width,
// so styling, joining and overlays keep them in place.

// Zone is where a marked string was drawn in the last frame.
type Zone struct {
	StartX, StartY, EndX, EndY int // EndX is exclusive
	ok                         bool
}

// InBounds reports whether the mouse event happened inside the zone (the
// rectangle from its first cell to its last).
func (z Zone) InBounds(msg tea.MouseMsg) bool {
	if !z.ok || msg == nil {
		return false
	}
	m := msg.Mouse()
	return z.Contains(m.X, m.Y)
}

// Contains reports whether the cell x, y is inside the zone.
func (z Zone) Contains(x, y int) bool {
	if !z.ok {
		return false
	}
	return y >= z.StartY && y <= z.EndY && x >= z.StartX && x < z.EndX
}

// Pos is the event position relative to the zone's top-left cell.
func (z Zone) Pos(msg tea.MouseMsg) (x, y int) {
	m := msg.Mouse()
	return m.X - z.StartX, m.Y - z.StartY
}

// IsZero reports whether the zone was not drawn in the last frame.
func (z Zone) IsZero() bool { return !z.ok }

// Manager tracks zones between View and Update.
type Manager struct {
	mu     sync.Mutex
	ids    map[string]int
	names  []string
	zones  map[string]Zone
	prefix int
}

// Z is the TUI's zone manager.
var Z = NewManager()

// NewManager returns an empty zone manager.
func NewManager() *Manager {
	return &Manager{ids: map[string]int{}, zones: map[string]Zone{}}
}

// NewPrefix returns a unique prefix for zone ids of one component instance.
func (m *Manager) NewPrefix() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prefix++
	return "z" + strconv.Itoa(m.prefix) + ":"
}

func (m *Manager) number(id string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.ids[id]
	if !ok {
		n = len(m.names)
		m.ids[id] = n
		m.names = append(m.names, id)
	}
	return n
}

// Mark wraps content in the markers of zone id.
func (m *Manager) Mark(id, content string) string {
	if id == "" {
		return content
	}
	n := strconv.Itoa(m.number(id))
	return "\x1b[" + n + "z" + content + "\x1b[" + n + ";1z"
}

// Get returns the zone id as drawn by the last Scan.
func (m *Manager) Get(id string) Zone {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.zones[id]
}

// Clear forgets every recorded zone (tests).
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.zones = map[string]Zone{}
}

// Scan strips the markers from a frame and records the zones' positions.
// Zones that were not drawn (or lost a marker under an overlay) are gone.
func (m *Manager) Scan(content string) string {
	if !strings.Contains(content, "z") || !strings.Contains(content, "\x1b[") {
		m.mu.Lock()
		m.zones = map[string]Zone{}
		m.mu.Unlock()
		return content
	}
	var (
		out    strings.Builder
		x, y   int
		state  byte
		starts = map[int][2]int{}
		found  = map[int]Zone{}
	)
	out.Grow(len(content))
	s := content
	for len(s) > 0 {
		seq, width, n, newState := ansi.DecodeSequence(s, state, nil)
		state = newState
		s = s[n:]
		if id, end, ok := parseMarker(seq); ok {
			if !end {
				starts[id] = [2]int{x, y}
			} else if st, ok := starts[id]; ok {
				z := Zone{StartX: st[0], StartY: st[1], EndX: x, EndY: y, ok: true}
				if z.EndX <= z.StartX && z.EndY > z.StartY {
					// The block ended at a line start: its width is unknown,
					// keep the start column as the left edge.
					z.EndX = z.StartX + 1
				}
				if z.EndX < z.StartX {
					z.StartX, z.EndX = z.EndX, z.StartX
				}
				found[id] = z
				delete(starts, id)
			}
			continue
		}
		out.WriteString(seq)
		if seq == "\n" {
			x = 0
			y++
			continue
		}
		x += width
	}
	m.mu.Lock()
	m.zones = make(map[string]Zone, len(found))
	for id, z := range found {
		if id < len(m.names) {
			m.zones[m.names[id]] = z
		}
	}
	m.mu.Unlock()
	return out.String()
}

// parseMarker recognises "ESC [ n z" and "ESC [ n ; 1 z".
func parseMarker(seq string) (id int, end bool, ok bool) {
	if len(seq) < 4 || seq[0] != '\x1b' || seq[1] != '[' || seq[len(seq)-1] != 'z' {
		return 0, false, false
	}
	body := seq[2 : len(seq)-1]
	if before, ok0 := strings.CutSuffix(body, ";1"); ok0 {
		body, end = before, true
	}
	n, err := strconv.Atoi(body)
	if err != nil || n < 0 {
		return 0, false, false
	}
	return n, end, true
}
