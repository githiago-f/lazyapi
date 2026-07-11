package editor

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
)

type body struct {
	active bool
	editor components.Text
}

func (b *body) SetActive(active bool) {
	b.active = active
	if !active {
		b.editor.Blur()
	} else {
		b.editor.Focus()
	}
}

func (b *body) IsActive() bool {
	return b.active
}

func BodyTab() *body {
	editor := components.NewTextarea("Json data", "")

	return &body{editor: editor}
}

func (b *body) SetValue(s string) {
	b.editor.SetValue(s)
}

func (b *body) Value() string {
	return b.editor.Value()
}

func (b body) HelpBindings() []key.Binding {
	return []key.Binding{
		config.DefaultKeyMap.Back,
	}
}

func (b body) Init() tea.Cmd {
	return nil
}

func (b body) View() string {
	b.editor.Style = b.editor.Style.UnsetBorderForeground()
	if b.active {
		b.editor.Style = b.editor.Style.BorderForeground(config.DefaultConfig.PrimaryColor())
	}
	return b.editor.View()
}

func (b body) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		bodyHeight := max(1, msg.Height-5)
		b.editor.Style = b.editor.Style.
			Width(msg.Width - 1).
			Height(bodyHeight)
		b.editor.TextArea.SetWidth(max(0, msg.Width-3))
		b.editor.TextArea.SetHeight(bodyHeight)

	case tabs.SetActiveTabMsg:
		b.SetActive(msg.Active)
		return &b, nil
	}

	if b.active {
		model, cmd := b.editor.Update(msg)
		b.editor, _ = model.(components.Text)
		return &b, cmd
	}

	return &b, nil
}
