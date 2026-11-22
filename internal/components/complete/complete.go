// Package complete finds completions at the cursor of a text field or code
// editor: {{env.X}} / $X variables and the script API (response., expect().,
// lazyapi., env., pm., …). It is pure: the components render the menu.
package complete

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Item is one completion.
type Item struct {
	Label  string // shown in the menu
	Insert string // replaces Result.Start..Result.End
	Detail string // shown dimmed (type, masked value)
	// CursorBack moves the cursor left after inserting (inside "()").
	CursorBack int
}

// Result is what to replace and the candidates.
type Result struct {
	Start, End int // rune offsets in the text
	Items      []Item
}

// Var is a template variable that can be completed.
type Var struct {
	Ref   string // "env.TOKEN" or "var.userId"
	Value string
}

var sensitive = regexp.MustCompile(`(?i)(token|secret|pass|key|auth|cookie|credential|private)`)

// Mask hides values that look like credentials.
func Mask(name, value string) string {
	switch {
	case value == "":
		return "(empty)"
	case sensitive.MatchString(name):
		return "••••••"
	case len([]rune(value)) > 28:
		return string([]rune(value)[:27]) + "…"
	}
	return value
}

func isWord(r rune) bool { return r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// Templates completes variables: inside an open "{{" (up to the cursor) or
// after "$name" (a shortcut that becomes {{env.NAME}}).
func Templates(text []rune, cursor int, vars []Var) (Result, bool) {
	if cursor < 0 || cursor > len(text) || len(vars) == 0 {
		return Result{}, false
	}
	before := string(text[:cursor])

	if open := strings.LastIndex(before, "{{"); open >= 0 && !strings.Contains(before[open:], "}}") {
		typed := before[open+2:]
		if strings.ContainsAny(typed, " \t\"'") {
			return Result{}, false
		}
		start := len([]rune(before[:open]))
		end := cursor
		if strings.HasPrefix(string(text[cursor:]), "}}") {
			end += 2 // auto-closed braces are replaced too
		}
		items := rank(vars, strings.TrimSpace(typed), func(v Var) Item {
			return Item{Label: v.Ref, Insert: "{{" + v.Ref + "}}", Detail: Mask(v.Ref, v.Value)}
		})
		return Result{Start: start, End: end, Items: items}, len(items) > 0
	}

	// $name: the word right before the cursor, starting with $.
	i := cursor
	for i > 0 && isWord(text[i-1]) {
		i--
	}
	if i == 0 || text[i-1] != '$' || (i >= 2 && isWord(text[i-2])) {
		return Result{}, false
	}
	typed := string(text[i:cursor])
	items := rank(vars, typed, func(v Var) Item {
		return Item{Label: v.Ref, Insert: "{{" + v.Ref + "}}", Detail: Mask(v.Ref, v.Value)}
	})
	return Result{Start: i - 1, End: cursor, Items: items}, len(items) > 0
}

// rank keeps the vars matching typed (case-insensitive): name prefix first,
// then substring, then alphabetical.
func rank(vars []Var, typed string, item func(Var) Item) []Item {
	typed = strings.ToLower(typed)
	type scored struct {
		v     Var
		score int
	}
	var hits []scored
	for _, v := range vars {
		ref := strings.ToLower(v.Ref)
		_, name, _ := strings.Cut(ref, ".")
		switch {
		case typed == "":
			hits = append(hits, scored{v, 1})
		case strings.HasPrefix(ref, typed), strings.HasPrefix(name, typed):
			hits = append(hits, scored{v, 0})
		case strings.Contains(ref, typed):
			hits = append(hits, scored{v, 1})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].score != hits[b].score {
			return hits[a].score < hits[b].score
		}
		return hits[a].v.Ref < hits[b].v.Ref
	})
	out := make([]Item, len(hits))
	for i, h := range hits {
		out[i] = item(h.v)
	}
	return out
}

// member is one property or method of the script API.
type member struct{ name, detail string }

func (m member) item() Item {
	if strings.HasSuffix(m.name, "()") {
		return Item{Label: m.name, Insert: m.name, Detail: m.detail, CursorBack: 1}
	}
	return Item{Label: m.name, Insert: m.name, Detail: m.detail}
}

var api = map[string][]member{
	"response": {{"status", "number"}, {"statusText", "string"}, {"headers", "Record<string,string>"}, {"body", "string"},
		{"time", "ms"}, {"json()", "parsed body"}, {"text()", "string"}, {"header()", "header value"}},
	"request": {{"method", "string"}, {"url", "string"}, {"path", "string"}, {"headers", "Record<string,string>"},
		{"query", "Record<string,string>"}, {"params", "Record<string,string>"}, {"body", "string"}, {"json()", "parsed body"}},
	"env":     {{"get()", "string | undefined"}, {"set()", "void"}, {"has()", "boolean"}, {"all()", "Record<string,string>"}},
	"vars":    {{"get()", "string | undefined"}, {"set()", "void"}, {"has()", "boolean"}, {"all()", "Record<string,string>"}},
	"lazyapi": {{"send()", "(operation, options?) → response"}, {"iteration", "0-based data row"}, {"iterations", "rows"}},
	"console": {{"log()", ""}, {"info()", ""}, {"warn()", ""}, {"error()", ""}},
	"pm": {{"test()", "(name, fn)"}, {"expect()", "chai-style"}, {"response", "pm response"}, {"request", "pm request"},
		{"environment", "scope"}, {"variables", "scope"}, {"collectionVariables", "scope"}, {"globals", "scope"},
		{"iterationData", "scope"}, {"sendRequest()", "(req, callback)"}, {"info", "iteration, eventName"}},
	"pm.response": {{"code", "number"}, {"status", "string"}, {"json()", "parsed body"}, {"text()", "string"},
		{"headers", "list"}, {"responseTime", "ms"}, {"to", "assertions"}},
}

var matchers = []member{
	{"toBe()", "strict equality"}, {"toEqual()", "deep equality"}, {"toBeTruthy()", ""}, {"toBeFalsy()", ""},
	{"toBeNull()", ""}, {"toBeUndefined()", ""}, {"toBeDefined()", ""}, {"toContain()", "string or array"},
	{"toMatch()", "regexp"}, {"toHaveProperty()", "(path, value?)"}, {"toHaveLength()", ""},
	{"toBeGreaterThan()", ""}, {"toBeGreaterThanOrEqual()", ""}, {"toBeLessThan()", ""}, {"toBeLessThanOrEqual()", ""},
	{"toHaveStatus()", "response status"}, {"toHaveHeader()", "(name, value?)"}, {"toMatchSchema()", "OpenAPI contract"},
	{"toThrow()", ""}, {"not", "negate"},
}

var globals = []member{
	{"test()", "(name, fn)"}, {"expect()", "(actual)"}, {"response", "the response"}, {"request", "the request"},
	{"env", "environment"}, {"vars", "run variables"}, {"lazyapi", "send, iteration"}, {"console", "log"}, {"pm", "Postman API"},
}

// Script completes the script API after "object." and "expect(…).", and
// global names while typing an identifier at the start of an expression.
func Script(text []rune, cursor int) (Result, bool) {
	if cursor < 0 || cursor > len(text) {
		return Result{}, false
	}
	start := cursor
	for start > 0 && (isWord(text[start-1]) || text[start-1] == '$') && text[start-1] != '-' {
		start--
	}
	typed := string(text[start:cursor])

	if inStringOrComment(text, start) {
		return Result{}, false
	}
	var cands []member
	if start > 0 && text[start-1] == '.' {
		obj := objectBefore(text, start-1)
		switch obj {
		case "expect()", "expect().not":
			cands = matchers
		default:
			cands = api[obj]
		}
	} else {
		if typed == "" {
			return Result{}, false
		}
		if start > 0 && (isWord(text[start-1]) || text[start-1] == '"' || text[start-1] == '\'') {
			return Result{}, false
		}
		cands = globals
	}
	var items []Item
	lower := strings.ToLower(typed)
	for _, m := range cands {
		if strings.HasPrefix(strings.ToLower(m.name), lower) && m.name != typed {
			items = append(items, m.item())
		}
	}
	return Result{Start: start, End: cursor, Items: items}, len(items) > 0
}

// objectBefore names the expression ending right before the dot at dot:
// "response", "pm.response", "expect()" (for expect(...)), "expect().not".
func objectBefore(text []rune, dot int) string {
	i := dot
	if i > 0 && text[i-1] == ')' {
		depth := 0
		j := i - 1
		for ; j >= 0; j-- {
			switch text[j] {
			case ')':
				depth++
			case '(':
				depth--
			}
			if depth == 0 {
				break
			}
		}
		k := j
		for k > 0 && isWord(text[k-1]) {
			k--
		}
		if j > 0 && string(text[k:j]) == "expect" {
			return "expect()"
		}
		return ""
	}
	k := i
	for k > 0 && (isWord(text[k-1]) || text[k-1] == '.') {
		k--
	}
	name := string(text[k:i])
	// ").not" → what is before ").": expect(...).not
	if rest, ok := strings.CutPrefix(name, "."); ok && k > 0 && text[k-1] == ')' {
		if objectBefore(text, k) == "expect()" && rest == "not" {
			return "expect().not"
		}
		return ""
	}
	return name
}

// inStringOrComment is a cheap check for the current line only.
func inStringOrComment(text []rune, pos int) bool {
	lineStart := pos
	for lineStart > 0 && text[lineStart-1] != '\n' {
		lineStart--
	}
	var quote rune
	for i := lineStart; i < pos; i++ {
		r := text[i]
		if quote != 0 {
			switch r {
			case '\\':
				i++
			case quote:
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'', '`':
			quote = r
		case '/':
			if i+1 < pos && text[i+1] == '/' {
				return true
			}
		}
	}
	return quote != 0
}
