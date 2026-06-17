// Package requests implements a list component for requests
package requests

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/config"
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
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Select):
			item, _ := rl.List.SelectedItem().(RequestItem)
			return rl, OpenRequestView(item.FileName)
		}
	}

	return rl, cmd
}
