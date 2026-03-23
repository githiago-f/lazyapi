// Package requesteditor implements editor page for request data
package requesteditor

import (
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

type CloseRequestPaneMsg int

func Close() tea.Cmd {
	return func() tea.Msg {
		return CloseRequestPaneMsg(1)
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
)

const (
	docTab tab = iota
	paramsTab
)

const lastField = reqTabs

type RequestPane struct {
	tea.Model
	fieldsCursor int
	FormData     model.Request

	Method components.MethodSelector
	URI    components.Field
	Send   components.Button

	Tabs tabs.Model
}

var defaultBtnStyle = lipgloss.NewStyle().
	Bold(true).
	Padding(0, 2).
	Border(lipgloss.InnerHalfBlockBorder()).
	Foreground(lipgloss.Color(config.Crust))

func (rp RequestPane) Reset() RequestPane {
	rp.FormData = model.Request{}
	rp.fieldsCursor = 0

	return rp
}

func New() RequestPane {
	return RequestPane{
		FormData: model.Request{},
		URI:      components.InitField("https://example.com/hello_world", ""),
		Method:   components.MethodSelector{Cursor: 0},
		Send: components.Button{
			Label:  "Send",
			Active: true,
			Style: defaultBtnStyle.
				Background(lipgloss.Color(config.Peach)).
				BorderForeground(lipgloss.Color(config.Peach)),
			ClickedStyle: defaultBtnStyle.
				Background(lipgloss.Color(config.Yellow)).
				BorderForeground(lipgloss.Color(config.Yellow)),
			HoverStyle: defaultBtnStyle.
				Background(lipgloss.Color(config.Maroon)).
				BorderForeground(lipgloss.Color(config.Maroon)),
		},

		Tabs: tabs.New(
			tabs.NewTab("Documentation", DocumentationTab(model.About{})),
			tabs.NewTab("Params", ParamsTab()),
			tabs.NewTab("Authorize", AuthorizeTab()),
			tabs.NewTab("Header", HeaderTab()),
			tabs.NewTab("Body", BodyTab()),
			tabs.NewTab("Tests", TestsTab()),
		),
	}
}

func (rp RequestPane) View() string {
	requestURL := lipgloss.JoinHorizontal(
		lipgloss.Left,
		rp.Method.View(),
		rp.URI.View(),
		rp.Send.View(),
	)

	return lipgloss.JoinVertical(lipgloss.Top, requestURL, rp.Tabs.View())
}

func (rp RequestPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case store.LoadedFile:
		rp.FormData = msg.Data
	}

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
		model, cmd = rp.Tabs.Update(msg)
		rp.Tabs, _ = model.(tabs.Model)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Next):
			rp.fieldsCursor = inmath.Cicle(rp.fieldsCursor+1, 0, int(lastField))
		case key.Matches(msg, config.DefaultKeyMap.Prev):
			rp.fieldsCursor = inmath.Cicle(rp.fieldsCursor-1, 0, int(lastField))
		case key.Matches(msg, config.DefaultKeyMap.Back):
			return rp, Close()
		}
	}

	return rp, cmd
}
