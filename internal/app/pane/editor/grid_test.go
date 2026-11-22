package editor

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestGridViewDoesNotMutate(t *testing.T) {
	g := newGrid()
	g.setWidth(40)
	g.setMap(map[string]string{"b": "2", "a": "1", "c": "3"})
	var names []string
	for _, r := range g.rows {
		names = append(names, r.name.Value())
	}
	if !reflect.DeepEqual(names, []string{"a", "b", "c"}) {
		t.Errorf("rows must be sorted by name: %v", names)
	}

	before := g.rows[1].value.Style.GetBorderTopForeground()
	plain := g.view(-1)
	focused := g.view(3)
	if g.rows[1].value.Style.GetBorderTopForeground() != before {
		t.Error("view must not change the grid's styles")
	}
	if focused == plain {
		t.Error("the focused cell must look different")
	}
	if after := g.view(-1); after != plain {
		t.Error("rendering with focus must not affect later renders")
	}
}

func TestGridEditAndAdd(t *testing.T) {
	g := newGrid()
	g.setWidth(40)
	g.setMap(nil)
	if g.cells() != 2 {
		t.Fatalf("empty grid has one blank row, got %d cells", g.cells())
	}
	cell := g.add()
	if cell != 2 || g.cells() != 4 {
		t.Fatalf("add → cell %d, %d cells", cell, g.cells())
	}
	for _, r := range "X-Trace" {
		g.update(cell, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	g.update(cell+1, tea.KeyPressMsg{Code: '1', Text: "1"})
	if got := g.toMap(); !reflect.DeepEqual(got, map[string]string{"X-Trace": "1"}) {
		t.Errorf("toMap = %v (blank rows are dropped)", got)
	}
	if g.update(99, tea.KeyPressMsg{Code: 'x'}) != nil {
		t.Error("out-of-range cells are ignored")
	}
}
