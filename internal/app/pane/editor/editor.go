// Package editor implements editor page for request data
package editor

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/scrollable"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/inmath"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

type CloseRequestPaneMsg struct {
	SaveToFile bool
	File       string
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

	Method components.Selector
	Server components.Selector
	URI    components.Field
	Send   components.Button

	RequestTabs tabs.Model

	ResponsePreview viewport.Model

	lastStatusCode int
	lastHeader     http.Header
	lastBody       string
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
			Width:  9,
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
			tabs.NewTab("Documentation", docs),
			tabs.NewTab("Params", scrollable.New(params)),
			tabs.NewTab("Authorize", scrollable.New(authorize)),
			tabs.NewTab("Header", scrollable.New(header)),
			tabs.NewTab("Body", body),
			tabs.NewTab("Tests", scrollable.New(tests)),
		),

		ResponsePreview: viewport.New(0, 0),
	}
}

func (rp RequestPane) View() string {
	activeColor := config.DefaultConfig.PrimaryColor()

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
		if rp.BlockTab {
			rp.RequestTabs.Tabs[rp.RequestTabs.Cursor].Active = true
		}
		rp.RequestTabs.Style = rp.RequestTabs.Style.BorderForeground(activeColor)
	case response:
		rp.ResponsePreview.Style = rp.ResponsePreview.Style.BorderForeground(activeColor)
		rp.ResponsePreview.Width = max(0, rp.ResponsePreview.Width)
		rp.ResponsePreview.Height = max(0, rp.ResponsePreview.Height)
	}

	requestURL := lipgloss.JoinHorizontal(
		lipgloss.Left,
		rp.Method.View(),
		rp.Server.View(),
		rp.URI.View(),
		rp.Send.View(),
	)

	return lipgloss.JoinVertical(
		lipgloss.Top,
		requestURL,
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			rp.RequestTabs.View(),
			rp.ResponsePreview.View(),
		),
	)
}

func (rp *RequestPane) SetEnvStore(s *env.Store) {
	rp.envStore = s
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

	return rp
}

func (rp *RequestPane) Reset() {
	rp.Method.Cursor = 0
	rp.Server = components.Selector{Width: 45}
	rp.fieldsCursor = 0
	rp.URI.TextInput.SetValue("")
	rp.openAPIRef = nil
	rp.draftPath = ""
	rp.lastStatusCode = 0
	rp.lastHeader = nil
	rp.lastBody = ""

	rp.RequestTabs.Cursor = 0
}

func (rp RequestPane) GetAsRequestData() model.Request {
	docs := rp.RequestTabs.Tabs[Documentation].Content.(*documentation)
	bd := rp.RequestTabs.Tabs[Body].Content.(*body)
	hd := rp.RequestTabs.Tabs[Header].Content.(scrollable.Model).Content.(*header)
	pr := rp.RequestTabs.Tabs[Params].Content.(scrollable.Model).Content.(*params)
	au := rp.RequestTabs.Tabs[Authorize].Content.(scrollable.Model).Content.(*auth)

	envMap, _ := rp.envStore.Load()

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
		Env:     envMap,
	}
}

func (rp RequestPane) SetResponse(statusCode int, status string, header http.Header, body string) RequestPane {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "Status: %d %s\n\n", statusCode, status)

	if len(header) > 0 {
		b.WriteString("--- Headers ---\n")
		var names []string
		for name := range header {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			_, _ = fmt.Fprintf(&b, "  %s: %s\n", name, header.Get(name))
		}
		b.WriteString("\n")
	}

	b.WriteString("--- Body ---\n")
	b.WriteString(body)

	rp.ResponsePreview.SetContent(b.String())
	rp.lastStatusCode = statusCode
	rp.lastHeader = header
	rp.lastBody = body
	return rp
}

func (rp RequestPane) SetResponseFeedback(feedback string) RequestPane {
	rp.ResponsePreview.SetContent(feedback + "\n" + rp.ResponsePreview.View())
	return rp
}

func (rp RequestPane) SetResponseError(err string) RequestPane {
	rp.ResponsePreview.SetContent(fmt.Sprintf("Error: %s", err))
	rp.lastStatusCode = 0
	rp.lastHeader = nil
	rp.lastBody = ""
	return rp
}

func (rp RequestPane) FocusResponse() RequestPane {
	rp.fieldsCursor = int(response)
	return rp
}

func (rp RequestPane) LastResponse() (statusCode int, header http.Header, body string) {
	return rp.lastStatusCode, rp.lastHeader, rp.lastBody
}

func (rp RequestPane) CurrentRef() *model.OpenAPIRef {
	return rp.openAPIRef
}

func (rp RequestPane) shouldBlockTabCommands() bool {
	return field(rp.fieldsCursor) == reqTabs && rp.BlockTab
}

func (rp RequestPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd      tea.Cmd
		model    tea.Model
		consumed bool
	)
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
			req := rp.GetAsRequestData()
			cmd = tea.Batch(cmd, req.RunRequest())
		}
	case reqTabs:
		// Two-stage Esc: check if content is active before the tabs update
		wasActive := false
		if rp.BlockTab {
			if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, config.DefaultKeyMap.Back) {
				content := rp.RequestTabs.Tabs[rp.RequestTabs.Cursor].Content
				if a, ok := content.(interface{ IsActive() bool }); ok {
					wasActive = a.IsActive()
				}
			}
		}

		model, cmd = rp.RequestTabs.Update(msg)
		rp.RequestTabs, _ = model.(tabs.Model)

		// Only exit tab if content was NOT active (Esc already blurred it)
		if rp.BlockTab && !wasActive {
			if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, config.DefaultKeyMap.Back) {
				rp.BlockTab = false
				consumed = true
			}
		}

	case response:
		rp.ResponsePreview, cmd = rp.ResponsePreview.Update(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		rp.RequestTabs.Width = (msg.Width / 2) - 15
		methodWidth, methodHeight := lipgloss.Size(rp.Method.View())
		serverWidth, _ := lipgloss.Size(rp.Server.View())
		sendWidth, _ := lipgloss.Size(rp.Send.View())

		rp.URI.Style = rp.URI.Style.Width(msg.Width - (methodWidth + serverWidth + sendWidth + 3))
		tabsHeight := msg.Height - (methodHeight + 5)
		rp.RequestTabs.Style = rp.RequestTabs.Style.Height(tabsHeight)

		respWidth := msg.Width - (rp.RequestTabs.Width + 5)
		rp.ResponsePreview.Width = max(0, respWidth-2)
		rp.ResponsePreview.Height = max(0, tabsHeight-2)
		rp.ResponsePreview.Style = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			Width(respWidth).
			Height(tabsHeight)

		childrenMsg := tea.WindowSizeMsg{Width: rp.RequestTabs.Width - 2, Height: tabsHeight}

		model, cmd = rp.RequestTabs.Update(childrenMsg)
		rp.RequestTabs, _ = model.(tabs.Model)

	case tea.KeyMsg:
		rp.dirty = true
		if consumed || rp.shouldBlockTabCommands() {
			break
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
		case key.Matches(msg, config.DefaultKeyMap.Back) && !rp.BlockTab:
			return rp, closePane(false)
		}
	}

	if rp.fileName != "" && rp.dirty {
		rp.dirty = false
		cmd = tea.Batch(cmd, store.SaveTempFile(rp.GetAsRequestData()))
	}

	return rp, cmd
}
