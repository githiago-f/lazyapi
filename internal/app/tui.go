// Package app implements the tui main view
package app

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	"github.com/githiago-f/lazyapi/internal/app/pane/editor"
	"github.com/githiago-f/lazyapi/internal/app/pane/requests"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

type viewKeyMap struct {
	short []key.Binding
	full  [][]key.Binding
}

func (v viewKeyMap) ShortHelp() []key.Binding { return v.short }
func (v viewKeyMap) FullHelp() [][]key.Binding { return v.full }

type Tui struct {
	tea.Model
	titleBar components.TitleBar

	currentRequest *model.Request

	help        help.Model
	requestList requests.RequestList
	editor      *editor.RequestPane
	prompt      components.PromptModel
	envStore    *env.Store

	notes components.Notifications

	showHelp     bool
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
		notes:    components.NewNotifications(),
	}

	tui.editor = editor.New(tui.currentRequest)
	tui.editor.SetEnvStore(tui.envStore)

	return tui
}

func renderHelpList(bindings []key.Binding) string {
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(config.Green))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(config.Text))

	type entry struct{ keys, desc string }
	var entries []entry
	seen := map[string]bool{}
	maxKey := 0

	for _, b := range bindings {
		k := strings.Join(b.Keys(), "/")
		d := b.Help().Desc
		if d == "" || seen[k+":"+d] {
			continue
		}
		seen[k+":"+d] = true
		entries = append(entries, entry{k, d})
		if len(k) > maxKey {
			maxKey = len(k)
		}
	}

	rows := make([]string, 0, len(entries))
	for _, e := range entries {
		padded := lipgloss.NewStyle().Width(maxKey).Render(e.keys)
		rows = append(rows, keyStyle.Render(padded)+"  "+descStyle.Render(e.desc))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (t Tui) View() tea.View {
	var currentView string
	var vkm viewKeyMap

	switch config.DefaultConfig.Active {
	case config.RequestList:
		currentView = t.requestList.View().Content
		vkm.short = t.requestList.HelpBindings()
		vkm.full = config.DefaultKeyMap.FullHelp()

	case config.RequestEditor:
		currentView = t.editor.View().Content
		bindings := t.editor.HelpBindings()
		if len(bindings) > 0 {
			vkm.short = bindings
		} else {
			vkm.short = []key.Binding{
				config.DefaultKeyMap.Back,
				config.DefaultKeyMap.Save,
				config.DefaultKeyMap.SaveExample,
				config.DefaultKeyMap.Next,
				config.DefaultKeyMap.Prev,
			}
		}
		vkm.full = config.DefaultKeyMap.FullHelp()
	}

	base := lipgloss.JoinVertical(
		lipgloss.Left,
		t.prompt.View().Content,
		t.titleBar.View().Content,
		currentView,
	)

	if t.notes.Visible() {
		base = overlay.Composite(
			t.notes.View().Content,
			base,
			overlay.Center,
			overlay.Top,
			0,
			1,
		)
	}

	if t.showHelp {
		var allBindings []key.Binding
		allBindings = append(allBindings, vkm.ShortHelp()...)
		for _, section := range vkm.FullHelp() {
			allBindings = append(allBindings, section...)
		}
		helpContent := renderHelpList(allBindings)
		helpWidth := min(60, t.titleBar.Width-4)
		helpBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(config.Peach)).
			Background(lipgloss.Color(config.Surface0)).
			Padding(1, 2).
			Width(helpWidth).
			Render(helpContent)
		base = overlay.Composite(helpBox, base, overlay.Center, overlay.Center, 0, 0)
	}

	scanned := components.Z.Scan(base)
	return tea.View{
		Content:   scanned,
		AltScreen: true,
		MouseMode: tea.MouseModeCellMotion,
	}
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
			func() tea.Msg { return tea.RequestWindowSize() },
			func() tea.Msg {
				return store.RequestFilesMsg{Paths: []string{t.defaultFile}}
			},
		)
	}
	return tea.Batch(
		func() tea.Msg { return tea.RequestWindowSize() },
		store.FindRequestFiles(),
	)
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
			store.LoadRequestsList(t.openAPIFiles),
			func() tea.Msg {
				return components.ShowNotificationMsg{
					Message: "Request deleted",
					Type:    components.Info,
				}
			},
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
				store.LoadRequestsList(t.openAPIFiles),
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
			var noteCmd tea.Cmd
			t.notes, noteCmd = t.notes.Update(components.ShowNotificationMsg{
				Message: fmt.Sprintf("✓ %d %s", msg.StatusCode, msg.Status),
				Type:    components.Success,
			})
			return t, noteCmd
		}

	case model.FailureMsg:
		if config.DefaultConfig.Active == config.RequestEditor {
			rp := t.editor.SetResponseError(msg.Message)
			rp = rp.FocusResponse()
			t.editor = &rp
			var noteCmd tea.Cmd
			t.notes, noteCmd = t.notes.Update(components.ShowNotificationMsg{
				Message: fmt.Sprintf("✗ %s", msg.Message),
				Type:    components.Error,
			})
			return t, noteCmd
		}

	case tea.WindowSizeMsg:
		_, titleBarHeight := lipgloss.Size(t.titleBar.View().Content)
		t.titleBar.Width = msg.Width
		t.help.SetWidth(msg.Width)

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

	case tea.KeyPressMsg:
		if key.Matches(msg, config.DefaultKeyMap.HelpToggle) {
			t.showHelp = !t.showHelp
			return t, nil
		}
		if t.showHelp {
			t.showHelp = false
			return t, nil
		}
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

	case components.ShowNotificationMsg:
		var noteCmd tea.Cmd
		t.notes, noteCmd = t.notes.Update(msg)
		return t, noteCmd

	case components.DismissNotificationMsg:
		var noteCmd tea.Cmd
		t.notes, noteCmd = t.notes.Update(msg)
		return t, noteCmd

	case components.NotificationTickMsg:
		var noteCmd tea.Cmd
		t.notes, noteCmd = t.notes.Update(msg)
		return t, noteCmd

	case tea.MouseClickMsg:
		var noteCmd tea.Cmd
		t.notes, noteCmd = t.notes.Update(msg)
		cmd = noteCmd
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
