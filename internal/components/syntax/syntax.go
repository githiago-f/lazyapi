// Package syntax colors text for the editors: code (JSON, JavaScript,
// TypeScript) through chroma, {{env.X}} templates, and URLs. A Line keeps
// one Kind per rune, so a view can show any window of it and draw the
// cursor anywhere without breaking escape sequences.
package syntax

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/githiago-f/lazyapi/internal/config"
)

// Kind is what a rune is (and so how it is drawn).
type Kind uint8

const (
	Plain Kind = iota
	Keyword
	String
	Number
	Comment
	Punct
	Func
	Prop
	Builtin
	TplBrace   // {{ }}
	TplScope   // env. var.
	TplOK      // a name with a value
	TplMissing // a name without a value
	PathParam  // {id}
	QueryKey
	QuerySep // ? & =
	Host
	ErrorMark
)

var styles = map[Kind]lipgloss.Style{
	Plain:      lipgloss.NewStyle().Foreground(lipgloss.Color(config.Text)),
	Keyword:    lipgloss.NewStyle().Foreground(lipgloss.Color(config.Mauve)),
	String:     lipgloss.NewStyle().Foreground(lipgloss.Color(config.Green)),
	Number:     lipgloss.NewStyle().Foreground(lipgloss.Color(config.Peach)),
	Comment:    lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1)).Italic(true),
	Punct:      lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay2)),
	Func:       lipgloss.NewStyle().Foreground(lipgloss.Color(config.Blue)),
	Prop:       lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender)),
	Builtin:    lipgloss.NewStyle().Foreground(lipgloss.Color(config.Yellow)),
	TplBrace:   lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay0)),
	TplScope:   lipgloss.NewStyle().Foreground(lipgloss.Color(config.Teal)),
	TplOK:      lipgloss.NewStyle().Foreground(lipgloss.Color(config.Teal)).Bold(true),
	TplMissing: lipgloss.NewStyle().Foreground(lipgloss.Color(config.Red)).Bold(true).Underline(true),
	PathParam:  lipgloss.NewStyle().Foreground(lipgloss.Color(config.Peach)),
	QueryKey:   lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender)),
	QuerySep:   lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1)),
	Host:       lipgloss.NewStyle().Foreground(lipgloss.Color(config.Subtext0)),
	ErrorMark:  lipgloss.NewStyle().Foreground(lipgloss.Color(config.Red)).Underline(true),
}

// Style returns how a kind is drawn.
func Style(k Kind) lipgloss.Style { return styles[k] }

// Line is one line of text with a Kind per rune.
type Line struct {
	Runes []rune
	Kinds []Kind
}

// Len is the number of runes.
func (l Line) Len() int { return len(l.Runes) }

func plainLine(s string) Line {
	r := []rune(s)
	return Line{Runes: r, Kinds: make([]Kind, len(r))}
}

// Plain splits text into uncolored lines.
func PlainLines(src string) []Line {
	parts := strings.Split(src, "\n")
	out := make([]Line, len(parts))
	for i, p := range parts {
		out[i] = plainLine(p)
	}
	return out
}

// maxHighlight bounds what is tokenized (bigger texts are drawn plain).
const maxHighlight = 256 << 10

// Code colors src in lang ("json", "javascript", "typescript"); unknown
// languages are plain.
func Code(lang, src string) []Line {
	lexer := lexers.Get(lang)
	if lexer == nil || len(src) > maxHighlight {
		return PlainLines(src)
	}
	it, err := chroma.Coalesce(lexer).Tokenise(nil, src)
	if err != nil {
		return PlainLines(src)
	}
	lines := []Line{{}}
	for tok := it(); tok != chroma.EOF; tok = it() {
		k := kindOf(tok.Type, tok.Value)
		for _, r := range tok.Value {
			if r == '\n' {
				lines = append(lines, Line{})
				continue
			}
			cur := &lines[len(lines)-1]
			cur.Runes = append(cur.Runes, r)
			cur.Kinds = append(cur.Kinds, k)
		}
	}
	// chroma adds a final newline when src has none.
	if !strings.HasSuffix(src, "\n") && len(lines) > 1 && lines[len(lines)-1].Len() == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func kindOf(t chroma.TokenType, v string) Kind {
	switch {
	case t == chroma.NameTag: // JSON keys
		return Prop
	case t.InCategory(chroma.Comment):
		return Comment
	case t.InSubCategory(chroma.LiteralString):
		return String
	case t.InSubCategory(chroma.LiteralNumber):
		return Number
	case t == chroma.KeywordConstant:
		return Number
	case t.InCategory(chroma.Keyword):
		return Keyword
	case t == chroma.NameFunction:
		return Func
	case t == chroma.NameBuiltin || t == chroma.NameBuiltinPseudo:
		return Builtin
	case t == chroma.NameOther, t == chroma.NameAttribute, t == chroma.NameProperty:
		return Prop
	case t == chroma.Punctuation || t.InCategory(chroma.Operator):
		return Punct
	}
	return Plain
}

var template = regexp.MustCompile(`\{\{\s*(\$|\w+\.)([^{}\s]+)\s*\}\}`)

// Resolver reports whether a template reference ("env.TOKEN") has a value.
type Resolver func(ref string) bool

// MarkTemplates colors {{scope.name}} references in place: braces dim,
// scope teal, name teal when resolve says it has a value, red otherwise.
// A nil resolve treats every reference as resolved.
func MarkTemplates(lines []Line, resolve Resolver) {
	for i := range lines {
		l := &lines[i]
		s := string(l.Runes)
		for _, m := range template.FindAllStringSubmatchIndex(s, -1) {
			// byte offsets → rune offsets
			start, end := runeIndex(s, m[0]), runeIndex(s, m[1])
			scopeEnd := runeIndex(s, m[3]) // "env." or "$"
			nameStart, nameEnd := runeIndex(s, m[4]), runeIndex(s, m[5])
			ok := resolve == nil || resolve(s[m[2]:m[5]])
			for j := start; j < end; j++ {
				switch {
				case j >= nameStart && j < nameEnd && ok:
					l.Kinds[j] = TplOK
				case j >= nameStart && j < nameEnd:
					l.Kinds[j] = TplMissing
				case j < scopeEnd && j >= start+2:
					l.Kinds[j] = TplScope
				default:
					l.Kinds[j] = TplBrace
				}
			}
		}
	}
}

func runeIndex(s string, byteOff int) int { return len([]rune(s[:byteOff])) }

// URL colors a URL or path: host dim, {param} peach, query keys and
// separators, then templates.
func URL(s string, resolve Resolver) Line {
	l := plainLine(s)
	inQuery := false
	inParam := false
	afterEq := false
	for i, r := range l.Runes {
		switch {
		case r == '?' && !inQuery:
			inQuery = true
			l.Kinds[i] = QuerySep
		case inQuery && (r == '&' || r == '='):
			l.Kinds[i] = QuerySep
			afterEq = r == '='
		case inQuery && !afterEq:
			l.Kinds[i] = QueryKey
		case inQuery:
			l.Kinds[i] = String
		case r == '{' && (i+1 >= len(l.Runes) || l.Runes[i+1] != '{') && (i == 0 || l.Runes[i-1] != '{'):
			inParam = true
			l.Kinds[i] = PathParam
		case inParam:
			l.Kinds[i] = PathParam
			if r == '}' {
				inParam = false
			}
		}
	}
	if idx := strings.Index(s, "://"); idx >= 0 {
		end := len([]rune(s[:idx])) + 3
		for end < len(l.Runes) && l.Runes[end] != '/' && l.Runes[end] != '?' {
			end++
		}
		for i := 0; i < end; i++ {
			l.Kinds[i] = Host
		}
	}
	lines := []Line{l}
	MarkTemplates(lines, resolve)
	return lines[0]
}

// Template colors a plain value with its templates.
func Template(s string, resolve Resolver) Line {
	lines := []Line{plainLine(s)}
	MarkTemplates(lines, resolve)
	return lines[0]
}

var cursorStyle = lipgloss.NewStyle().Reverse(true)

// Render draws runes [from, from+width) of l. cursor (a rune index, or -1)
// is drawn reversed; a cursor at the end of the line is a reversed space.
func Render(l Line, from, width, cursor int) string {
	return RenderSelection(l, from, width, cursor, -1, -1)
}

var selectionBg = lipgloss.Color(config.Surface2)

// RenderSelection is Render with runes [selStart, selEnd) drawn on the
// selection background (selEnd past the line marks the line break too).
func RenderSelection(l Line, from, width, cursor, selStart, selEnd int) string {
	if width <= 0 {
		return ""
	}
	inSel := func(i int) bool { return i >= selStart && i < selEnd }
	style := func(i int) lipgloss.Style {
		st := styles[l.Kinds[i]]
		if inSel(i) {
			st = st.Background(selectionBg)
		}
		return st
	}
	var b strings.Builder
	end := min(l.Len(), from+width)
	i := max(0, from)
	for i < end {
		if i == cursor {
			b.WriteString(cursorStyle.Inherit(styles[l.Kinds[i]]).Render(string(l.Runes[i])))
			i++
			continue
		}
		j := i
		for j < end && l.Kinds[j] == l.Kinds[i] && j != cursor && inSel(j) == inSel(i) {
			j++
		}
		b.WriteString(style(i).Render(string(l.Runes[i:j])))
		i = j
	}
	switch {
	case cursor >= l.Len() && cursor >= from && cursor < from+width:
		b.WriteString(cursorStyle.Render(" "))
	case selEnd > l.Len() && l.Len() >= from && l.Len() < from+width:
		b.WriteString(lipgloss.NewStyle().Background(selectionBg).Render(" ")) // the selected line break
	}
	return b.String()
}
