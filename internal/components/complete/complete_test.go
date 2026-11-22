package complete

import (
	"strings"
	"testing"
)

var vars = []Var{{"env.TOKEN", "s3cr3t"}, {"env.BASE_URL", "http://x"}, {"var.userId", "42"}, {"env.USER", "ana"}}

func labels(r Result) []string {
	var out []string
	for _, it := range r.Items {
		out = append(out, it.Label)
	}
	return out
}

func at(s string) ([]rune, int) {
	r := []rune(s)
	for i, c := range r {
		if c == '|' {
			return append(r[:i:i], r[i+1:]...), i
		}
	}
	return r, len(r)
}

func TestTemplates(t *testing.T) {
	text, cur := at("/users/{{env.U|")
	r, ok := Templates(text, cur, vars)
	if !ok || r.Start != 7 || r.End != cur || labels(r)[0] != "env.USER" {
		t.Errorf("{{env.U → %+v", r)
	}
	text, cur = at("Bearer {{|}}")
	r, _ = Templates(text, cur, vars)
	if len(r.Items) != 4 || r.End != cur+2 {
		t.Errorf("auto-closed braces must be replaced: %+v", r)
	}
	text, cur = at("$tok|")
	r, ok = Templates(text, cur, vars)
	if !ok || r.Start != 0 || r.Items[0].Insert != "{{env.TOKEN}}" || r.Items[0].Detail != "••••••" {
		t.Errorf("$tok → %+v", r)
	}
	for _, s := range []string{"price$5|", "{{env.X}} done|", "plain|", "{{ a b|"} {
		text, cur := at(s)
		if _, ok := Templates(text, cur, vars); ok {
			t.Errorf("%q must not complete", s)
		}
	}
	if Mask("BASE_URL", "http://x") != "http://x" || Mask("x", "") != "(empty)" {
		t.Error("mask")
	}
}

func TestScript(t *testing.T) {
	cases := map[string]string{
		"response.st|":                     "status",
		"expect(response.json().id).toBe|": "toBe()",
		"expect(x).not.toC|":               "toContain()",
		"lazyapi.s|":                       "send()",
		"pm.response.co|":                  "code",
		"const a = 1;\nres|":               "response",
		"  expect(r).toHave|":              "toHaveProperty()",
	}
	for src, want := range cases {
		text, cur := at(src)
		r, ok := Script(text, cur)
		if !ok || labels(r)[0] != want {
			t.Errorf("%q → %v", src, labels(r))
		}
	}
	for _, s := range []string{"// response.st|", "\"res|", "foo.bar|", "x|"} {
		text, cur := at(s)
		if r, ok := Script(text, cur); ok {
			t.Errorf("%q must not complete: %v", s, labels(r))
		}
	}
	text, cur := at("response.json|")
	r, _ := Script(text, cur)
	if r.Items[0].Insert != "json()" || r.Items[0].CursorBack != 1 {
		t.Errorf("methods insert () with the cursor inside: %+v", r.Items[0])
	}
}

func TestSignature(t *testing.T) {
	cases := map[string]string{
		`lazyapi.send("getUser", |`:                    "lazyapi.send(target",
		`expect(response.json().id).toBe(|`:            "toBe(expected)",
		`expect(response.json().id|`:                   "expect(actual",
		`test("x", () => { expect(a).toHaveProperty(|`: "toHaveProperty(path",
		`env.get(|`:        "env.get(name",
		`foo(|`:            "",
		`response.status|`: "",
	}
	for src, want := range cases {
		text, cur := at(src)
		got := Signature(text, cur)
		if want == "" && got != "" || want != "" && !strings.HasPrefix(got, want) {
			t.Errorf("%q → %q, want prefix %q", src, got, want)
		}
	}
}
