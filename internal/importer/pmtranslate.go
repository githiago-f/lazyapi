package importer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// translatePostman rewrites a Postman script to the LazyAPI script API
// (test, expect, response, env, vars, lazyapi). pre is true for pre-request
// code, which runs inside before(req). Lines that still use `pm` after the
// translation are commented out with a TODO(postman) marker; if the result
// does not compile, the whole original script is kept as a comment.
func translatePostman(src string, pre bool) (out string, todos int) {
	t := translateExpects(src)
	for _, r := range pmRewrites {
		t = r.re.ReplaceAllString(t, r.to)
	}
	if pre {
		t = pmHeaderSet.ReplaceAllString(t, "req.headers[$2] = $3")
		t = pmHeaderRemove.ReplaceAllString(t, "delete req.headers[$1]")
	}
	lines := strings.Split(t, "\n")
	for i, line := range lines {
		if pmLeft.MatchString(line) {
			lines[i] = "// TODO(postman): " + line
			todos++
		}
	}
	t = strings.Join(lines, "\n")
	check := t
	if pre {
		check = "function before(req) {\n" + t + "\n}"
	}
	if res := api.Transform(check, api.TransformOptions{Loader: api.LoaderJS}); len(res.Errors) > 0 {
		var b strings.Builder
		b.WriteString("// TODO(postman): could not translate this script; original kept below.\n")
		for _, line := range strings.Split(src, "\n") {
			b.WriteString("// " + line + "\n")
		}
		return strings.TrimRight(b.String(), "\n"), 1
	}
	return t, todos
}

var (
	pmLeft         = regexp.MustCompile(`\bpm\.`)
	pmHeaderSet    = regexp.MustCompile(`pm\.request\.headers\.(add|upsert)\(\s*\{\s*key\s*:\s*([^,]+?)\s*,\s*value\s*:\s*(.+?)\s*\}\s*\)`)
	pmHeaderRemove = regexp.MustCompile(`pm\.request\.headers\.remove\(\s*(.+?)\s*\)`)
)

var pmRewrites = []struct {
	re *regexp.Regexp
	to string
}{
	{regexp.MustCompile(`\bpm\.test\(`), "test("},
	{regexp.MustCompile(`\bpm\.response\.code\b`), "response.status"},
	{regexp.MustCompile(`\bpm\.response\.status\b`), "response.statusText"},
	{regexp.MustCompile(`\bpm\.response\.responseTime\b`), "response.time"},
	{regexp.MustCompile(`\bpm\.response\.json\(\)`), "response.json()"},
	{regexp.MustCompile(`\bpm\.response\.text\(\)`), "response.text()"},
	{regexp.MustCompile(`\bpm\.response\.headers\.get\(`), "response.header("},
	{regexp.MustCompile(`\bpm\.response\.headers\.has\(([^)]*)\)`), "(response.header($1) !== null)"},
	{regexp.MustCompile(`\bpm\.environment\.(get|set|has)\(`), "env.$1("},
	{regexp.MustCompile(`\bpm\.iterationData\.(get|has)\(`), "env.$1("},
	{regexp.MustCompile(`\bpm\.(variables|collectionVariables|globals)\.(get|set|has)\(`), "vars.$2("},
	{regexp.MustCompile(`\bpm\.info\.iteration\b`), "lazyapi.iteration"},
	{regexp.MustCompile(`\bpm\.info\.iterationCount\b`), "lazyapi.iterations"},
	{regexp.MustCompile(`\bpm\.request\.url\.toString\(\)`), "request.url"},
	{regexp.MustCompile(`\bpm\.request\.method\b`), "request.method"},
}

// translateExpects rewrites every pm.expect(x).<chai chain> and
// pm.response.to.<chain> it can into native expect() calls. Untranslatable
// chains are left as they are (and later commented out).
func translateExpects(src string) string {
	var b strings.Builder
	for {
		i, subject, isResp := nextAssertion(src)
		if i < 0 {
			b.WriteString(src)
			return b.String()
		}
		b.WriteString(src[:i])
		var rest string
		var start int
		if isResp {
			start = i + len("pm.response")
		} else {
			open := i + len("pm.expect")
			end := matchParen(src, open)
			if end < 0 {
				b.WriteString(src[i:])
				return b.String()
			}
			subject = strings.TrimSpace(src[open+1 : end])
			start = end + 1
		}
		links, n := parseChain(src[start:])
		rest = src[start+n:]
		if code, ok := chaiToExpect(subject, links, isResp); ok {
			b.WriteString(code)
		} else {
			b.WriteString(src[i : start+n])
		}
		src = rest
	}
}

func nextAssertion(src string) (int, string, bool) {
	e := strings.Index(src, "pm.expect(")
	r := strings.Index(src, "pm.response.to.")
	switch {
	case e < 0 && r < 0:
		return -1, "", false
	case r >= 0 && (e < 0 || r < e):
		return r, "response", true
	default:
		return e, "", false
	}
}

// matchParen returns the index of the parenthesis closing the one at open,
// skipping string literals.
func matchParen(s string, open int) int {
	depth := 0
	var quote byte
	for i := open; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

type chainLink struct {
	name string
	args string // "" without a call
	call bool
}

var identRe = regexp.MustCompile(`^\.([A-Za-z_]\w*)`)

func parseChain(s string) ([]chainLink, int) {
	var links []chainLink
	n := 0
	for {
		m := identRe.FindStringSubmatch(s[n:])
		if m == nil {
			return links, n
		}
		link := chainLink{name: m[1]}
		n += len(m[0])
		if n < len(s) && s[n] == '(' {
			end := matchParen(s, n)
			if end < 0 {
				return links, n
			}
			link.call, link.args = true, strings.TrimSpace(s[n+1:end])
			n = end + 1
		}
		links = append(links, link)
	}
}

var chaiNoop = map[string]bool{
	"to": true, "be": true, "been": true, "is": true, "that": true, "which": true, "and": true,
	"has": true, "have": true, "with": true, "at": true, "of": true, "same": true, "does": true,
	"also": true, "still": true, "own": true, "but": true,
}

// chaiToExpect converts one chain. Several assertions in a chain become a
// comma expression, valid both as a statement and inside an arrow function.
func chaiToExpect(subject string, links []chainLink, isResp bool) (string, bool) {
	var out []string
	not, deep := false, false
	emit := func(matcher string) {
		neg := ""
		if not {
			neg = ".not"
		}
		out = append(out, fmt.Sprintf("expect(%s)%s.%s", subject, neg, matcher))
		not = false
	}
	for i := 0; i < len(links); i++ {
		l := links[i]
		switch {
		case l.name == "not" && !l.call:
			not = !not
		case l.name == "deep" && !l.call:
			deep = true
		case (l.name == "a" || l.name == "an") && !l.call:
		case chaiNoop[l.name] && !l.call:
		case isResp && l.name == "status" && l.call:
			if strings.HasPrefix(l.args, "'") || strings.HasPrefix(l.args, `"`) {
				emit("toBe(" + l.args + ")")
				out[len(out)-1] = strings.Replace(out[len(out)-1], "expect(response)", "expect(response.statusText.replace(/^\\d+ /, \"\"))", 1)
			} else {
				emit("toHaveStatus(" + l.args + ")")
			}
		case isResp && l.name == "header" && l.call:
			emit("toHaveHeader(" + l.args + ")")
		case isResp && l.name == "ok" && !l.call:
			saved := subject
			subject = "response.status >= 200 && response.status < 300"
			emit("toBe(true)")
			subject = saved
		case isResp && (l.name == "json" || l.name == "withBody") && !l.call:
			// the body is checked against the contract automatically
		case !isResp && l.name == "ok" && !l.call:
			emit("toBeTruthy()")
		case !isResp && (l.name == "true" || l.name == "false") && !l.call:
			emit("toBe(" + l.name + ")")
		case !isResp && l.name == "null" && !l.call:
			emit("toBeNull()")
		case !isResp && l.name == "undefined" && !l.call:
			emit("toBeUndefined()")
		case !isResp && (l.name == "exist" || l.name == "exists") && !l.call:
			saved := subject
			subject = "(" + subject + ") != null"
			emit("toBe(true)")
			subject = saved
		case !isResp && l.name == "empty" && !l.call:
			emit("toHaveLength(0)")
		case !isResp && (l.name == "a" || l.name == "an") && l.call:
			saved := subject
			switch strings.Trim(l.args, `'"`) {
			case "array":
				subject = "Array.isArray(" + saved + ")"
				emit("toBe(true)")
			case "null":
				emit("toBeNull()")
			default:
				subject = "typeof (" + saved + ")"
				emit("toBe(" + l.args + ")")
			}
			subject = saved
		case !isResp && (l.name == "equal" || l.name == "equals" || l.name == "eq") && l.call:
			if deep {
				emit("toEqual(" + l.args + ")")
			} else {
				emit("toBe(" + l.args + ")")
			}
		case !isResp && (l.name == "eql" || l.name == "eqls") && l.call:
			emit("toEqual(" + l.args + ")")
		case !isResp && (l.name == "above" || l.name == "gt" || l.name == "greaterThan") && l.call:
			emit("toBeGreaterThan(" + l.args + ")")
		case !isResp && (l.name == "least" || l.name == "gte") && l.call:
			emit("toBeGreaterThanOrEqual(" + l.args + ")")
		case !isResp && (l.name == "below" || l.name == "lt" || l.name == "lessThan") && l.call:
			emit("toBeLessThan(" + l.args + ")")
		case !isResp && (l.name == "most" || l.name == "lte") && l.call:
			emit("toBeLessThanOrEqual(" + l.args + ")")
		case !isResp && (l.name == "include" || l.name == "includes" || l.name == "contain" || l.name == "contains") && l.call:
			emit("toContain(" + l.args + ")")
		case !isResp && (l.name == "lengthOf" || l.name == "length") && l.call:
			emit("toHaveLength(" + l.args + ")")
		case !isResp && l.name == "match" && l.call:
			emit("toMatch(" + l.args + ")")
		case !isResp && l.name == "oneOf" && l.call:
			saved := subject
			subject = l.args
			emit("toContain(" + saved + ")")
			subject = saved
		case !isResp && l.name == "property" && l.call:
			// property(p).that.equals(v) → toHaveProperty(p, v); a later
			// assertion applies to the property value.
			args := l.args
			if i+1 < len(links) {
				j := i + 1
				for j < len(links) && chaiNoop[links[j].name] && !links[j].call {
					j++
				}
				if j < len(links) && links[j].call && !strings.Contains(args, ",") &&
					(links[j].name == "equal" || links[j].name == "equals" || links[j].name == "eql") {
					args += ", " + links[j].args
					i = j
				}
			}
			emit("toHaveProperty(" + args + ")")
			if !strings.Contains(args, ",") {
				subject = "(" + subject + ")[" + args + "]"
			}
		default:
			return "", false
		}
	}
	if len(out) == 0 {
		return "", false
	}
	if len(out) == 1 {
		return out[0], true
	}
	return "(" + strings.Join(out, ", ") + ")", true
}
