// Package editor implements editor page for request data
package editor

import (
	"net/http"
	"os"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/githiago-f/lazyapi/internal/app/pane/responses"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/scrollable"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/inmath"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/script"
	"github.com/githiago-f/lazyapi/internal/store"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	"gopkg.in/yaml.v3"
)

type CloseRequestPaneMsg struct {
	SaveToFile bool
	File       string
}

type TestsResultMsg struct {
	Result script.Result
}

// externalEditDoneMsg carries the content edited in $EDITOR back to a tab.
type externalEditDoneMsg struct {
	tab     tab
	content string
	err     error
}

func closePane(save bool) tea.Cmd {
	return func() tea.Msg {
		return CloseRequestPaneMsg{
			SaveToFile: save,
		}
	}
}

type (
	field int
	tab   int
)

const (
	method field = iota
	serverField
	uri
	send
	reqTabs
	response
)

const lastField = response

const (
	Documentation tab = iota
	Params
	Authorize
	Header
	Body
	Tests
)

type RequestPane struct {
	tea.Model
	fieldsCursor int

	fileName  string
	draftPath string

	openAPIRef *model.OpenAPIRef

	envStore *env.Store

	BlockTab bool
	dirty    bool
	// savedFingerprint is the last request state written to the temp file;
	// the temp file is only rewritten when the request actually changes.
	savedFingerprint string

	Method components.Selector
	Server components.Selector
	URI    components.Field
	Send   components.Button

	RequestTabs tabs.Model

	Response  responses.Model
	respStyle lipgloss.Style
}

func methodLabels() []string {
	labels := make([]string, model.LastMethod+1)
	for i := 0; i <= int(model.LastMethod); i++ {
		labels[i] = model.Method(i).Label()
	}
	return labels
}

var defaultBtnStyle = lipgloss.NewStyle().
	Bold(true).
	Padding(0, 2).
	Border(lipgloss.InnerHalfBlockBorder()).
	Foreground(lipgloss.Color(config.Crust))

func New(request *model.Request) *RequestPane {
	docs := DocumentationTab()
	params := ParamsTab()
	authorize := AuthorizeTab()
	header := HeaderTab()
	body := BodyTab()
	tests := TestsTab()

	return &RequestPane{
		URI: components.InitField("", ""),
		Method: components.Selector{
			Cursor: 0,
			Labels: methodLabels(),
			Width:  13,
		},
		Server: components.Selector{
			Width: 45,
		},
		Send: components.Button{
			Label:  "Send",
			Active: true,
			Style: defaultBtnStyle.
				Background(lipgloss.Color(config.Flamingo)).
				BorderForeground(lipgloss.Color(config.Flamingo)),
		},

		RequestTabs: tabs.New(
			tabs.NewTab("Docs", docs),
			tabs.NewTab("Params", scrollable.New(params)),
			tabs.NewTab("Auth", scrollable.New(authorize)),
			tabs.NewTab("Headers", scrollable.New(header)),
			tabs.NewTab("Body", body),
			tabs.NewTab("Tests", scrollable.New(tests)),
		),

		Response:  responses.New("No response yet · alt+enter sends from any field"),
		respStyle: lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
	}
}

func (rp RequestPane) View() tea.View {
	activeColor := config.DefaultConfig.PrimaryColor()

	rp.Method.Style = rp.Method.Style.UnsetBorderForeground()
	rp.Server.Style = rp.Server.Style.UnsetBorderForeground()
	rp.URI.Style = rp.URI.Style.UnsetBorderForeground()
	rp.Send.Style = rp.Send.Style.UnsetBorderForeground()
	rp.RequestTabs.Style = rp.RequestTabs.Style.UnsetBorderForeground()
	respStyle := rp.respStyle

	switch field(rp.fieldsCursor) {
	case method:
		rp.Method.Style = rp.Method.Style.BorderForeground(activeColor)
	case serverField:
		rp.Server.Style = rp.Server.Style.BorderForeground(activeColor)
	case uri:
		rp.URI.Style = rp.URI.Style.BorderForeground(activeColor)
	case send:
		rp.Send.Style = rp.Send.Style.BorderForeground(activeColor).Background(activeColor)
	case reqTabs:
		rp.RequestTabs.Style = rp.RequestTabs.Style.BorderForeground(activeColor)
	case response:
		respStyle = respStyle.BorderForeground(activeColor)
	}

	requestURL := lipgloss.JoinHorizontal(
		lipgloss.Left,
		rp.Method.View().Content,
		rp.Server.View().Content,
		rp.URI.View().Content,
		rp.Send.View().Content,
	)

	fullView := lipgloss.JoinVertical(
		lipgloss.Top,
		requestURL,
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			rp.RequestTabs.View().Content,
			respStyle.Render(rp.Response.View()),
		),
	)

	if field(rp.fieldsCursor) == method && rp.Method.Open {
		dd := rp.Method.DropDownView()
		if dd != "" {
			return tea.NewView(overlay.Composite(dd, fullView, overlay.Left, overlay.Top, 0, 1))
		}
	} else if field(rp.fieldsCursor) == serverField && rp.Server.Open {
		dd := rp.Server.DropDownView()
		if dd != "" {
			methodWidth, _ := lipgloss.Size(rp.Method.View().Content)
			return tea.NewView(overlay.Composite(dd, fullView, overlay.Left, overlay.Top, methodWidth, 1))
		}
	}

	return tea.NewView(fullView)
}

func (rp *RequestPane) SetEnvStore(s *env.Store) {
	rp.envStore = s

	tp := rp.testsPane()
	tp.SetEnvStore(s)
}

func (rp RequestPane) testsPane() *testsPane {
	return rp.RequestTabs.Tabs[Tests].Content.(scrollable.Model).Content.(*testsPane)
}

func (rp RequestPane) SetValue(formData model.Request) RequestPane {
	rp.fileName = formData.FileName
	rp.draftPath = formData.DraftPath
	rp.openAPIRef = formData.OpenAPIRef
	rp.Method.Cursor = int(formData.Method)
	rp.URI.TextInput.SetValue(formData.URI)

	rp.Server.Labels = formData.Servers
	rp.Server.Cursor = 0
	for i, s := range formData.Servers {
		if s == formData.ServerURL {
			rp.Server.Cursor = i
			break
		}
	}

	docs := rp.RequestTabs.Tabs[Documentation].Content.(*documentation)
	*docs = docs.SetValue(formData.About)

	bd := rp.RequestTabs.Tabs[Body].Content.(*body)
	bd.SetValue(formData.Body.Raw)

	hd := rp.RequestTabs.Tabs[Header].Content.(scrollable.Model).Content.(*header)
	hd.SetValue(formData.Headers)

	pr := rp.RequestTabs.Tabs[Params].Content.(scrollable.Model).Content.(*params)
	pr.SetValue(formData.Query, formData.Params)

	au := rp.RequestTabs.Tabs[Authorize].Content.(scrollable.Model).Content.(*auth)
	au.SetValue(formData.Auth)

	tp := rp.testsPane()
	tp.SetValue(formData.Tests)
	tp.SetRequestData(formData)

	rp.Response.Clear()
	rp.savedFingerprint = fingerprint(rp.GetAsRequestData())

	return rp
}

func (rp *RequestPane) Reset() {
	rp.Method.Cursor = 0
	rp.Server = components.Selector{Width: 45}
	rp.fieldsCursor = 0
	rp.URI.TextInput.SetValue("")
	rp.openAPIRef = nil
	rp.draftPath = ""
	rp.savedFingerprint = ""
	rp.Response.Clear()
	rp.Response.Blur()

	rp.testsPane().Reset()

	rp.RequestTabs.SetCursor(0)
}

func (rp RequestPane) GetAsRequestData() model.Request {
	docs := rp.RequestTabs.Tabs[Documentation].Content.(*documentation)
	bd := rp.RequestTabs.Tabs[Body].Content.(*body)
	hd := rp.RequestTabs.Tabs[Header].Content.(scrollable.Model).Content.(*header)
	pr := rp.RequestTabs.Tabs[Params].Content.(scrollable.Model).Content.(*params)
	au := rp.RequestTabs.Tabs[Authorize].Content.(scrollable.Model).Content.(*auth)
	tp := rp.testsPane()

	var envMap map[string]string
	if rp.envStore != nil {
		envMap, _ = rp.envStore.Load()
	}

	return model.Request{
		FileName:   rp.fileName,
		DraftPath:  rp.draftPath,
		OpenAPIRef: rp.openAPIRef,
		About:      docs.Value(),
		URI:        rp.URI.TextInput.Value(),
		Method:     model.Method(rp.Method.Cursor),
		ServerURL:  rp.Server.Value(),
		Servers:    rp.Server.Labels,
		Body: model.Body{
			Type: model.ApplicationJSON,
			Raw:  bd.Value(),
		},
		Auth:    au.Value(),
		Headers: hd.Value(),
		Params:  pr.ParamsValue(),
		Query:   pr.QueryValue(),
		Tests:   tp.Value(),
		Env:     envMap,
	}
}

// fingerprint identifies the persisted part of a request.
func fingerprint(r model.Request) string {
	data, err := yaml.Marshal(r)
	if err != nil {
		return ""
	}
	return string(data)
}

// sendCmd asks the TUI to send the current request with the current
// (possibly unsaved) test script.
func (rp RequestPane) sendCmd() tea.Cmd {
	req := rp.GetAsRequestData()
	rp.testsPane().SetRequestData(req)
	source := rp.testsPane().Value()
	return func() tea.Msg {
		return responses.SendMsg{Source: responses.FromEditor, Request: req, Script: &source}
	}
}

// StartSending shows the spinner; the TUI calls it when it starts a request.
func (rp RequestPane) StartSending() (RequestPane, tea.Cmd) {
	cmd := rp.Response.StartSending()
	return rp, cmd
}

// SetResult shows a finished exchange and feeds it to the Tests tab.
func (rp RequestPane) SetResult(res responses.ResultMsg) RequestPane {
	rp.Response.SetResult(res.Result)
	tp := rp.testsPane()
	if r := res.Result.Response; r != nil {
		header := http.Header{}
		for k, v := range r.Headers {
			header.Set(k, v)
		}
		tp.SetResponseData(r.Status, r.StatusText, header, r.Body)
	} else {
		tp.SetResponseError()
	}
	return rp
}

func (rp RequestPane) FocusResponse() RequestPane {
	rp.fieldsCursor = int(response)
	rp.Response.Focus()
	return rp
}

// LastResponse returns the last received response (status 0 when none).
func (rp RequestPane) LastResponse() (statusCode int, header http.Header, body string) {
	res := rp.Response.Result()
	if res == nil || res.Response == nil {
		return 0, nil, ""
	}
	header = http.Header{}
	for k, v := range res.Response.Headers {
		header.Set(k, v)
	}
	return res.Response.Status, header, res.Response.Body
}

func (rp RequestPane) SetTestResults(result script.Result) RequestPane {
	rp.testsPane().SetResults(result)
	return rp
}

func (rp RequestPane) CurrentRef() *model.OpenAPIRef {
	return rp.openAPIRef
}

// CapturesKeys reports whether a text input in the editor must receive
// every key (so global shortcuts like "q" or "?" are not triggered).
func (rp RequestPane) CapturesKeys() bool {
	return rp.Response.Searching() || field(rp.fieldsCursor) == uri || rp.shouldBlockTabCommands()
}

func (rp RequestPane) shouldBlockTabCommands() bool {
	return field(rp.fieldsCursor) == reqTabs && rp.BlockTab
}

func (rp RequestPane) editableTab() bool {
	c := tab(rp.RequestTabs.Cursor())
	return field(rp.fieldsCursor) == reqTabs && (c == Body || c == Tests)
}

func (rp RequestPane) HelpBindings() []key.Binding {
	var bindings []key.Binding

	switch field(rp.fieldsCursor) {
	case method, serverField:
		bindings = append(bindings,
			config.DefaultKeyMap.Up,
			config.DefaultKeyMap.Down,
			config.DefaultKeyMap.Select,
		)

	case send:
		bindings = append(bindings, config.DefaultKeyMap.Select)

	case reqTabs:
		if rp.BlockTab {
			content := rp.RequestTabs.Tabs[rp.RequestTabs.Cursor()].Content
			if h, ok := content.(interface{ HelpBindings() []key.Binding }); ok {
				bindings = append(bindings, h.HelpBindings()...)
			}
			if rp.editableTab() {
				bindings = append(bindings, config.DefaultKeyMap.OpenEditor)
			}
			return bindings
		}
		bindings = append(bindings, config.DefaultKeyMap.Left, config.DefaultKeyMap.Right, config.DefaultKeyMap.Select)
		if rp.editableTab() {
			bindings = append(bindings, config.DefaultKeyMap.OpenEditor)
		}

	case response:
		bindings = append(bindings, rp.Response.HelpBindings()...)
		if rp.Response.Searching() {
			return bindings
		}
		bindings = append(bindings, config.DefaultKeyMap.CopyResponse)
	}

	return append(bindings,
		config.DefaultKeyMap.Next,
		config.DefaultKeyMap.Save,
		config.DefaultKeyMap.SaveExample,
		config.DefaultKeyMap.Back,
	)
}

// openExternalEditor edits the Body or Tests tab content in $EDITOR.
func (rp RequestPane) openExternalEditor() tea.Cmd {
	t := tab(rp.RequestTabs.Cursor())
	var content, ext string
	switch t {
	case Body:
		content, ext = rp.RequestTabs.Tabs[Body].Content.(*body).Value(), ".json"
	case Tests:
		content, ext = rp.testsPane().script.Value(), ".js"
	default:
		return nil
	}
	path, err := store.WriteScratchFile(ext, content)
	if err != nil {
		return notify("Cannot open editor: "+err.Error(), components.Error)
	}
	editorCmd := os.Getenv("VISUAL")
	if editorCmd == "" {
		editorCmd = os.Getenv("EDITOR")
	}
	if editorCmd == "" {
		editorCmd = "vi"
	}
	parts := strings.Fields(editorCmd)
	c := exec.Command(parts[0], append(parts[1:], path)...) //nolint:gosec // the user's own $EDITOR
	return tea.ExecProcess(c, func(err error) tea.Msg {
		edited, readErr := store.ReadScratchFile(path)
		if err == nil {
			err = readErr
		}
		return externalEditDoneMsg{tab: t, content: edited, err: err}
	})
}

func notify(msg string, kind components.NotificationType) tea.Cmd {
	return func() tea.Msg { return components.ShowNotificationMsg{Message: msg, Type: kind} }
}

func (rp RequestPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd      tea.Cmd
		model    tea.Model
		consumed bool
	)

	switch msg := msg.(type) {
	case externalEditDoneMsg:
		if msg.err != nil {
			return rp, notify("Editor failed: "+msg.err.Error(), components.Error)
		}
		switch msg.tab {
		case Body:
			rp.RequestTabs.Tabs[Body].Content.(*body).SetValue(msg.content)
		case Tests:
			rp.testsPane().script.SetValue(msg.content)
		}
		rp.dirty = true
		return rp.autosave(nil)
	case tea.KeyPressMsg:
		if key.Matches(msg, config.DefaultKeyMap.OpenEditor) && rp.editableTab() {
			return rp, rp.openExternalEditor()
		}
		if (msg.String() == "alt+enter" || msg.String() == "ctrl+enter") && !rp.Response.Searching() {
			return rp, rp.sendCmd()
		}
		// F5 re-runs the script on the last response, or sends when there is none.
		if key.Matches(msg, config.DefaultKeyMap.RunTests) && !rp.Response.Searching() {
			if rp.Response.Result() == nil || rp.Response.Result().Response == nil {
				return rp, rp.sendCmd()
			}
			rp.testsPane().SetRequestData(rp.GetAsRequestData())
			return rp, rp.testsPane().runTestsCmd()
		}
	}

	if !rp.Response.Searching() {
		switch field(rp.fieldsCursor) {
		case method:
			model, cmd = rp.Method.Update(msg)
			rp.Method, _ = model.(components.Selector)
		case serverField:
			model, cmd = rp.Server.Update(msg)
			rp.Server, _ = model.(components.Selector)
		case uri:
			model, cmd = rp.URI.Update(msg)
			rp.URI, _ = model.(components.Field)
		case send:
			model, cmd = rp.Send.Update(msg)
			rp.Send, _ = model.(components.Button)
			if rp.Send.Clicked {
				rp.Send.Clicked = false
				cmd = tea.Batch(cmd, rp.sendCmd())
			}
		case reqTabs:
			// Two-stage Esc: check if content is active before the tabs update
			wasActive := false
			if rp.BlockTab {
				if keyMsg, ok := msg.(tea.KeyPressMsg); ok && key.Matches(keyMsg, config.DefaultKeyMap.Back) {
					content := rp.RequestTabs.Tabs[rp.RequestTabs.Cursor()].Content
					if a, ok := content.(interface{ IsActive() bool }); ok {
						wasActive = a.IsActive()
					}
				}
			}

			model, cmd = rp.RequestTabs.Update(msg)
			rp.RequestTabs, _ = model.(tabs.Model)

			// Only exit tab if content was NOT active (Esc already blurred it)
			if rp.BlockTab && !wasActive {
				if keyMsg, ok := msg.(tea.KeyPressMsg); ok && key.Matches(keyMsg, config.DefaultKeyMap.Back) {
					rp.BlockTab = false
					consumed = true
				}
			}
		}
	}

	// The response view also needs spinner ticks and mouse wheel while unfocused.
	if field(rp.fieldsCursor) == response || !isKey(msg) {
		var rcmd tea.Cmd
		rp.Response, rcmd = rp.Response.Update(msg)
		cmd = tea.Batch(cmd, rcmd)
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok && field(rp.fieldsCursor) == response && !rp.Response.Searching() &&
			key.Matches(keyMsg, config.DefaultKeyMap.CopyResponse) && rp.Response.Body() != "" {
			if err := clipboard.WriteAll(rp.Response.Body()); err == nil {
				cmd = tea.Batch(cmd, notify("✓ Response body copied", components.Success))
			}
		}
		if rp.Response.Searching() {
			return rp, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		rp.RequestTabs.SetWidth(msg.Width / 2)
		methodWidth, methodHeight := lipgloss.Size(rp.Method.View().Content)
		serverWidth, _ := lipgloss.Size(rp.Server.View().Content)
		sendWidth, _ := lipgloss.Size(rp.Send.View().Content)

		rp.URI.Style = rp.URI.Style.Width(msg.Width - (methodWidth + serverWidth + sendWidth + 3))
		tabsHeight := msg.Height - methodHeight
		rp.RequestTabs.Style = rp.RequestTabs.Style.Height(tabsHeight)

		respWidth := msg.Width - rp.RequestTabs.Width()
		rp.Response.SetSize(max(0, respWidth-2), max(0, tabsHeight-2))
		rp.respStyle = rp.respStyle.Width(respWidth)

		childrenMsg := tea.WindowSizeMsg{Width: rp.RequestTabs.Width() - 2, Height: max(0, tabsHeight-4)}

		model, cmd = rp.RequestTabs.Update(childrenMsg)
		rp.RequestTabs, _ = model.(tabs.Model)

	case tea.KeyPressMsg:
		rp.dirty = true
		if consumed || rp.shouldBlockTabCommands() {
			break
		}

		if (field(rp.fieldsCursor) == method && rp.Method.Open) ||
			(field(rp.fieldsCursor) == serverField && rp.Server.Open) {
			if key.Matches(msg, config.DefaultKeyMap.Next) || key.Matches(msg, config.DefaultKeyMap.Prev) {
				rp.Method.Open = false
				rp.Server.Open = false
				break
			}
		}

		switch {
		case key.Matches(msg, config.DefaultKeyMap.Select) && field(rp.fieldsCursor) == reqTabs:
			rp.BlockTab = true
		case key.Matches(msg, config.DefaultKeyMap.Next) && !rp.BlockTab:
			rp.fieldsCursor = inmath.Circle(rp.fieldsCursor+1, 0, int(lastField))
		case key.Matches(msg, config.DefaultKeyMap.Prev) && !rp.BlockTab:
			rp.fieldsCursor = inmath.Circle(rp.fieldsCursor-1, 0, int(lastField))
		case key.Matches(msg, config.DefaultKeyMap.Save) && !rp.BlockTab:
			cmd = tea.Batch(cmd, store.SaveFile(rp.GetAsRequestData()))
			rp.savedFingerprint = fingerprint(rp.GetAsRequestData())
		case key.Matches(msg, config.DefaultKeyMap.Back) && !rp.BlockTab:
			return rp, closePane(false)
		}
		if field(rp.fieldsCursor) == response {
			rp.Response.Focus()
		} else {
			rp.Response.Blur()
		}
	}

	return rp.autosave(cmd)
}

// autosave writes the temp file only when the request content changed.
func (rp RequestPane) autosave(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	if rp.fileName == "" || !rp.dirty {
		return rp, cmd
	}
	rp.dirty = false
	data := rp.GetAsRequestData()
	if fp := fingerprint(data); fp != rp.savedFingerprint {
		rp.savedFingerprint = fp
		cmd = tea.Batch(cmd, store.SaveTempFile(data))
	}
	return rp, cmd
}

func isKey(msg tea.Msg) bool {
	_, ok := msg.(tea.KeyPressMsg)
	return ok
}
