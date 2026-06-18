package env

import "regexp"

var templatePattern = regexp.MustCompile(`\{\{(\w+)\.([^}]+)\}\}`)

// Context holds the variable scopes available for template resolution.
type Context struct {
	Vars map[string]string // test-scope variables
	Env  map[string]string // loaded environment variables
}

// Resolve replaces every {{type.name}} occurrence in input with the
// corresponding value from ctx. Unresolved or unknown variables resolve
// to the empty string.
//
// Supported prefixes:
//   - var     → ctx.Vars
//   - env     → ctx.Env
//   - lazyapi → "" (reserved for future use)
func Resolve(input string, ctx Context) string {
	return templatePattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := templatePattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		prefix := parts[1]
		name := parts[2]

		switch prefix {
		case "var":
			if ctx.Vars != nil {
				if val, ok := ctx.Vars[name]; ok {
					return val
				}
			}
			return ""
		case "env":
			if ctx.Env != nil {
				if val, ok := ctx.Env[name]; ok {
					return val
				}
			}
			return ""
		case "lazyapi":
			return ""
		default:
			return match
		}
	})
}
