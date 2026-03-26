// Package responses implements a panel that shows response metadata
package responses

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/model"
)

type ResponseView struct {
	Type model.MimeType
	Raw  string
	Tabs tabs.Model

	Style lipgloss.Style
}

var defaultStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder())

func New() ResponseView {
	return ResponseView{
		Tabs: tabs.New(),
	}
}

func (rv ResponseView) View() string {
	return rv.Style.Inherit(defaultStyle).Render("response")
}

func (rv ResponseView) Init() tea.Cmd {
	return nil
}

func (rv ResponseView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return rv, nil
}
