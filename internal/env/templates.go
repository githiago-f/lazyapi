package env

import (
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
