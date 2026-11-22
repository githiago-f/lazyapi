package env

import (
	"sort"
	"strings"
)

// Refs returns the template references in s as "scope.name" (e.g.
// "env.TOKEN"), in order of appearance, without duplicates.
func Refs(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range templatePattern.FindAllStringSubmatch(s, -1) {
		ref := m[1] + "." + m[2]
		if !seen[ref] {
			seen[ref] = true
			out = append(out, ref)
		}
	}
	return out
}

// Missing returns the env and var references in inputs that have no value
// in ctx (they would be sent as empty strings).
func Missing(ctx Context, inputs ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, in := range inputs {
		for _, ref := range Refs(in) {
			scope, name, _ := strings.Cut(ref, ".")
			var ok bool
			switch scope {
			case "env":
				_, ok = ctx.Env[name]
			case "var":
				_, ok = ctx.Vars[name]
			default:
				continue
			}
			if !ok && !seen[ref] {
				seen[ref] = true
				out = append(out, ref)
			}
		}
	}
	return out
}

// Names lists the template names ctx can resolve ("env.X", "var.Y"), sorted.
func Names(ctx Context) []string {
	out := make([]string, 0, len(ctx.Env)+len(ctx.Vars))
	for k := range ctx.Env {
		out = append(out, "env."+k)
	}
	for k := range ctx.Vars {
		out = append(out, "var."+k)
	}
	sort.Strings(out)
	return out
}

// Completions returns full-value suggestions for an input whose text ends
// inside an open template ("/users/{{env.U" → "/users/{{env.USER_ID}}").
// names are the candidates ("env.USER_ID"); scopes are offered while no
// scope has been typed yet.
func Completions(value string, names []string) []string {
	open := strings.LastIndex(value, "{{")
	if open < 0 || strings.Contains(value[open:], "}}") {
		return nil
	}
	prefix, typed := value[:open], strings.ToLower(value[open+2:])
	var out []string
	for _, n := range names {
		if strings.HasPrefix(strings.ToLower(n), typed) {
			out = append(out, prefix+"{{"+n+"}}")
		}
	}
	if !strings.Contains(typed, ".") {
		for _, scope := range []string{"env.", "var."} {
			if strings.HasPrefix(scope, typed) {
				out = append(out, prefix+"{{"+scope)
			}
		}
	}
	return out
}
