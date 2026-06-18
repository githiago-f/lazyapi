// Package app implements the tui main view
package app

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/app/pane/editor"
	"github.com/githiago-f/lazyapi/internal/app/pane/requests"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/env"
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
	envStore    *env.Store

	defaultFile  string
	openAPIFiles []string
}

func NewTui(defaultFile string, envFile string) Tui {
	actualList := list.New([]list.Item{}, requests.TreeDelegate{}, 0, 0)
	actualList.Title = "Requests"
	actualList.SetShowHelp(false)
	actualList.SetShowStatusBar(false)

	tui := Tui{
		defaultFile: defaultFile,
		titleBar: components.TitleBar{
			Style: lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, true, false),
		},
		help: help.New(),
		requestList: requests.RequestList{
			List: actualList,
		},
		prompt:   components.Prompt(""),
		envStore: env.NewStore(envFile),
	}

	tui.editor = editor.New(tui.currentRequest)
	tui.editor.SetEnvStore(tui.envStore)

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
	if t.defaultFile != "" {
		if !store.IsOpenAPIFile(t.defaultFile) {
			return tea.Batch(
				tea.Println(fmt.Sprintf("Error: %q is not a valid OpenAPI file", t.defaultFile)),
				tea.Quit,
			)
		}
		return tea.Batch(
			tea.EnterAltScreen,
			tea.WindowSize(),
			func() tea.Msg {
				return store.RequestFilesMsg{Paths: []string{t.defaultFile}}
			},
		)
	}
	return tea.Batch(tea.EnterAltScreen, tea.WindowSize(), store.FindRequestFiles())
}

func (t Tui) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		subModel tea.Model
		cmd      tea.Cmd
	)

	switch msg := msg.(type) {
	case store.RequestFilesMsg:
		t.openAPIFiles = msg.Paths
		return t, store.LoadRequestsList(msg.Paths)

	case store.LoadedRequestListMsg:
		if len(msg.Items) == 0 {
			return t, cmd
		}
		grouped := requests.GroupByResource(msg.Items)
		cmd = t.requestList.List.SetItems(grouped)
		return t, cmd

	case requests.OpenRequestViewMsg:
		if msg.DraftPath != "" {
			return t, store.OpenDraftFile(msg.DraftPath, msg.FileName)
		}
		return t, store.OpenRequestFile(*msg.OpenAPIRef)

	case requests.CreateNewRequestMsg:
		target := t.defaultFile
		if target == "" && len(t.openAPIFiles) > 0 {
			target = t.openAPIFiles[0]
		}
		if target == "" {
			return t, nil
		}
		servers, serverURL := store.LoadServers(target)
		envMap, _ := t.envStore.Load()
		req := model.Request{
			FileName:  target,
			URI:       "",
			Method:    model.GET,
			About:     model.About{},
			Body:      model.Body{Type: model.ApplicationJSON, Raw: ""},
			Headers:   map[string]string{},
			Params:    map[string]string{},
			Query:     map[string]string{},
			Servers:   servers,
			ServerURL: serverURL,
			DraftPath: store.NewDraftPath(target),
			Env:       envMap,
		}
		t.currentRequest = &req
		rp := t.editor.SetValue(req)
		t.editor = &rp
		config.DefaultConfig.Active = config.RequestEditor
		return t, store.SaveTempFile(req)

	case requests.DuplicateRequestMsg:
		return t, store.LoadForDuplicate(msg.Item)

	case requests.DeleteRequestMsg:
		return t, tea.Batch(
			store.DeleteRequestFile(msg.Item),
			store.FindRequestFiles(),
		)

	case store.DuplicateData:
		target := msg.Data.FileName
		if target == "" {
			target = t.defaultFile
		}
		if target == "" && len(t.openAPIFiles) > 0 {
			target = t.openAPIFiles[0]
		}
		if target == "" {
			return t, nil
		}
		envMap, _ := t.envStore.Load()
		dup := msg.Data
		dup.URI = dup.URI + "-copy"
		dup.FileName = target
		dup.OpenAPIRef = nil
		dup.DraftPath = store.NewDraftPath(target)
		dup.Env = envMap
		t.currentRequest = &dup
		rp := t.editor.SetValue(dup)
		t.editor = &rp
		config.DefaultConfig.Active = config.RequestEditor
		return t, store.SaveTempFile(dup)

	case store.LoadedFile:
		envMap, _ := t.envStore.Load()
		msg.Data.Env = envMap
		t.currentRequest = &msg.Data
		rp := t.editor.SetValue(msg.Data)
		t.editor = &rp
		config.DefaultConfig.Active = config.RequestEditor
		return t, nil

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

	case model.SuccessMsg:
		if config.DefaultConfig.Active == config.RequestEditor {
			rp := t.editor.SetResponse(msg.StatusCode, msg.Status, msg.Header, msg.Body)
			rp = rp.FocusResponse()
			t.editor = &rp
			return t, nil
		}

	case model.FailureMsg:
		if config.DefaultConfig.Active == config.RequestEditor {
			rp := t.editor.SetResponseError(msg.Message)
			rp = rp.FocusResponse()
			t.editor = &rp
			return t, nil
		}

	case tea.WindowSizeMsg:
		_, titleBarHeight := lipgloss.Size(t.titleBar.View())
		t.titleBar.Width = msg.Width

		t.requestList.List.SetSize(msg.Width, msg.Height-(titleBarHeight+1))

		subModel, _ = t.editor.Update(msg)
		ptr := subModel.(editor.RequestPane)
		t.editor = &ptr

	case store.ExampleSavedMsg:
		if config.DefaultConfig.Active == config.RequestEditor {
			if msg.Success {
				rp := t.editor.SetResponseFeedback("✓ Saved as example")
				t.editor = &rp
			} else {
				rp := t.editor.SetResponseError(msg.Error)
				t.editor = &rp
			}
			return t, nil
		}

	case tea.KeyMsg:
		switch {
		case config.DefaultConfig.Active == config.PageIndex(0) && key.Matches(msg, config.DefaultKeyMap.Quit):
			return t, tea.Quit
		case key.Matches(msg, config.DefaultKeyMap.Kill):
			return t, tea.Quit
		case config.DefaultConfig.Active == config.RequestEditor &&
			key.Matches(msg, config.DefaultKeyMap.SaveExample):
			statusCode, header, body := t.editor.LastResponse()
			ref := t.editor.CurrentRef()
			if ref != nil && statusCode > 0 {
				return t, store.SaveResponseExampleCmd(*ref, statusCode, header, body)
			}
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
