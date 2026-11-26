package components

import (
	"regexp"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/components/complete"
	"github.com/githiago-f/lazyapi/internal/components/syntax"
	"github.com/githiago-f/lazyapi/internal/config"
)

// Templates is where fields and editors get variables from.
type Templates interface {
	// Vars are offered after "{{" or "$" (the env file and the environment's
	// vars, not the whole process environment).
	Vars() []complete.Var
	// Has reports whether a reference ("env.TOKEN") has a value.
	Has(ref string) bool
}

// TemplateSource is set by the TUI to the active environment (nil: no
// completions, every reference drawn as resolved).
var TemplateSource Templates

// Resolver colors references by TemplateSource.
func Resolver() syntax.Resolver {
	if TemplateSource == nil {
		return nil
	}
	return TemplateSource.Has
}

func templateVars() []complete.Var {
	if TemplateSource == nil {
		return nil
	}
	return TemplateSource.Vars()
}

// FieldKind selects how a Field's value is colored.
type FieldKind int

const (
	TemplateField FieldKind = iota // values: {{env.X}} highlighted
	URLField                       // paths and URLs: params, query, templates
)

// Field is a one-line input with highlighting, horizontal scrolling and a
// completion menu. TextInput holds the value and the cursor.
type Field struct {
	tea.Model
	TextInput textinput.Model
	Focused   bool
	Style     lipgloss.Style
	Kind      FieldKind
	// Prefix is drawn before the value inside the box (e.g. a file marker).
	Prefix string

	offset    int // first visible rune
	menu      *Completion
	dismissed string // value at which the menu was closed with esc
}

var defaultStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(config.Surface2))

var placeholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay0)).Italic(true)

func InitField(placeholder, value string) Field {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = ""
	// ctrl+v is ours: PasteCmd also reaches the Windows clipboard from WSL.
	ti.KeyMap.Paste = key.NewBinding(key.WithDisabled())
	ti.SetValue(value)
	ti.Focus()
	return Field{TextInput: ti}
}

func (f Field) Value() string { return f.TextInput.Value() }

func (f *Field) SetValue(s string) {
	f.TextInput.SetValue(s)
	f.menu = nil
}

// Completing reports whether the completion menu is open: tab, esc and the
// arrows belong to the field, so containers must not use them to move.
func (f Field) Completing() bool { return f.menu != nil }

func (f Field) Init() tea.Cmd { return nil }

func (f Field) View() tea.View {
	return tea.NewView(f.Style.Inherit(defaultStyle).Render(f.Content(f.TextInput.EchoMode == textinput.EchoPassword)))
}

var prefixStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Accent)).Bold(true)

// Content draws the value (or the placeholder) without the box.
func (f Field) Content(masked bool) string {
	if f.Prefix == "" {
		return f.content(masked, f.width())
	}
	pw := lipgloss.Width(f.Prefix)
	return prefixStyle.Render(f.Prefix) + f.content(masked, max(1, f.width()-pw))
}

func (f Field) content(masked bool, width int) string {
	cursor := -1
	if f.TextInput.Focused() {
		cursor = f.TextInput.Position()
	}
	value := f.TextInput.Value()
	if value == "" {
		ph := []rune(f.TextInput.Placeholder)
		if len(ph) > width {
			ph = ph[:width]
		}
		if cursor == 0 && len(ph) > 0 {
			return syntax.Render(syntax.Line{Runes: ph[:1], Kinds: []syntax.Kind{syntax.Comment}}, 0, 1, 0) + placeholderStyle.Render(string(ph[1:]))
		}
		if cursor == 0 {
			return syntax.Render(syntax.Line{}, 0, width, 0)
		}
		return placeholderStyle.Render(string(ph))
	}
	var line syntax.Line
	switch {
	case masked && !onlyReferences(value):
		r := make([]rune, len([]rune(value)))
		for i := range r {
			r[i] = '•'
		}
		line = syntax.Line{Runes: r, Kinds: make([]syntax.Kind, len(r))}
	case f.Kind == URLField:
		line = syntax.URL(value, Resolver())
	default:
		line = syntax.Template(value, Resolver())
	}
	return syntax.Render(line, clampOffset(f.offset, cursor, width, line.Len()), width, cursor)
}

var referencesOnly = regexp.MustCompile(`^(\s*\{\{[^{}]+\}\}\s*)+$`)

// onlyReferences reports whether a value is just {{env.X}} references: it
// holds no secret, so a password field may show it.
func onlyReferences(v string) bool { return referencesOnly.MatchString(v) }

// width is the text width: the input's, else the box's minus its border.
func (f Field) width() int {
	if w := f.TextInput.Width(); w > 0 {
		return w
	}
	if w := f.Style.GetWidth() - 2; w > 0 {
		return w
	}
	return 40
}

// clampOffset keeps the cursor visible.
func clampOffset(offset, cursor, width, n int) int {
	if cursor >= 0 {
		if cursor < offset {
			offset = cursor
		}
		if cursor >= offset+width {
			offset = cursor - width + 1
		}
	}
	return max(0, min(offset, max(0, n+1-width)))
}

// ClickAt moves the cursor to the column clicked inside the box (0 is the
// left border) and closes the menu.
func (f *Field) ClickAt(col int) {
	value := []rune(f.TextInput.Value())
	off := clampOffset(f.offset, f.TextInput.Position(), f.width(), len(value))
	col -= lipgloss.Width(f.Prefix)
	f.TextInput.SetCursor(max(0, min(len(value), off+col-1)))
	f.offset = off
	f.menu = nil
}

// MenuView is the open completion menu ("" when closed); containers place it
// under the field.
func (f Field) MenuView(width int) string {
	if f.menu == nil {
		return ""
	}
	return f.menu.View(width)
}

// CanCopy reports whether ctrl+c copies the field (it has a value) rather
// than quitting.
func (f Field) CanCopy() bool { return f.TextInput.Value() != "" }

func (f Field) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			if !f.CanCopy() {
				return f, nil
			}
			return f, CopyCmd(f.TextInput.Value(), "✓ Copied")
		case "ctrl+v":
			return f, PasteCmd()
		}
	case tea.PasteMsg:
		// One line: line breaks become spaces; a dropped file is its path.
		text := CleanDrop(msg.Content)
		text = strings.TrimRight(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(text), " ")
		var cmd tea.Cmd
		f.TextInput, cmd = f.TextInput.Update(tea.PasteMsg{Content: text})
		f.dismissed = ""
		f.refresh()
		return f, cmd
	}
	if k, ok := msg.(tea.KeyPressMsg); ok && f.menu != nil {
		switch k.String() {
		case "up", "ctrl+p":
			f.menu.Move(-1)
			return f, nil
		case "down", "ctrl+n":
			f.menu.Move(1)
			return f, nil
		case "tab", "enter":
			text, cur := f.menu.Apply([]rune(f.TextInput.Value()))
			f.TextInput.SetValue(string(text))
			f.TextInput.SetCursor(cur)
			f.menu = nil
			f.refresh()
			return f, nil
		case "esc":
			f.menu = nil
			f.dismissed = f.TextInput.Value()
			return f, nil
		}
	}
	before := f.TextInput.Value()
	var cmd tea.Cmd
	f.TextInput, cmd = f.TextInput.Update(msg)
	if _, ok := msg.(tea.KeyPressMsg); ok {
		if f.TextInput.Value() != before {
			f.dismissed = ""
		}
		f.refresh()
	}
	return f, cmd
}

// refresh recomputes the menu and the scroll offset after a change.
func (f *Field) refresh() {
	cursor := f.TextInput.Position()
	f.offset = clampOffset(f.offset, cursor, f.width(), len([]rune(f.TextInput.Value())))
	f.menu = nil
	if f.dismissed == f.TextInput.Value() && f.dismissed != "" {
		return
	}
	if res, ok := complete.Templates([]rune(f.TextInput.Value()), cursor, templateVars()); ok {
		f.menu = &Completion{Result: res}
	}
}
