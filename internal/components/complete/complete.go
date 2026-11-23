// Package complete finds completions at the cursor of a text field or code
// editor: {{env.X}} / $X variables and the script API (response., expect().,
// lazyapi., env., …). It is pure: the components render the menu.
package complete

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/githiago-f/lazyapi/internal/fake"
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
	Ref   string // "env.TOKEN", "var.userId" or "$randomEmail"
	Value string
	// Detail replaces the (masked) value in the menu, e.g. the description
	// of a dynamic variable.
	Detail string
}

func varDetail(v Var) string {
	if v.Detail != "" {
		return v.Detail
	}
	return Mask(v.Ref, v.Value)
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
			return Item{Label: v.Ref, Insert: "{{" + v.Ref + "}}", Detail: varDetail(v)}
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
		return Item{Label: v.Ref, Insert: "{{" + v.Ref + "}}", Detail: varDetail(v)}
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
		if strings.HasPrefix(ref, "$") {
			name = ref[1:]
		}
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
		// env/var before the (many) dynamic variables
		if da, db := strings.HasPrefix(hits[a].v.Ref, "$"), strings.HasPrefix(hits[b].v.Ref, "$"); da != db {
			return db
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

func init() {
	f := []member{{"int()", "(min, max) → integer"}, {"float()", "(min, max) → number"}, {"pick()", "(list) → element"}, {"get()", "(\"randomEmail\") → string"}}
	for _, v := range fake.Vars() {
		f = append(f, member{fake.JSName(v.Name) + "()", v.Description})
	}
	api["faker"] = f
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
	{"env", "environment"}, {"vars", "run variables"}, {"lazyapi", "send, iteration"}, {"faker", "fake data"}, {"console", "log"},
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
// "response", "expect()" (for expect(...)), "expect().not".
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

// signatures of the script API, by call name.
var signatures = map[string]string{
	"test":                   "test(name: string, fn: () => void)",
	"expect":                 "expect(actual: unknown) → matchers",
	"lazyapi.send":           "lazyapi.send(target: string, options?: {params, query, headers, body}) → LazyResponse",
	"env.get":                "env.get(name: string) → string | undefined",
	"env.set":                "env.set(name: string, value: string)",
	"env.has":                "env.has(name: string) → boolean",
	"vars.get":               "vars.get(name: string) → string | undefined",
	"vars.set":               "vars.set(name: string, value: string)",
	"vars.has":               "vars.has(name: string) → boolean",
	"response.json":          "response.json<T>() → T",
	"response.header":        "response.header(name: string) → string | undefined",
	"request.json":           "request.json<T>() → T",
	"console.log":            "console.log(...values)",
	"faker.int":              "faker.int(min = 0, max = 1000) → number",
	"faker.float":            "faker.float(min = 0, max = 1) → number",
	"faker.pick":             "faker.pick<T>(list: T[]) → T",
	"faker.get":              "faker.get(name: string) → string — a {{$name}} built-in",
	"toBe":                   "toBe(expected) — strict equality (===)",
	"toEqual":                "toEqual(expected) — deep equality",
	"toContain":              "toContain(item) — substring or array element",
	"toMatch":                "toMatch(pattern: RegExp | string)",
	"toHaveProperty":         "toHaveProperty(path: string, value?)",
	"toHaveLength":           "toHaveLength(n: number)",
	"toHaveStatus":           "toHaveStatus(code: number)",
	"toHaveHeader":           "toHaveHeader(name: string, value?: string)",
	"toMatchSchema":          "toMatchSchema(schema?) — the operation's response schema by default",
	"toBeGreaterThan":        "toBeGreaterThan(n: number)",
	"toBeGreaterThanOrEqual": "toBeGreaterThanOrEqual(n: number)",
	"toBeLessThan":           "toBeLessThan(n: number)",
	"toBeLessThanOrEqual":    "toBeLessThanOrEqual(n: number)",
	"toThrow":                "toThrow(message?: string | RegExp)",
	"before":                 "export function before(req) — change the request before it is sent",
	"after":                  "export function after(res) — clean up after the tests",
}

// Signature returns the signature of the innermost call the cursor is in
// ("" when none is known): inside lazyapi.send(|) it is lazyapi.send's.
func Signature(text []rune, cursor int) string {
	if cursor > len(text) {
		return ""
	}
	depth := 0
	for i := cursor - 1; i >= max(0, cursor-2000); i-- {
		switch text[i] {
		case ')':
			depth++
		case '(':
			if depth > 0 {
				depth--
				continue
			}
			j := i
			for j > 0 && (isWord(text[j-1]) || text[j-1] == '.') && text[j-1] != '-' {
				j--
			}
			name := string(text[j:i])
			if sig, ok := signatures[name]; ok {
				return sig
			}
			// expect(x).toBe( → the last member
			if dot := strings.LastIndex(name, "."); dot >= 0 {
				if sig, ok := signatures[name[dot+1:]]; ok {
					return sig
				}
			}
			return ""
		}
	}
	return ""
}
