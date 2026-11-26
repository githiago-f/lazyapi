package editor

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/model"
)

func activeBody(t *testing.T) *body {
	t.Helper()
	b := BodyTab()
	m, _ := b.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	b = m.(*body)
	m, _ = b.Update(tabs.SetActiveTabMsg{Active: true})
	return m.(*body)
}

func feed(b *body, msgs ...tea.Msg) *body {
	for _, msg := range msgs {
		m, _ := b.Update(msg)
		b = m.(*body)
	}
	return b
}

// pick opens the type select with alt+t and picks t with the arrows.
func pick(t *testing.T, b *body, mime model.MimeType) *body {
	t.Helper()
	b = feed(b, tea.KeyPressMsg{Code: 't', Mod: tea.ModAlt})
	if !b.kind.Open {
		t.Fatal("alt+t opens the type list")
	}
	for range len(b.types) {
		if b.selected() == mime {
			break
		}
		b = feed(b, tea.KeyPressMsg{Code: tea.KeyDown})
		if b.Type() == mime && b.kind.Open {
			t.Fatal("browsing the list must not change the type")
		}
	}
	b = feed(b, tea.KeyPressMsg{Code: tea.KeyEnter})
	if b.kind.Open || b.Type() != mime {
		t.Fatalf("picked %q, type %q (open %v)", mime, b.Type(), b.kind.Open)
	}
	return b
}

func tempFiles(t *testing.T, names ...string) []string {
	t.Helper()
	dir := t.TempDir()
	var out []string
	for _, n := range names {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte(n), 0o600); err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

func TestBodyTypeSwitchCarriesContent(t *testing.T) {
	b := activeBody(t)
	b.SetValue(`{"name": "ana", "age": 3}`)

	b = pick(t, b, model.MultipartForm)
	if b.mode != modeForm || b.focus != focusContent {
		t.Fatalf("multipart edits rows with the focus on them: mode %v focus %v", b.mode, b.focus)
	}
	if got := b.rowsMap(false, true); !reflect.DeepEqual(got, map[string]string{"name": "ana", "age": "3"}) {
		t.Errorf("rows from the JSON object: %v", got)
	}

	b = pick(t, b, model.ApplicationJSON)
	if b.mode != modeRaw || !strings.Contains(b.Value(), `"name": "ana"`) {
		t.Errorf("rows back to JSON: %q", b.Value())
	}

	b = pick(t, b, "")
	if b.Value() != "" || b.Type() != "" {
		t.Errorf("none sends nothing: %q %q", b.Type(), b.Value())
	}
	b = pick(t, b, model.FormURLEncoded) // through "none": the JSON is kept
	if got := b.rowsMap(false, true); got["name"] != "ana" {
		t.Errorf("rows after none: %v", got)
	}
}

func TestBodyDropFiles(t *testing.T) {
	files := tempFiles(t, "a b.png", "c.csv", "d.txt")

	// multipart: the focused row becomes a file, other files new rows.
	b := activeBody(t)
	b.SetType(model.MultipartForm)
	b.SetValue(`{"caption": "hi"}`)
	b = feed(b, tabs.SetActiveTabMsg{Active: true})
	b = feed(b, tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl}) // a blank row, focused
	for _, r := range "avatar" {
		b = feed(b, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	b = feed(b, tea.PasteMsg{Content: "'" + files[0] + "' '" + files[1] + "' "})
	want := map[string]string{"avatar": files[0], "avatar2": files[1]}
	if got := b.Files(); !reflect.DeepEqual(got, want) {
		t.Errorf("files = %v, want %v", got, want)
	}
	if !strings.Contains(b.Value(), "caption") || strings.Contains(b.Value(), "avatar") {
		t.Errorf("text fields only in the JSON: %s", b.Value())
	}
	// ctrl+f turns a file row back into text.
	b = feed(b, tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	if len(b.Files()) != 1 || !strings.Contains(b.Value(), "avatar2") {
		t.Errorf("ctrl+f: files %v, value %s", b.Files(), b.Value())
	}

	// urlencoded cannot send files.
	u := activeBody(t)
	u.SetType(model.FormURLEncoded)
	u = feed(u, tabs.SetActiveTabMsg{Active: true})
	m, cmd := u.Update(tea.PasteMsg{Content: files[2]})
	if u = m.(*body); cmd == nil || u.Files() != nil {
		t.Error("a drop on urlencoded tells to use multipart")
	}

	// No body: a drop makes it a binary body.
	n := activeBody(t)
	n = pick(t, n, "")
	n = feed(n, tea.PasteMsg{Content: files[2]})
	if n.Type() != model.OctetStream || n.File() != files[2] || n.Value() != "" {
		t.Errorf("drop on none: %q %q", n.Type(), n.File())
	}
	// A binary body takes the drop as its file.
	n = feed(n, tea.PasteMsg{Content: files[0]})
	if n.File() != files[0] {
		t.Errorf("drop on binary: %q", n.File())
	}
}

func TestEditorBodyRoundtrip(t *testing.T) {
	files := tempFiles(t, "a.png", "b.png")
	cases := []model.Request{
		{Method: model.POST, URI: "/up", Body: model.Body{Type: model.MultipartForm, Raw: `{"caption": "hi"}`,
			Files: map[string]string{"avatar": files[0]}},
			Disabled: model.Disabled{Files: map[string]string{"old": files[1]}}},
		{Method: model.PUT, URI: "/img", Body: model.Body{Type: "image/png", File: files[0]}},
		{Method: model.GET, URI: "/none"},
	}
	for _, req := range cases {
		m, _ := New(&req).Update(tea.WindowSizeMsg{Width: 160, Height: 40})
		got := m.(RequestPane).SetValue(req).GetAsRequestData()
		if got.Body.Type != req.Body.Type || got.Body.File != req.Body.File || !reflect.DeepEqual(got.Body.Files, req.Body.Files) {
			t.Errorf("%s: body %+v, want %+v", req.URI, got.Body, req.Body)
		}
		if !reflect.DeepEqual(got.Disabled.Files, req.Disabled.Files) {
			t.Errorf("%s: disabled files %v", req.URI, got.Disabled.Files)
		}
	}
}
