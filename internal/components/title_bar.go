package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
)

type TitleBar struct {
	config config.Config

	Title string
	Width int
	Style lipgloss.Style
}

func (t TitleBar) View() string {
	name := t.config.Name()
	if t.Title != "" {
		name = t.Title
	}
	return t.Style.
		Width(t.Width).
		Render(name)
}
