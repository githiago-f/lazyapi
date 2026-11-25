package editor

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/model"
)

func TestDisabledRowsRoundtrip(t *testing.T) {
	req := model.Request{
		Method: model.POST, URI: "/pets/{id}",
		Headers: map[string]string{"X-On": "1"},
		Query:   map[string]string{"q": "a"},
		Params:  map[string]string{"id": "7"},
		Body:    model.Body{Type: "application/x-www-form-urlencoded", Raw: `{"name": "rex"}`},
		Auth: []model.AuthScheme{
			{Type: model.AuthBearer, SchemeName: "b", Token: "t"},
			{Type: model.AuthAPIKey, SchemeName: "k", KeyName: "X-Key", KeyIn: "header", KeyValue: "v", Disabled: true},
		},
		Disabled: model.Disabled{
			Headers: map[string]string{"X-Off": "2"},
			Query:   map[string]string{"debug": "true"},
			Form:    map[string]string{"color": "red"},
		},
	}
	m, _ := New(&req).Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	rp := m.(RequestPane).SetValue(req)
	got := rp.GetAsRequestData()

	if !reflect.DeepEqual(got.Disabled, req.Disabled) {
		t.Errorf("disabled: %+v", got.Disabled)
	}
	if !reflect.DeepEqual(got.Headers, req.Headers) || !reflect.DeepEqual(got.Query, req.Query) {
		t.Errorf("enabled rows: %v %v", got.Headers, got.Query)
	}
	if strings.Contains(got.Body.Raw, "color") || !strings.Contains(got.Body.Raw, "rex") {
		t.Errorf("form body sends only enabled fields: %s", got.Body.Raw)
	}
	if len(got.Auth) != 2 || got.Auth[0].Disabled || !got.Auth[1].Disabled {
		t.Errorf("auth: %+v", got.Auth)
	}
}

func TestGridToggleAndCheckboxClick(t *testing.T) {
	components.Z.Clear()
	h := HeaderTab()
	m, _ := h.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	h = m.(*header)
	h.SetValue(map[string]string{"A": "1", "B": "2"}, nil)
	h.SetActive(true)

	m, _ = h.Update(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl}) // row A
	h = m.(*header)
	if !reflect.DeepEqual(h.Value(), map[string]string{"B": "2"}) || !reflect.DeepEqual(h.Disabled(), map[string]string{"A": "1"}) {
		t.Fatalf("ctrl+t: on=%v off=%v", h.Value(), h.Disabled())
	}
	view := h.View().Content
	if plain := ansi.Strip(view); !strings.Contains(plain, "☐") || !strings.Contains(plain, "☑") {
		t.Errorf("checkboxes:\n%s", plain)
	}
	components.Z.Scan(view)
	z := components.Z.Get(h.grid.boxZone(0))
	m, _ = h.Update(tea.MouseClickMsg{X: z.StartX + 1, Y: (z.StartY + z.EndY) / 2, Button: tea.MouseLeft})
	h = m.(*header)
	if h.Disabled() != nil || len(h.Value()) != 2 {
		t.Errorf("click turns it back on: on=%v off=%v", h.Value(), h.Disabled())
	}
}
