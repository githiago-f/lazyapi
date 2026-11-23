package env

import (
	"regexp"
	"strings"
)

// dynamicPattern matches {{$name}}: a dynamic value (faker or a helper
// exported by the request's script), generated when the request is sent.
var dynamicPattern = regexp.MustCompile(`\{\{\s*\$(\w+)\s*\}\}`)

// DynamicFunc returns the value of one {{$name}} occurrence.
type DynamicFunc func(name string) (string, error)

// DynamicRefs returns the {{$name}} names in inputs (without "$"), in order
// of appearance, without duplicates.
func DynamicRefs(inputs ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, in := range inputs {
		if !strings.Contains(in, "{{") {
			continue
		}
		for _, m := range dynamicPattern.FindAllStringSubmatch(in, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				out = append(out, m[1])
			}
		}
	}
	return out
}

// ResolveDynamic replaces every {{$name}} in input with fn(name); each
// occurrence gets its own value. The first error stops the replacement.
func ResolveDynamic(input string, fn DynamicFunc) (string, error) {
	if !strings.Contains(input, "{{") {
		return input, nil
	}
	var firstErr error
	out := dynamicPattern.ReplaceAllStringFunc(input, func(m string) string {
		if firstErr != nil {
			return m
		}
		v, err := fn(dynamicPattern.FindStringSubmatch(m)[1])
		if err != nil {
			firstErr = err
			return m
		}
		return v
	})
	return out, firstErr
}
