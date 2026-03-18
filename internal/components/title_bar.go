package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
)

type TitleBar struct {
	config config.Config
	Width  int
	Style  lipgloss.Style
}

func (t TitleBar) View() string {
	return t.Style.
		Width(t.Width).
		Render(t.config.Name())
}
