package requesteditor

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/model"
)

type documentation struct {
	docData model.About
	Style   lipgloss.Style
}

func DocumentationTab(docData model.About) documentation {
	return documentation{
		docData: docData,
	}
}

func (d documentation) View() string {
	return "Documentation"
}

func (d documentation) Init() tea.Cmd {
	return nil
}

func (d documentation) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return d, nil
}
