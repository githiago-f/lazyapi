package syntax

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func kindAt(l Line, sub string) Kind {
	i := strings.Index(string(l.Runes), sub)
	return l.Kinds[len([]rune(string(l.Runes)[:i]))]
}

func TestCodeJSON(t *testing.T) {
	lines := Code("json", "{\n  \"name\": \"ana\",\n  \"age\": 3,\n  \"ok\": true\n}")
	if len(lines) != 5 {
		t.Fatalf("lines = %d", len(lines))
	}
	if kindAt(lines[1], "name") != Prop || kindAt(lines[1], "ana") != String || kindAt(lines[2], "3") != Number || kindAt(lines[3], "true") != Number {
		t.Errorf("kinds: %v %v", lines[1].Kinds, lines[2].Kinds)
	}
	js := Code("javascript", "// hi\nconst x = await f(\"a\");")
	if kindAt(js[0], "hi") != Comment || kindAt(js[1], "const") != Keyword || kindAt(js[1], "\"a\"") != String {
		t.Errorf("js kinds: %v", js[1].Kinds)
	}
}

func TestTemplatesAndURL(t *testing.T) {
	resolve := func(ref string) bool { return ref == "env.HOST" }
	l := URL("https://api.x.com/users/{id}?token={{env.TOKEN}}&h={{env.HOST}}", resolve)
	if kindAt(l, "api.x.com") != Host || kindAt(l, "{id}") != PathParam || kindAt(l, "token") != QueryKey {
		t.Errorf("url kinds: %v", l.Kinds)
	}
	if kindAt(l, "TOKEN") != TplMissing || kindAt(l, "HOST") != TplOK || kindAt(l, "env.T") != TplScope || kindAt(l, "{{env.T") != TplBrace {
		t.Errorf("template kinds: %v", l.Kinds)
	}
	// Multi-byte text before a template keeps the rune offsets right.
	l = Template("ção {{var.id}}", nil)
	if kindAt(l, "id}}") != TplOK || kindAt(l, "ção") != Plain {
		t.Errorf("unicode offsets: %v", l.Kinds)
	}
}

func TestRender(t *testing.T) {
	l := Template("hello {{env.X}}", nil)
	if got := ansi.Strip(Render(l, 0, 100, -1)); got != "hello {{env.X}}" {
		t.Errorf("render = %q", got)
	}
	if got := ansi.Strip(Render(l, 6, 4, -1)); got != "{{en" {
		t.Errorf("window = %q", got)
	}
	if got := ansi.Strip(Render(l, 0, 100, l.Len())); got != "hello {{env.X}} " {
		t.Errorf("cursor at end = %q", got)
	}
	if out := Render(l, 0, 100, 1); !strings.Contains(out, "\x1b[7") {
		t.Errorf("cursor must be reversed: %q", out)
	}
}
