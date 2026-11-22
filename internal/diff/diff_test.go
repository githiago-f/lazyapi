package diff

import (
	"strings"
	"testing"
)

func TestLines(t *testing.T) {
	a := "{\n  \"id\": 1,\n  \"name\": \"ana\",\n  \"role\": \"user\"\n}"
	b := "{\n  \"id\": 1,\n  \"name\": \"bia\",\n  \"role\": \"user\",\n  \"admin\": true\n}"
	lines := Lines(a, b)
	added, removed := Stats(lines)
	if added != 3 || removed != 2 {
		t.Errorf("stats +%d -%d\n%s", added, removed, Unified(lines, 3))
	}
	out := Unified(lines, 3)
	for _, want := range []string{`-   "name": "ana",`, `+   "name": "bia",`, `+   "admin": true`, `    "id": 1,`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if Changed(Lines(a, a)) {
		t.Error("same text must not change")
	}
}

func TestHunksFoldContext(t *testing.T) {
	var a, b []string
	for i := 0; i < 50; i++ {
		a = append(a, "line")
		b = append(b, "line")
	}
	b[25] = "changed"
	out := Unified(Lines(strings.Join(a, "\n"), strings.Join(b, "\n")), 2)
	if !strings.Contains(out, "@@ 23 unchanged lines @@") || !strings.Contains(out, "@@ 22 unchanged lines @@") || strings.Count(out, "\n") > 10 {
		t.Errorf("folding:\n%s", out)
	}
}
