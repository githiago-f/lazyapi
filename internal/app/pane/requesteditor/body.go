package requesteditor

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
)

type body struct {
	active bool
	editor components.Text
}

func (b body) SetActive(active bool) tabs.StatefulInputBase {
	b.active = active
	return b
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

func (b body) Init() tea.Cmd {
	return nil
}

func (b body) View() string {
	if b.active {
		b.editor.Style = b.editor.Style.BorderForeground(config.DefaultConfig.PrimaryColor())
	}
	return b.editor.View()
}

func (b body) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.editor.Style = b.editor.Style.
			Width(msg.Width - 1).
			Height(msg.Height - 4)
	}
	if b.active {
		model, cmd := b.editor.Update(msg)
		b.editor, _ = model.(components.Text)

		return b, cmd
	}

	return b, nil
}
