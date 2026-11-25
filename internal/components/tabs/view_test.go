package tabs

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/githiago-f/lazyapi/internal/components"
)

type content struct {
	active bool
	clicks int
}

func (c *content) Init() tea.Cmd { return nil }
func (c *content) View() tea.View {
	return tea.NewView("line one of the content\nline two")
}
func (c *content) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SetActiveTabMsg:
		c.active = msg.Active
	case tea.MouseClickMsg:
		c.clicks++
	}
	return c, nil
}

func TestTabsClick(t *testing.T) {
	components.Z.Clear()
	a, b := &content{}, &content{}
	m := New(NewTab("Alpha", a), NewTab("Beta", b))
	m.SetWidth(60)
	components.Z.Scan(m.View().Content)

	z := components.Z.Get(m.labelZone(1))
	nm, _ := m.Update(tea.MouseClickMsg{X: z.StartX + 1, Y: z.StartY, Button: tea.MouseLeft})
	m = nm.(Model)
	if m.Cursor() != 1 || m.Selected() {
		t.Fatalf("label click: cursor=%d selected=%v", m.Cursor(), m.Selected())
	}
	components.Z.Scan(m.View().Content)
	z = components.Z.Get(m.contentZone())
	nm, _ = m.Update(tea.MouseClickMsg{X: z.StartX + 2, Y: z.StartY + 1, Button: tea.MouseLeft})
	m = nm.(Model)
	if !m.Selected() || !b.active || b.clicks != 1 {
		t.Errorf("content click: selected=%v active=%v clicks=%d", m.Selected(), b.active, b.clicks)
	}
	if m, _ = m.Deselect(); m.Selected() || b.active {
		t.Error("deselect")
	}
}
