package editor

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
)

func TestBodyFormRoundTrip(t *testing.T) {
	b := BodyTab()
	b.SetForm(true)
	b.SetValue(`{"zeta": "1", "alpha": 2, "tags": ["a", "b"], "meta": {"x": true}}`)

	var names []string
	for _, f := range b.grid.rows {
		names = append(names, f.name.Value()+"="+f.value.Value())
	}
	want := []string{"zeta=1", "alpha=2", "tags=a", "tags=b", `meta={"x":true}`}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("rows = %v, want %v (key order kept, arrays repeated)", names, want)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(b.Value()), &got); err != nil {
		t.Fatalf("value is not JSON: %v\n%s", err, b.Value())
	}
	if got["zeta"] != "1" || got["alpha"] != "2" || !reflect.DeepEqual(got["tags"], []any{"a", "b"}) {
		t.Errorf("value = %v", got)
	}
	if strings.Index(b.Value(), "zeta") > strings.Index(b.Value(), "alpha") {
		t.Errorf("row order must be kept:\n%s", b.Value())
	}

	b.SetValue("a=1&b=two+words")
	if b.Value() != "{\n  \"a\": \"1\",\n  \"b\": \"two words\"\n}" {
		t.Errorf("urlencoded input → %q", b.Value())
	}
	b.SetValue("")
	if len(b.grid.rows) != 1 || b.Value() != "" {
		t.Errorf("empty body: one blank row and an empty value, got %d rows %q", len(b.grid.rows), b.Value())
	}
}

func TestBodyFormEditing(t *testing.T) {
	b := BodyTab()
	b.SetForm(true)
	b.SetValue(`{"user": "ana"}`)
	m, _ := b.Update(tabs.SetActiveTabMsg{Active: true})
	b = m.(*body)

	press := func(msg tea.KeyPressMsg) {
		m, _ := b.Update(msg)
		b = m.(*body)
	}
	press(tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl}) // add field, focus its name
	for _, r := range "pass" {
		press(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	press(tea.KeyPressMsg{Code: tea.KeyTab})
	for _, r := range "s3cret" {
		press(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(b.Value()), &got); err != nil || got["user"] != "ana" || got["pass"] != "s3cret" {
		t.Errorf("value = %s (%v)", b.Value(), err)
	}

	// Text mode is unchanged.
	txt := BodyTab()
	txt.SetValue(`{"raw": true}`)
	if txt.Value() != `{"raw": true}` || txt.form {
		t.Errorf("text body = %q", txt.Value())
	}
}
