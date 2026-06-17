// Package editor implements editor page for request data
package editor

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
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

	fileName string

	BlockTab bool

	Method components.MethodSelector
	URI    components.Field
	Send   components.Button

	RequestTabs tabs.Model

	ResponsePreview components.View
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
		URI:    components.InitField("https://example.com/hello_world", ""),
		Method: components.MethodSelector{Cursor: 0},
		Send: components.Button{
			Label:  "Send",
			Active: true,
			Style: defaultBtnStyle.
				Background(lipgloss.Color(config.Flamingo)).
				BorderForeground(lipgloss.Color(config.Flamingo)),
		},

		RequestTabs: tabs.New(
			tabs.NewTab("Documentation", docs),
			tabs.NewTab("Params", params),
			tabs.NewTab("Authorize", authorize),
			tabs.NewTab("Header", header),
			tabs.NewTab("Body", body),
			tabs.NewTab("Tests", tests),
		),

		ResponsePreview: components.View{},
	}
}

func (rp RequestPane) View() string {
	activeColor := config.DefaultConfig.PrimaryColor()

	switch field(rp.fieldsCursor) {
	case method:
		rp.Method.Style = rp.Method.Style.BorderForeground(activeColor)
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
	}

	rp.ResponsePreview.Content = fmt.Sprintf("%s %p", rp.ResponsePreview.Content, &rp.RequestTabs.Tabs[Documentation].Content)

	requestURL := lipgloss.JoinHorizontal(
		lipgloss.Left,
		rp.Method.View(),
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

func (rp RequestPane) SetValue(formData model.Request) RequestPane {
	rp.fileName = formData.FileName
	rp.Method.Cursor = int(formData.Method)
	rp.URI.TextInput.SetValue(formData.URI)

	docs := rp.RequestTabs.Tabs[Documentation].Content.(*documentation)
	*docs = docs.SetValue(formData.About)

	bd := rp.RequestTabs.Tabs[Body].Content.(*body)
	bd.SetValue(formData.Body.Raw)

	hd := rp.RequestTabs.Tabs[Header].Content.(*header)
	hd.SetValue(formData.Headers)

	pr := rp.RequestTabs.Tabs[Params].Content.(*params)
	pr.SetValue(formData.Query, formData.Params)

	return rp
}

func (rp *RequestPane) Reset() {
	rp.Method.Cursor = 0
	rp.fieldsCursor = 0
	rp.URI.TextInput.SetValue("")

	rp.RequestTabs.Cursor = 0
}

func (rp RequestPane) GetAsRequestData() model.Request {
	docs := rp.RequestTabs.Tabs[Documentation].Content.(*documentation)
	bd := rp.RequestTabs.Tabs[Body].Content.(*body)
	hd := rp.RequestTabs.Tabs[Header].Content.(*header)
	pr := rp.RequestTabs.Tabs[Params].Content.(*params)

	return model.Request{
		FileName: rp.fileName,
		About:    docs.Value(),
		URI:      rp.URI.TextInput.Value(),
		Method:   rp.Method.Value(),
		Body: model.Body{
			Type: model.ApplicationJSON,
			Raw:  bd.Value(),
		},
		Headers: hd.Value(),
		Params:  pr.ParamsValue(),
		Query:   pr.QueryValue(),
	}
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
		rp.Method, _ = model.(components.MethodSelector)
	case uri:
		model, cmd = rp.URI.Update(msg)
		rp.URI, _ = model.(components.Field)
	case send:
		model, cmd = rp.Send.Update(msg)
		rp.Send, _ = model.(components.Button)
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
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		rp.RequestTabs.Width = (msg.Width / 2) - 15
		methodWidth, methodHeight := lipgloss.Size(rp.Method.View())
		sendWidth, _ := lipgloss.Size(rp.Send.View())

		rp.URI.Style = rp.URI.Style.Width(msg.Width - (methodWidth + sendWidth + 2))
		tabsHeight := msg.Height - (methodHeight + 5)
		rp.RequestTabs.Style = rp.RequestTabs.Style.Height(tabsHeight)
		rp.ResponsePreview.Style = rp.ResponsePreview.Style.
			Height(tabsHeight).
			Width(msg.Width - (rp.RequestTabs.Width + 5))

		childrenMsg := tea.WindowSizeMsg{Width: rp.RequestTabs.Width - 2, Height: tabsHeight}

		model, cmd = rp.RequestTabs.Update(childrenMsg)
		rp.RequestTabs, _ = model.(tabs.Model)

	case tea.KeyMsg:
		if consumed || rp.shouldBlockTabCommands() {
			break
		}
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Select) && field(rp.fieldsCursor) == reqTabs:
			rp.BlockTab = true
		case key.Matches(msg, config.DefaultKeyMap.Next) && !rp.BlockTab:
			rp.fieldsCursor = inmath.Cicle(rp.fieldsCursor+1, 0, int(lastField))
		case key.Matches(msg, config.DefaultKeyMap.Prev) && !rp.BlockTab:
			rp.fieldsCursor = inmath.Cicle(rp.fieldsCursor-1, 0, int(lastField))
		case key.Matches(msg, config.DefaultKeyMap.Save) && !rp.BlockTab:
			return rp, closePane(true)
		case key.Matches(msg, config.DefaultKeyMap.Back) && !rp.BlockTab:
			return rp, closePane(false)
		}
	}

	if rp.fileName != "" {
		cmd = tea.Batch(cmd, store.SaveTempFile(rp.GetAsRequestData()))
	}

	return rp, cmd
}
