// Package requests implements a list component for requests
package requests

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
)

type OpenRequestViewMsg struct {
	FileName   string
	OpenAPIRef *model.OpenAPIRef
	DraftPath  string
}

func OpenRequestView(fileName string, ref *model.OpenAPIRef) tea.Cmd {
	return func() tea.Msg {
		return OpenRequestViewMsg{
			FileName:   fileName,
			OpenAPIRef: ref,
		}
	}
}

func OpenDraftView(fileName, draftPath string) tea.Cmd {
	return func() tea.Msg {
		return OpenRequestViewMsg{
			FileName:  fileName,
			DraftPath: draftPath,
		}
	}
}

type CreateNewRequestMsg struct{}

type DuplicateRequestMsg struct {
	Item RequestItem
}

type DeleteRequestMsg struct {
	Item RequestItem
}

type RequestList struct {
	List list.Model
}

func (rl RequestList) HelpBindings() []key.Binding {
	return []key.Binding{
		config.DefaultKeyMap.Select,
		config.DefaultKeyMap.Up,
		config.DefaultKeyMap.Down,
		config.DefaultKeyMap.Filter,
		config.DefaultKeyMap.CreateNew,
		config.DefaultKeyMap.Duplicate,
		config.DefaultKeyMap.Delete,
		config.DefaultKeyMap.Quit,
	}
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
		item, ok := rl.List.SelectedItem().(RequestItem)
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Select) && ok:
			if item.DraftPath != "" {
				return rl, OpenDraftView(item.FileName, item.DraftPath)
			}
			return rl, OpenRequestView(item.FileName, item.OpenAPIRef)
		case key.Matches(msg, config.DefaultKeyMap.CreateNew):
			return rl, func() tea.Msg { return CreateNewRequestMsg{} }
		case key.Matches(msg, config.DefaultKeyMap.Duplicate) && ok:
			return rl, func() tea.Msg { return DuplicateRequestMsg{Item: item} }
		case key.Matches(msg, config.DefaultKeyMap.Delete) && ok:
			return rl, func() tea.Msg { return DeleteRequestMsg{Item: item} }
		}
	}

	return rl, cmd
}
