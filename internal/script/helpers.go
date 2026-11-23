package script

import (
	"regexp"
	"sort"
)

var exportedFunc = regexp.MustCompile(`(?m)^\s*export\s+(?:(?:async\s+)?function\s*\*?\s*([A-Za-z_$][\w$]*)|(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=)`)

// HelperNames lists the functions a test script exports that can be used
// as {{$name}} (every export except before and after). It reads the source
// without running it, for completions; exports re-exported from other
// files are not seen.
func HelperNames(src string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range exportedFunc.FindAllStringSubmatch(src, -1) {
		name := m[1] + m[2]
		if name == "" || name == "before" || name == "after" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
