package components

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/env"
)

// TemplateNames, when set, lists the template names ("env.TOKEN",
// "var.id") offered as inline completions after "{{" in every Field. The
// TUI points it at the active environment. → accepts a completion.
var TemplateNames func() []string

type Field struct {
	tea.Model
	TextInput textinput.Model
	Focused   bool
	Style     lipgloss.Style
}

var defaultStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder())

func InitField(placeholder, value string) Field {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = ""
	ti.ShowSuggestions = true
	ti.KeyMap.AcceptSuggestion = key.NewBinding(key.WithKeys("right"))
	ti.KeyMap.NextSuggestion = key.NewBinding(key.WithKeys("ctrl+down"))
	ti.KeyMap.PrevSuggestion = key.NewBinding(key.WithKeys("ctrl+up"))
	ti.SetValue(value)

	ti.Focus()

	return Field{TextInput: ti}
}

func (f Field) Value() string {
	return f.TextInput.Value()
}

func (f *Field) SetValue(s string) {
	f.TextInput.SetValue(s)
}

func (f Field) Init() tea.Cmd {
	return nil
}

func (f Field) View() tea.View {
	return tea.NewView(f.Style.Inherit(defaultStyle).Render(f.TextInput.View()))
}

func (f Field) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	f.TextInput, cmd = f.TextInput.Update(msg)
	f.complete()
	return f, cmd
}

// complete refreshes the template completions for the current value.
func (f *Field) complete() {
	if TemplateNames == nil || !f.TextInput.ShowSuggestions {
		return
	}
	f.TextInput.SetSuggestions(env.Completions(f.TextInput.Value(), TemplateNames()))
}
