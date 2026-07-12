package components

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/config"
)

type TitleBar struct {
	config config.Config

	Title string
	Width int
	Style lipgloss.Style
}

func (t TitleBar) View() tea.View {
	name := t.config.Name()
	if t.Title != "" {
		name = t.Title
	}
	return tea.NewView(t.Style.
		Width(t.Width).
		Render(name))
}
