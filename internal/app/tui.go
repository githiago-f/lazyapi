package app

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/app/pane"
	"github.com/githiago-f/lazyapi/internal/components"
	requestlist "github.com/githiago-f/lazyapi/internal/components/request_list"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/store"
)

const (
	RequestList config.PageIndex = iota
)

type Tui struct {
	tea.Model
	keymap KeyMap
	config config.Config

	titleBar    components.TitleBar
	requestList requestlist.RequestList
	request     pane.RequestPane
	response    components.View
	options     components.OptionsPane
}

func NewTui() Tui {
	actualList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	actualList.Title = "Requests"
	actualList.SetShowHelp(false)
	actualList.SetShowStatusBar(false)

	return Tui{
		config: config.Config{
			Active: RequestList,
		},
		titleBar: components.TitleBar{
			Style: lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, true, false),
		},
		requestList: requestlist.RequestList{
			List: actualList,
		},
		keymap: KeyMap{},
		options: components.NewOptionsPane(
			components.NewOption("quit", "q", "^c"),
			components.NewOption("New request", "n", "+"),
			components.NewOption("filter", "/"),
			components.NewOption("up", "k", "↑"),
			components.NewOption("down", "j", "↓"),
		),
	}
}

func (t Tui) View() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		t.titleBar.View(),
		t.requestList.View(),
		t.options.View(),
	)
}

func (t Tui) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, tea.WindowSize(), store.FindRequestFiles())
}

func (t Tui) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

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
		t.titleBar.Title = msg.FileName
		return t, cmd
	case tea.WindowSizeMsg:
		t.options.Style = t.options.Style.Width(msg.Width)
		t.titleBar.Width = msg.Width

		_, optionsHeight := t.options.Style.GetFrameSize()
		_, titleBarHeight := t.titleBar.Style.GetFrameSize()

		t.requestList.List.SetSize(msg.Width, msg.Height-(optionsHeight+titleBarHeight+2))

		return t, nil
	case tea.KeyMsg:
		switch msg.String() {
		case t.keymap.Quit(), t.keymap.Kill():
			return t, tea.Quit
		}
	}

	if t.config.Active == RequestList {
		var list tea.Model
		list, cmd = t.requestList.Update(msg)
		t.requestList, _ = list.(requestlist.RequestList)
	}

	return t, cmd
}
