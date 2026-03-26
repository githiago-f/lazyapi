// Package requesteditor implements editor page for request data
package requesteditor

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/app/pane/responses"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/inmath"
	"github.com/githiago-f/lazyapi/internal/model"
)

type CloseRequestPaneMsg struct {
	SaveToFile bool
	File       string
}

func Close() tea.Cmd {
	return func() tea.Msg {
		return CloseRequestPaneMsg{
			SaveToFile: false,
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

	BlockTab bool

	Method components.MethodSelector
	URI    components.Field
	Send   components.Button

	RequestTabs tabs.Model

	ResponsePreview responses.ResponseView
}

var defaultBtnStyle = lipgloss.NewStyle().
	Bold(true).
	Padding(0, 2).
	Border(lipgloss.InnerHalfBlockBorder()).
	Foreground(lipgloss.Color(config.Crust))

func New() RequestPane {
	return RequestPane{
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
			tabs.NewTab("Documentation", DocumentationTab()),
			tabs.NewTab("Params", ParamsTab()),
			tabs.NewTab("Authorize", AuthorizeTab()),
			tabs.NewTab("Header", HeaderTab()),
			tabs.NewTab("Body", BodyTab()),
			tabs.NewTab("Tests", TestsTab()),
		),

		ResponsePreview: responses.New(),
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
	rp.Method.Cursor = int(formData.Method)
	rp.URI.TextInput.SetValue(formData.URI)

	docsTab := rp.RequestTabs.Tabs[Documentation].Content

	if m, ok := docsTab.(tabs.StatefulInput[model.About]); ok {
		rp.RequestTabs.Tabs[Documentation].Content = m.SetValue(formData.About)
	}

	return rp
}

func (rp RequestPane) Reset() RequestPane {
	rp.Method.Cursor = 0
	rp.fieldsCursor = 0
	rp.URI.TextInput.SetValue("")

	docsTab := rp.RequestTabs.Tabs[Documentation].Content

	if m, ok := docsTab.(tabs.StatefulInput[model.About]); ok {
		rp.RequestTabs.Tabs[Documentation].Content = m.SetValue(model.About{})
	}

	return rp
}

func (rp RequestPane) GetAsRequestData() model.Request {
	return model.Request{
		URI:       rp.URI.TextInput.Value(),
		Method:    rp.Method.Value(),
		About:     model.About{},
		Time:      0.0,
		Responses: []model.Response{},
		Body:      model.Body{},
		FileName:  "",
	}
}

func (rp RequestPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd   tea.Cmd
		model tea.Model
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
		model, cmd = rp.RequestTabs.Update(msg)
		rp.RequestTabs, _ = model.(tabs.Model)
	case response:
		model, cmd = rp.ResponsePreview.Update(msg)
		rp.ResponsePreview, _ = model.(responses.ResponseView)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		rp.RequestTabs.Width = (msg.Width / 2) - 15
		methodWidth, methodHeight := lipgloss.Size(rp.Method.View())
		sendWidth, _ := lipgloss.Size(rp.Send.View())

		rp.URI.Style = rp.URI.Style.Width(msg.Width - (methodWidth + sendWidth + 2))
		tabsHeight := msg.Height - (methodHeight + 4)
		rp.RequestTabs.Style = rp.RequestTabs.Style.Height(tabsHeight)
		rp.ResponsePreview.Style = rp.ResponsePreview.Style.
			Height(tabsHeight).
			Width(msg.Width - (rp.RequestTabs.Width + 4))

		childrenMsg := tea.WindowSizeMsg{Width: rp.RequestTabs.Width - 2, Height: tabsHeight}

		model, cmd = rp.RequestTabs.Update(childrenMsg)
		rp.RequestTabs, _ = model.(tabs.Model)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Select):
			rp.BlockTab = true
		case key.Matches(msg, config.DefaultKeyMap.Back) && rp.BlockTab:
			rp.BlockTab = false
		case key.Matches(msg, config.DefaultKeyMap.Next) && !rp.BlockTab:
			rp.fieldsCursor = inmath.Cicle(rp.fieldsCursor+1, 0, int(lastField))
		case key.Matches(msg, config.DefaultKeyMap.Prev) && !rp.BlockTab:
			rp.fieldsCursor = inmath.Cicle(rp.fieldsCursor-1, 0, int(lastField))
		case key.Matches(msg, config.DefaultKeyMap.Back) && !rp.BlockTab:
			return rp, Close()
		}
	}

	return rp, cmd
}
