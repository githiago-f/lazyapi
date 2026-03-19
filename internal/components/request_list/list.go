// Package requestlist implements a list component for requests
package requestlist

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type OpenRequestViewMsg struct {
	FileName string
}

func OpenRequestView(fileName string) tea.Cmd {
	return func() tea.Msg {
		return OpenRequestViewMsg{
			FileName: fileName,
		}
	}
}

type RequestList struct {
	List list.Model
}

func (rl RequestList) Init() tea.Cmd {
	return nil
}

func (rl RequestList) View() string {
	return rl.List.View()
}

func (rl RequestList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	rl.List, cmd = rl.List.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			item, _ := rl.List.SelectedItem().(RequestItem)
			return rl, OpenRequestView(item.FileName)
		}
	}

	return rl, cmd
}
