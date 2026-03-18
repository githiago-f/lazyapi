// Package components implements components for visualization
package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var style = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Padding(0, 1)

type View struct {
	tea.Model
	Style   lipgloss.Style
	Content string
}

func InitView(content string) View {
	return View{
		Style:   style,
		Content: content,
	}
}

func (v View) View() string {
	return v.Style.Inherit(style).Render(v.Content)
}
