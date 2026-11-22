// Package responses renders an HTTP exchange (status, body, headers, checks,
// console) for the request editor and the request list preview.
package responses

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"gopkg.in/yaml.v3"
)

// highlightLimit skips syntax highlighting for very large bodies (it is the
// slowest part of rendering and adds little on huge payloads).
const highlightLimit = 512 << 10

var (
	chromaStyle     = styles.Get("catppuccin-mocha")
	chromaFormatter = formatters.Get("terminal16m")
)

// kind guesses the body language from the Content-Type, falling back to
// sniffing the body.
func kind(contentType, body string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "json"):
		return "json"
	case strings.Contains(ct, "yaml"):
		return "yaml"
	case strings.Contains(ct, "html"):
		return "html"
	case strings.Contains(ct, "xml"):
		return "xml"
	case strings.Contains(ct, "javascript"):
		return "javascript"
	case strings.Contains(ct, "css"):
		return "css"
	}
	trimmed := strings.TrimSpace(body)
	if trimmed != "" && (trimmed[0] == '{' || trimmed[0] == '[') && json.Valid([]byte(trimmed)) {
		return "json"
	}
	if strings.HasPrefix(trimmed, "<") {
		return "xml"
	}
	return "text"
}

// Pretty re-indents JSON and YAML bodies; other bodies are returned as-is.
// JSON keeps key order and is not HTML-escaped.
func Pretty(contentType, body string) string {
	switch kind(contentType, body) {
	case "json":
		var buf bytes.Buffer
		if err := json.Indent(&buf, []byte(strings.TrimSpace(body)), "", "  "); err == nil {
			return buf.String()
		}
	case "yaml":
		var v any
		if err := yaml.Unmarshal([]byte(body), &v); err == nil {
			if out, err := yaml.Marshal(v); err == nil {
				return strings.TrimRight(string(out), "\n")
			}
		}
	}
	return body
}

// Highlight returns body with ANSI syntax colors for its language.
func Highlight(contentType, body string) string {
	lang := kind(contentType, body)
	if lang == "text" || len(body) > highlightLimit || chromaFormatter == nil || chromaStyle == nil {
		return body
	}
	lexer := lexers.Get(lang)
	if lexer == nil {
		return body
	}
	lexer = chroma.Coalesce(lexer)
	it, err := lexer.Tokenise(nil, body)
	if err != nil {
		return body
	}
	var buf bytes.Buffer
	if err := chromaFormatter.Format(&buf, chromaStyle, it); err != nil {
		return body
	}
	return strings.TrimRight(buf.String(), "\n")
}

// HumanSize renders a byte count like "1.2 KB".
func HumanSize(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}
