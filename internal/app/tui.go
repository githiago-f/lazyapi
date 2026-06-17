// Package app implements the tui main view
package app

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/app/pane/editor"
	"github.com/githiago-f/lazyapi/internal/app/pane/requests"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

type Tui struct {
	tea.Model
	titleBar components.TitleBar

	currentRequest *model.Request

	help        help.Model
	requestList requests.RequestList
	editor      *editor.RequestPane
	prompt      components.PromptModel
}

func NewTui() Tui {
	actualList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	actualList.Title = "Requests"
	actualList.SetShowHelp(false)
	actualList.SetShowStatusBar(false)

	tui := Tui{
		titleBar: components.TitleBar{
			Style: lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, true, false),
		},
		help: help.New(),
		requestList: requests.RequestList{
			List: actualList,
		},
		prompt: components.Prompt(""),
	}

	tui.editor = editor.New(tui.currentRequest)

	return tui
}

func (t Tui) View() string {
	var currentView string
	switch config.DefaultConfig.Active {
	case config.RequestList:
		currentView = t.requestList.View()
	case config.RequestEditor:
		currentView = t.editor.View()
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		t.prompt.View(),
		t.titleBar.View(),
		currentView,
		t.help.View(config.DefaultKeyMap),
	)
}

func (t Tui) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, tea.WindowSize(), store.FindRequestFiles())
}

func (t Tui) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		subModel tea.Model
		cmd      tea.Cmd
	)

	switch msg := msg.(type) {
	case store.RequestFilesMsg:
		return t, store.LoadRequestsList(msg.Paths)

	case store.LoadedRequestListMsg:
		if len(msg.Items) == 0 {
			return t, cmd
		}
		cmd = t.requestList.List.SetItems(msg.Items)
		return t, cmd

	case requests.OpenRequestViewMsg:
		return t, store.OpenRequestFile(*msg.OpenAPIRef)

	case editor.CloseRequestPaneMsg:
		if msg.SaveToFile {
			cmd = tea.Batch(
				store.SaveFile(t.editor.GetAsRequestData()),
				store.FindRequestFiles(),
			)
		}
		t.editor.Reset()
		config.DefaultConfig.Active = config.RequestList
		return t, cmd

	case tea.WindowSizeMsg:
		_, titleBarHeight := lipgloss.Size(t.titleBar.View())
		t.titleBar.Width = msg.Width

		t.requestList.List.SetSize(msg.Width, msg.Height-(titleBarHeight+1))

		subModel, _ = t.editor.Update(msg)
		ptr := subModel.(editor.RequestPane)
		t.editor = &ptr

		subModel, _ = t.prompt.Update(msg)
		t.prompt, _ = subModel.(components.PromptModel)

		t.prompt.SetPosition((msg.Width / 2), (msg.Height / 2))

	case store.LoadedFile:
		t.currentRequest = &msg.Data
		rp := t.editor.SetValue(msg.Data)
		t.editor = &rp
		config.DefaultConfig.Active = config.RequestEditor
		return t, nil

	case tea.KeyMsg:
		switch {
		case config.DefaultConfig.Active == config.PageIndex(0) && key.Matches(msg, config.DefaultKeyMap.Quit):
			return t, tea.Quit
		case key.Matches(msg, config.DefaultKeyMap.Kill):
			return t, tea.Quit
		}
	}

	switch config.DefaultConfig.Active {
	case config.RequestList:
		subModel, cmd = t.requestList.Update(msg)
		t.requestList, _ = subModel.(requests.RequestList)
	case config.RequestEditor:
		subModel, cmd = t.editor.Update(msg)
		ptr, _ := subModel.(editor.RequestPane)
		t.editor = &ptr
	}

	return t, cmd
}
