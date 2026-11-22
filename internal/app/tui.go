package app

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/app/pane/editor"
	"github.com/githiago-f/lazyapi/internal/app/pane/historyview"
	"github.com/githiago-f/lazyapi/internal/app/pane/palette"
	"github.com/githiago-f/lazyapi/internal/app/pane/requests"
	"github.com/githiago-f/lazyapi/internal/app/pane/responses"
	"github.com/githiago-f/lazyapi/internal/app/pane/runview"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/project"
	"github.com/githiago-f/lazyapi/internal/runner"
	"github.com/githiago-f/lazyapi/internal/store"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

type viewKeyMap struct {
	short []key.Binding
	full  [][]key.Binding
}

func (v viewKeyMap) ShortHelp() []key.Binding  { return v.short }
func (v viewKeyMap) FullHelp() [][]key.Binding { return v.full }

type Tui struct {
	tea.Model
	titleBar components.TitleBar

	currentRequest *model.Request

	help        help.Model
	requestList requests.RequestList
	preview     requests.PreviewModel
	previewResp responses.Model
	tagsOverlay *requests.TagsOverlay
	editor      *editor.RequestPane
	prompt      components.PromptModel
	envStore    *env.Store

	notes components.Notifications

	// cancelSend aborts the request in flight (nil when idle).
	cancelSend context.CancelFunc
	// confirm holds a destructive request waiting for y/n.
	confirm *responses.SendMsg
	// confirmRun is a run with writes waiting for y/N.
	confirmRun *runview.Run

	palette     *palette.Model
	runner      runview.Model
	runnerFrom  config.PageIndex // page to return to when the runner closes
	history     historyview.Model
	historyFrom config.PageIndex
	// templates feeds field completions; missing are the editor's
	// references without a value.
	templates *templateSource
	missing   []string

	// project is lazyapi.config.yml (nil when absent); activeEnv is the
	// selected named environment (nil: none).
	project   *project.Config
	activeEnv *project.Environment

	width, height int

	showHelp     bool
	defaultFile  string
	openAPIFiles []string
}

func selectedChanged(p requests.PreviewModel, selected requests.RequestItem) bool {
	if !p.HasItem() {
		return true
	}
	return p.CurrentItemURI() != selected.URI || p.CurrentItemMethod() != selected.Method
}

var (
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(config.Peach))
	titleInfoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Subtext0))
	titleEnvStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Crust)).Background(lipgloss.Color(config.Teal)).Padding(0, 1)
	titleNoEnv     = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1)).Background(lipgloss.Color(config.Surface0)).Padding(0, 1)
	missingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Yellow))
	confirmTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(config.Red))
	confirmKey     = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender))
	confirmStyle   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(config.Red)).
			Background(lipgloss.Color(config.Surface0)).
			Padding(1, 2)
	helpBarStyle = lipgloss.NewStyle().Padding(0, 1)
)

func NewTui(defaultFile string, envFile string) Tui {
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender))
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1))
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Surface2))

	tui := Tui{
		defaultFile: defaultFile,
		titleBar: components.TitleBar{
			Style: lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, true, false),
		},
		help:        h,
		requestList: requests.NewRequestList(),
		preview:     requests.NewPreview(),
		previewResp: responses.New("Press s to send this request · Enter to open it in the editor"),
		prompt:      components.Prompt(""),
		envStore:    env.NewStore(envFile),
		notes:       components.NewNotifications(),
		runner:      runview.New(),
		history:     historyview.New(),
	}

	tui.templates = &templateSource{store: tui.envStore}
	components.TemplateNames = tui.templates.names

	tui.editor = editor.New(tui.currentRequest)
	tui.editor.SetEnvStore(tui.envStore)
	tui.titleBar.Title = tui.title()

	return tui
}

// title shows the app, the spec in use and the active environment.
func (t Tui) title() string {
	parts := []string{titleStyle.Render(">_ lazyapi")}
	spec := t.defaultFile
	if spec == "" && len(t.openAPIFiles) == 1 {
		spec = t.openAPIFiles[0]
	}
	switch {
	case spec != "":
		parts = append(parts, titleInfoStyle.Render(filepath.Base(spec)))
	case len(t.openAPIFiles) > 1:
		parts = append(parts, titleInfoStyle.Render(fmt.Sprintf("%d specs", len(t.openAPIFiles))))
	}
	envLabel := titleNoEnv.Render("env: none · ctrl+v")
	switch {
	case t.activeEnv != nil:
		label := "env: " + t.activeEnv.Name
		if server := t.envServer(); server != "" {
			label += " → " + server
		}
		envLabel = titleEnvStyle.Render(label)
	case t.envStore.Path() != "":
		envLabel = titleEnvStyle.Render("env: " + env.Label(t.envStore.Path()))
	}
	return strings.Join(parts, titleInfoStyle.Render("  ·  ")) + "   " + envLabel
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

func (t Tui) keyMap() viewKeyMap {
	var vkm viewKeyMap
	switch config.DefaultConfig.Active {
	case config.RequestList:
		vkm.short = []key.Binding{
			config.DefaultKeyMap.SendRequest, config.DefaultKeyMap.Select, config.DefaultKeyMap.Filter,
			config.DefaultKeyMap.CreateNew, config.DefaultKeyMap.RunAll, config.DefaultKeyMap.Palette,
			config.DefaultKeyMap.SwitchEnv, config.DefaultKeyMap.HelpToggle, config.DefaultKeyMap.Quit,
		}
	case config.Runner:
		vkm.short = t.runner.HelpBindings()
	case config.History:
		vkm.short = t.history.HelpBindings()
	case config.RequestEditor:
		vkm.short = t.editor.HelpBindings()
		if !t.editor.CapturesKeys() {
			vkm.short = append(vkm.short, config.DefaultKeyMap.HelpToggle)
		}
	}
	if t.cancelSend != nil {
		vkm.short = append([]key.Binding{config.DefaultKeyMap.Cancel}, vkm.short...)
	}
	vkm.full = config.DefaultKeyMap.FullHelp()
	return vkm
}

func (t Tui) View() tea.View {
	var currentView string
	vkm := t.keyMap()

	switch config.DefaultConfig.Active {
	case config.RequestList:
		listView := t.requestList.View().Content
		right := t.preview.View().Content
		if t.preview.HasItem() {
			right = lipgloss.JoinVertical(lipgloss.Left, right, lipgloss.NewStyle().PaddingLeft(1).Render(t.previewResp.View()))
		}

		if t.tagsOverlay != nil {
			ov := t.tagsOverlay.View().Content
			cursorY := t.requestList.CursorPosY()
			_, titleBarHeight := lipgloss.Size(t.titleBar.View().Content)
			yOff := titleBarHeight + cursorY
			currentView = overlay.Composite(ov, listView+"\n"+right, overlay.Left, overlay.Top, 0, yOff)
		} else {
			currentView = lipgloss.JoinHorizontal(lipgloss.Top, listView, right)
		}

	case config.RequestEditor:
		currentView = t.editor.View().Content

	case config.Runner:
		currentView = t.runner.View()

	case config.History:
		currentView = t.history.View()
	}

	helpText := t.help.ShortHelpView(vkm.ShortHelp())
	if config.DefaultConfig.Active == config.RequestEditor && len(t.missing) > 0 {
		helpText = missingStyle.Render("⚠ no value: {{"+strings.Join(t.missing, "}} {{")+"}}") + "  " + helpText
	}
	helpBar := helpBarStyle.Width(t.width).MaxHeight(1).Render(ansi.Truncate(helpText, max(0, t.width-2), "…"))
	_, contentHeight := t.contentSize()
	currentView = lipgloss.NewStyle().Height(contentHeight).MaxHeight(contentHeight).Render(currentView)

	sections := []string{t.titleBar.View().Content, currentView, helpBar}
	if p := t.prompt.View().Content; p != "" {
		sections = append([]string{p}, sections...)
	}
	base := lipgloss.JoinVertical(lipgloss.Left, sections...)

	if t.notes.Visible() {
		base = overlay.Composite(t.notes.View().Content, base, overlay.Center, overlay.Top, 0, 1)
	}

	if t.confirm != nil {
		req := t.confirm.Request
		box := confirmStyle.Render(fmt.Sprintf("%s\n\n%s %s\n%s\n\n%s",
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(config.Red)).Render("Send a request that may change data?"),
			req.Method.Label(), req.ResolvedURL(), titleInfoStyle.Render("env: "+t.envName()),
			lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender)).Render("y")+" send   "+
				lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender)).Render("any other key")+" cancel"))
		base = overlay.Composite(box, base, overlay.Center, overlay.Center, 0, 0)
	}

	if t.confirmRun != nil {
		box := confirmStyle.Render(t.confirmRunText())
		base = overlay.Composite(box, base, overlay.Center, overlay.Center, 0, 0)
	}

	if t.palette != nil {
		base = overlay.Composite(t.palette.View(), base, overlay.Center, overlay.Top, 0, 3)
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

// envServer is the active environment's server as a URL.
func (t Tui) envServer() string {
	if t.activeEnv == nil || t.activeEnv.Server == "" {
		return ""
	}
	spec := t.defaultFile
	if spec == "" && len(t.openAPIFiles) > 0 {
		spec = t.openAPIFiles[0]
	}
	servers, _ := store.LoadServers(spec)
	if u := serverFor(t.activeEnv.Server, servers); u != "" {
		return u
	}
	return t.activeEnv.Server
}

// envName labels the active environment for messages.
func (t Tui) envName() string {
	if t.activeEnv != nil {
		return t.activeEnv.Name
	}
	return env.Label(t.envStore.Path())
}

// contentSize is the area between the title bar and the help bar.
func (t Tui) contentSize() (int, int) {
	_, titleBarHeight := lipgloss.Size(t.titleBar.View().Content)
	return t.width, max(0, t.height-titleBarHeight-1)
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

func notify(message string, kind components.NotificationType) tea.Cmd {
	return func() tea.Msg { return components.ShowNotificationMsg{Message: message, Type: kind} }
}

// startSend sends msg with a cancellable context and shows the spinner.
func (t Tui) startSend(msg responses.SendMsg) (Tui, tea.Cmd) {
	if t.cancelSend != nil {
		t.cancelSend()
	}
	t.applyEnv(&msg)
	ctx, cancel := context.WithCancel(context.Background())
	t.cancelSend = cancel
	var spin tea.Cmd
	switch msg.Source {
	case responses.FromEditor:
		var rp editor.RequestPane
		rp, spin = t.editor.StartSending()
		t.editor = &rp
	case responses.FromPreview:
		spin = t.previewResp.StartSending()
	}
	return t, tea.Batch(spin, responses.Exchange(ctx, msg))
}

// previewSendMsg builds the request for the item selected in the list,
// including unsaved edits and spec examples.
func (t Tui) previewSendMsg() (*responses.SendMsg, error) {
	item := t.preview.Item()
	if item == nil {
		return nil, nil
	}
	if item.OpenAPIRef == nil {
		return nil, fmt.Errorf("drafts are sent from the editor: press enter to open it")
	}
	req, err := store.LoadRequest(*item.OpenAPIRef)
	if err != nil {
		return nil, err
	}
	if u := t.preview.CurrentServerURL(); u != "" {
		req.ServerURL = u
	}
	req.Env, _ = t.envStore.Load()
	msg := &responses.SendMsg{Source: responses.FromPreview, Request: req}
	t.applyEnv(msg)
	return msg, nil
}

// applyEnv points a request at the active environment (server, vars,
// credentials). It is idempotent.
func (t Tui) applyEnv(msg *responses.SendMsg) {
	e := t.activeEnv
	if e == nil {
		return
	}
	if e.Server != "" {
		if u := serverFor(e.Server, msg.Request.Servers); u != "" {
			msg.Request.ServerURL = u
		}
	}
	msg.Auth, msg.Vars, msg.HTTP = e.Auth, e.Vars, e.HTTP
}

// serverFor resolves an environment's server: a URL, or an index into the
// spec's servers.
func serverFor(v string, servers []string) string {
	if idx, err := strconv.Atoi(v); err == nil {
		if idx >= 0 && idx < len(servers) {
			return servers[idx]
		}
		return ""
	}
	return v
}

// useEnv activates a named environment (nil clears it).
func (t *Tui) useEnv(e *project.Environment) {
	t.activeEnv = e
	path := ""
	t.templates.vars = nil
	if e != nil {
		path = e.Env
		t.templates.vars = e.Vars
	}
	t.envStore.SetPath(path)
	t.titleBar.Title = t.title()
}

// switchEnv cycles through the named environments of lazyapi.config.yml
// (then "none"), or through the env files in the working directory when
// there is no config.
func (t Tui) switchEnv() (Tui, tea.Cmd) {
	if t.project != nil {
		names := append(t.project.Names(), "")
		current := ""
		if t.activeEnv != nil {
			current = t.activeEnv.Name
		}
		next := names[(slices.Index(names, current)+1)%len(names)]
		if next == "" {
			t.useEnv(nil)
			return t, notify("Environment: none", components.Info)
		}
		e, _ := t.project.Select(next)
		t.useEnv(e)
		return t, notify("Environment: "+next, components.Info)
	}

	options := append([]string{""}, env.Discover(".")...)
	if p := t.envStore.Path(); p != "" && !slices.Contains(options, p) {
		options = append(options, p)
	}
	idx := slices.Index(options, t.envStore.Path())
	next := options[(idx+1)%len(options)]
	t.envStore.SetPath(next)
	t.titleBar.Title = t.title()
	if len(options) == 1 {
		return t, notify("No .env files in this directory (.env, .env.<name>, <name>.env)", components.Info)
	}
	return t, notify("Environment: "+env.Label(next), components.Info)
}

// capturesKeys reports whether a text input owns the keyboard, so global
// shortcuts (q, ?, s, ctrl+v) must not fire.
func (t Tui) capturesKeys() bool {
	switch config.DefaultConfig.Active {
	case config.RequestList:
		return t.requestList.Filtering()
	case config.RequestEditor:
		return t.editor.CapturesKeys()
	case config.Runner:
		return t.runner.CapturesKeys()
	case config.History:
		return t.history.CapturesKeys()
	}
	return false
}

func (t Tui) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		subModel tea.Model
		cmd      tea.Cmd
	)

	// A destructive request waits for confirmation: y sends, anything else cancels.
	if t.confirm != nil {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			pending := *t.confirm
			t.confirm = nil
			if keyMsg.String() == "y" || keyMsg.String() == "Y" {
				return t.startSend(pending)
			}
			return t, notify("Request not sent", components.Info)
		}
	}

	if t.confirmRun != nil {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			pending := *t.confirmRun
			t.confirmRun = nil
			if keyMsg.String() == "y" || keyMsg.String() == "Y" {
				return t.beginRun(pending)
			}
			return t, notify("Run cancelled", components.Info)
		}
	}

	// The palette owns the keyboard while open.
	if t.palette != nil {
		switch msg := msg.(type) {
		case palette.ClosedMsg:
			t.palette = nil
			return t, nil
		case palette.ChosenMsg:
			t.palette = nil
			return t.Update(msg.Msg)
		case tea.KeyPressMsg:
			if key.Matches(msg, config.DefaultKeyMap.Kill) {
				return t, tea.Quit
			}
			p, pcmd := t.palette.Update(msg)
			t.palette = &p
			return t, pcmd
		case tea.WindowSizeMsg:
			t.palette.SetSize(msg.Width, msg.Height)
		}
	}

	// esc cancels a request in flight (before it reaches any pane).
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && t.cancelSend != nil && key.Matches(keyMsg, config.DefaultKeyMap.Cancel) {
		t.cancelSend()
		t.cancelSend = nil
		return t, nil
	}

	// When tag overlay is open, route keyboard events to it.
	// Ctrl+C still quits. Lifecycle/resize messages pass through.
	if t.tagsOverlay != nil {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			if key.Matches(keyMsg, config.DefaultKeyMap.Kill) {
				return t, tea.Quit
			}
			model, overlayCmd := t.tagsOverlay.Update(keyMsg)
			t.tagsOverlay, _ = model.(*requests.TagsOverlay)
			return t, overlayCmd
		}
		switch msg.(type) {
		case requests.CancelTagsMsg, requests.SaveTagsMsg, tea.WindowSizeMsg:
		default:
			return t, nil
		}
	}

	switch msg := msg.(type) {
	case runTestsMsg:
		return t.startRun(msg)

	case showRunnerMsg:
		return t.beginShowRunner()

	case runview.CloseMsg:
		config.DefaultConfig.Active = t.runnerFrom
		return t, nil

	case showHistoryMsg:
		return t.showHistory()

	case historyview.CloseMsg:
		config.DefaultConfig.Active = t.historyFrom
		return t, nil

	case historyview.ReplayMsg:
		return t.replay(msg.Entry)

	case runview.OpenOperationMsg:
		ref := msg.Ref
		return t, store.OpenRequestFile(ref)

	case useEnvMsg:
		if msg.name == "" {
			t.useEnv(nil)
			return t, notify("Environment: none", components.Info)
		}
		e, err := t.project.Select(msg.name)
		if err != nil {
			return t, notify(err.Error(), components.Error)
		}
		t.useEnv(e)
		return t, notify("Environment: "+msg.name, components.Info)

	case useEnvFileMsg:
		t.envStore.SetPath(msg.path)
		t.titleBar.Title = t.title()
		return t, notify("Environment: "+env.Label(msg.path), components.Info)

	case copyCurlMsg:
		return t, t.copyCurl()

	case toggleHelpMsg:
		t.showHelp = !t.showHelp
		return t, nil

	case store.RequestFilesMsg:
		t.openAPIFiles = msg.Paths
		spec := t.defaultFile
		if spec == "" && len(msg.Paths) > 0 {
			spec = msg.Paths[0]
		}
		if cfg, err := project.Find(spec); err != nil {
			cmd = notify("lazyapi.config.yml: "+err.Error(), components.Error)
		} else if cfg != nil {
			t.project = cfg
			// An explicit --env file keeps priority over the default environment.
			if t.envStore.Path() == "" {
				if e, err := cfg.Select(""); err == nil && e != nil {
					t.useEnv(e)
				}
			}
		}
		t.titleBar.Title = t.title()
		return t, tea.Batch(cmd, store.LoadRequestsList(msg.Paths))

	case store.LoadedRequestListMsg:
		t.requestList = t.requestList.SetItems(msg.Items)
		if len(msg.Items) > 0 && !t.preview.HasItem() {
			first := msg.Items[0]
			servers, serverURL := store.LoadServers(first.FileName)
			t.preview.SetItem(first, servers, serverURL)
		}
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
			return t, notify("No OpenAPI spec loaded: start lazyapi with a spec file", components.Error)
		}
		servers, serverURL := store.LoadServers(target)
		envMap, _ := t.envStore.Load()
		req := model.Request{
			FileName:  target,
			Method:    model.GET,
			Body:      model.Body{Type: model.ApplicationJSON},
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
			notify("Request deleted", components.Info),
		)

	case requests.SendRequestMsg:
		send, err := t.previewSendMsg()
		if err != nil {
			return t, notify(err.Error(), components.Error)
		}
		if send == nil {
			return t, nil
		}
		if !slices.Contains(runner.SafeMethods, send.Request.Method.Label()) {
			t.confirm = send
			return t, nil
		}
		return t.startSend(*send)

	case responses.SendMsg:
		return t.startSend(msg)

	case responses.ResultMsg:
		t.cancelSend = nil
		cmd = t.recordHistory(msg)
		switch msg.Source {
		case responses.FromEditor:
			rp := t.editor.SetResult(msg)
			if config.DefaultConfig.Active == config.RequestEditor {
				rp = rp.FocusResponse()
			}
			t.editor = &rp
		case responses.FromPreview:
			t.previewResp.SetResult(msg.Result)
		}
		return t, tea.Batch(cmd, resultNotification(msg.Result))

	case requests.EditTagsMsg:
		item, ok := t.requestList.SelectedItem()
		if !ok {
			return t, nil
		}
		ov := requests.NewTagsOverlay(item)
		t.tagsOverlay = &ov
		return t, nil

	case requests.CycleServerMsg:
		t.preview.CycleServer()
		item, ok := t.requestList.SelectedItem()
		if ok && item.OpenAPIRef != nil {
			return t, store.PersistServerURLCmd(*item.OpenAPIRef, t.preview.CurrentServerURL())
		}
		return t, nil

	case requests.SaveTagsMsg:
		t.tagsOverlay = nil
		switch {
		case msg.Ref != nil:
			cmd = tea.Batch(
				store.UpdateOperationTagsCmd(*msg.Ref, msg.Tags),
				store.LoadRequestsList(t.openAPIFiles),
			)
		case msg.DraftPath != "":
			cmd = tea.Batch(
				store.UpdateDraftTagsCmd(msg.DraftPath, msg.Tags),
				store.LoadRequestsList(t.openAPIFiles),
			)
		}
		return t, cmd

	case requests.CancelTagsMsg:
		t.tagsOverlay = nil
		return t, nil

	case store.ErrorMsg:
		return t, notify(msg.Message, components.Error)

	case store.FileSaved:
		return t, notify("✓ Saved to "+filepath.Base(msg.Path), components.Success)

	case store.RequestDeleted:
		return t, nil

	case store.TagsUpdatedMsg:
		if !msg.Success {
			return t, notify("Failed to update tags: "+msg.Error, components.Error)
		}

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
		t.missing = t.missingRefs()
		return t, nil

	case editor.CloseRequestPaneMsg:
		// Save first, then reload the list so it reflects the saved spec.
		cmd = store.LoadRequestsList(t.openAPIFiles)
		if msg.SaveToFile {
			cmd = tea.Sequence(store.SaveFile(t.editor.GetAsRequestData()), cmd)
		}
		if t.cancelSend != nil {
			t.cancelSend()
			t.cancelSend = nil
		}
		t.editor.Reset()
		config.DefaultConfig.Active = config.RequestList
		return t, cmd

	case tea.WindowSizeMsg:
		t.width, t.height = msg.Width, msg.Height
		t.titleBar.Width = msg.Width
		t.help.SetWidth(msg.Width - 2)
		_, contentHeight := t.contentSize()

		listWidth := int(float64(msg.Width) * 0.35)
		previewWidth := msg.Width - listWidth

		t.requestList.SetSize(listWidth, contentHeight)
		t.preview.SetSize(previewWidth, contentHeight)
		t.previewResp.SetSize(previewWidth-2, contentHeight-t.preview.HeaderHeight())

		t.runner.SetSize(msg.Width, contentHeight)
		t.history.SetSize(msg.Width, contentHeight)

		subModel, _ = t.editor.Update(tea.WindowSizeMsg{Width: msg.Width, Height: contentHeight})
		ptr := subModel.(editor.RequestPane)
		t.editor = &ptr
		return t, nil

	case store.ExampleSavedMsg:
		if msg.Success {
			return t, notify("✓ Response saved as an example in the spec", components.Success)
		}
		return t, notify(msg.Error, components.Error)

	case editor.TestsResultMsg:
		if config.DefaultConfig.Active == config.RequestEditor {
			rp := t.editor.SetTestResults(msg.Result)
			t.editor = &rp
			return t, nil
		}

	case tea.KeyPressMsg:
		capture := t.capturesKeys()
		if !capture && key.Matches(msg, config.DefaultKeyMap.HelpToggle) {
			t.showHelp = !t.showHelp
			return t, nil
		}
		if t.showHelp {
			t.showHelp = false
			return t, nil
		}
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Kill):
			return t, tea.Quit
		case key.Matches(msg, config.DefaultKeyMap.Palette) && t.tagsOverlay == nil:
			w, h := t.width, t.height
			p := palette.New(t.paletteCommands(), w, h)
			t.palette = &p
			return t, p.Init()
		case !capture && config.DefaultConfig.Active == config.RequestList && key.Matches(msg, config.DefaultKeyMap.RunAll):
			if specs := t.specs(); len(specs) > 0 {
				return t.startRun(runTestsMsg{spec: specs[0], methods: runner.SafeMethods, title: filepath.Base(specs[0])})
			}
			return t, notify("No OpenAPI spec loaded", components.Error)
		case !capture && config.DefaultConfig.Active == config.RequestList && key.Matches(msg, config.DefaultKeyMap.History):
			return t.showHistory()
		case !capture && config.DefaultConfig.Active == config.RequestList && key.Matches(msg, config.DefaultKeyMap.Quit):
			return t, tea.Quit
		case !capture && key.Matches(msg, config.DefaultKeyMap.SwitchEnv):
			return t.switchEnv()
		case config.DefaultConfig.Active == config.RequestEditor && key.Matches(msg, config.DefaultKeyMap.CopyCurl):
			return t, t.copyCurl()
		case config.DefaultConfig.Active == config.RequestEditor &&
			key.Matches(msg, config.DefaultKeyMap.SaveExample):
			statusCode, header, body := t.editor.LastResponse()
			ref := t.editor.CurrentRef()
			switch {
			case ref == nil:
				return t, notify("Save the request to the spec before saving examples", components.Info)
			case statusCode == 0:
				return t, notify("Send the request first", components.Info)
			default:
				return t, store.SaveResponseExampleCmd(*ref, statusCode, header, body)
			}
		case config.DefaultConfig.Active == config.RequestList && !capture:
			// Response tabs and scrolling for the preview.
			switch msg.String() {
			case "[", "]", "pgup", "pgdown":
				t.previewResp.Focus()
				t.previewResp, cmd = t.previewResp.Update(msg)
				t.previewResp.Blur()
				return t, cmd
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

	// Spinner ticks and mouse wheel for the preview response.
	if !isKey(msg) {
		var pcmd tea.Cmd
		t.previewResp, pcmd = t.previewResp.Update(msg)
		cmd = tea.Batch(cmd, pcmd)
	}

	switch config.DefaultConfig.Active {
	case config.RequestList:
		var lcmd tea.Cmd
		subModel, lcmd = t.requestList.Update(msg)
		t.requestList, _ = subModel.(requests.RequestList)
		cmd = tea.Batch(cmd, lcmd)

		selected, ok := t.requestList.SelectedItem()
		if ok && (!t.preview.HasItem() || selectedChanged(t.preview, selected)) {
			servers, serverURL := store.LoadServers(selected.FileName)

			if persistedURL := store.LoadPersistedServerURL(selected); persistedURL != "" {
				serverURL = persistedURL
			}

			t.preview.SetItem(selected, servers, serverURL)
			t.previewResp.Clear()
			_, contentHeight := t.contentSize()
			t.previewResp.SetSize(t.width-int(float64(t.width)*0.35)-2, contentHeight-t.preview.HeaderHeight())
		}

	case config.RequestEditor:
		var ecmd tea.Cmd
		subModel, ecmd = t.editor.Update(msg)
		ptr, _ := subModel.(editor.RequestPane)
		t.editor = &ptr
		cmd = tea.Batch(cmd, ecmd)
		if isKey(msg) {
			t.missing = t.missingRefs()
		}
	}

	if config.DefaultConfig.Active == config.History {
		var hcmd tea.Cmd
		t.history, hcmd = t.history.Update(msg)
		cmd = tea.Batch(cmd, hcmd)
	}

	// The runner keeps receiving its results while another page is shown.
	if config.DefaultConfig.Active == config.Runner || !isKey(msg) {
		var rcmd tea.Cmd
		t.runner, rcmd = t.runner.Update(msg)
		cmd = tea.Batch(cmd, rcmd)
	}

	return t, cmd
}

// copyCurl puts the editor's request on the clipboard as a curl command,
// with the active environment's server and credentials (before() hooks are
// not run: they may send requests).
func (t Tui) copyCurl() tea.Cmd {
	msg := responses.SendMsg{Request: t.editor.GetAsRequestData()}
	t.applyEnv(&msg)
	req := msg.Request
	req.Auth = model.ApplySecrets(req.Auth, msg.Auth)
	cmd, err := req.Curl(context.Background())
	if err != nil {
		return notify("Cannot build curl: "+err.Error(), components.Error)
	}
	if err := clipboard.WriteAll(cmd); err != nil {
		return notify("Clipboard unavailable: "+err.Error(), components.Error)
	}
	return notify("✓ Request copied as cURL", components.Success)
}

// resultNotification summarises a finished request.
func resultNotification(r runner.OpResult) tea.Cmd {
	if r.Response == nil {
		return notify("✗ "+r.Reason, components.Error)
	}
	failed := 0
	for _, c := range r.Checks {
		if !c.Passed {
			failed++
		}
	}
	text := fmt.Sprintf("%d · %d ms", r.Response.Status, r.Response.DurationMs)
	if failed > 0 {
		return notify(fmt.Sprintf("✗ %s · %d check(s) failed", text, failed), components.Error)
	}
	return notify("✓ "+text, components.Success)
}

func isKey(msg tea.Msg) bool {
	_, ok := msg.(tea.KeyPressMsg)
	return ok
}
