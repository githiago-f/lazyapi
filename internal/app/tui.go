package app

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	requesteditor "github.com/githiago-f/lazyapi/internal/app/pane/requesteditor"
	requestlist "github.com/githiago-f/lazyapi/internal/app/pane/requestlist"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/store"
)

type Tui struct {
	tea.Model
	titleBar    components.TitleBar
	help        help.Model
	requestList requestlist.RequestList
	request     requesteditor.RequestPane
}

func NewTui() Tui {
	actualList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	actualList.Title = "Requests"
	actualList.SetShowHelp(false)
	actualList.SetShowStatusBar(false)

	return Tui{
		titleBar: components.TitleBar{
			Style: lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, true, false),
		},
		help: help.New(),
		requestList: requestlist.RequestList{
			List: actualList,
		},
		request: requesteditor.New(),
	}
}

func (t Tui) View() string {
	var currentView string
	switch config.DefaultConfig.Active {
	case config.RequestList:
		currentView = t.requestList.View()
	case config.RequestEditor:
		currentView = t.request.View()
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
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
			// TODO redirect to create file
			return t, cmd
		}
		cmd = t.requestList.List.SetItems(msg.Items)
		return t, cmd

	case requestlist.OpenRequestViewMsg:
		config.DefaultConfig.Active = config.RequestEditor
		return t, store.OpenRequestFile(msg.FileName)

	case requesteditor.CloseRequestPaneMsg:
		config.DefaultConfig.Active = config.RequestList
		return t, cmd

	case tea.WindowSizeMsg:
		t.titleBar.Width = msg.Width
		t.request.Tabs.Width = msg.Width - 2
		methodWidth, _ := lipgloss.Size(t.request.Method.View())
		sendWidth, _ := lipgloss.Size(t.request.Send.View())

		t.request.URI.Style = t.request.URI.Style.Width(msg.Width - (methodWidth + sendWidth + 2))
		_, titleBarHeight := lipgloss.Size(t.titleBar.View())

		t.requestList.List.SetSize(msg.Width, msg.Height-(titleBarHeight+2))

		return t, nil
	case store.LoadedFile:
		t.request.FormData = msg.Data
		return t, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Quit) && config.DefaultConfig.Active == config.PageIndex(0):
			return t, tea.Quit
		case key.Matches(msg, config.DefaultKeyMap.Kill):
			return t, tea.Quit
		}
	}

	switch config.DefaultConfig.Active {
	case config.RequestList:
		subModel, cmd = t.requestList.Update(msg)
		t.requestList, _ = subModel.(requestlist.RequestList)
	case config.RequestEditor:
		subModel, cmd = t.request.Update(msg)
		t.request, _ = subModel.(requesteditor.RequestPane)
	}

	return t, cmd
}
