package requests

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/model"
)

func TestRequestListHover(t *testing.T) {
	rl := NewRequestList()
	rl.SetSize(60, 20)
	rl = rl.SetItems([]RequestItem{
		{Method: model.GET, URI: "/a"},
		{Method: model.GET, URI: "/b"},
	})
	components.Z.Scan(rl.View().Content)
	z := components.Z.Get(rowZone(1))

	m, _ := rl.Update(tea.MouseMotionMsg{X: z.StartX + 1, Y: z.StartY})
	rl = m.(RequestList)
	if rl.hover != 1 {
		t.Fatalf("hover = %d, want 1", rl.hover)
	}
	if rl.cursor != 0 {
		t.Errorf("hover moved the cursor to %d", rl.cursor)
	}

	m, _ = rl.Update(tea.MouseMotionMsg{X: 0, Y: 99})
	if rl = m.(RequestList); rl.hover != -1 {
		t.Errorf("hover = %d after leaving the rows, want -1", rl.hover)
	}
}
