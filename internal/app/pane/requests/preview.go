package requests

import (
	"fmt"
	"net/http"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/response"
)

type PreviewModel struct {
	item    *RequestItem
	request *model.Request

	responseView viewport.Model
	contentWidth int

	lastStatusCode int
	lastStatus     string
	lastHeader     http.Header
	lastBody       string

	serverIdx int
	sending   bool
	err       string
	width     int
	height    int
}

func (p *PreviewModel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *PreviewModel) HasItem() bool {
	return p.item != nil
}

func (p *PreviewModel) CurrentItemURI() string {
	if p.item == nil {
		return ""
	}
	return p.item.URI
}

func (p *PreviewModel) CurrentItemMethod() model.Method {
	if p.item == nil {
		return model.GET
	}
	return p.item.Method
}

func (p *PreviewModel) SetViewportSize(w, h int) {
	p.responseView.SetWidth(max(0, w))
	p.responseView.SetHeight(max(0, h))
}

func NewPreview() PreviewModel {
	return PreviewModel{
		responseView: viewport.New(viewport.WithWidth(0), viewport.WithHeight(0)),
	}
}

func (p *PreviewModel) SetItem(item RequestItem, servers []string, serverURL string) {
	p.item = &item
	p.serverIdx = 0
	p.sending = false
	p.err = ""

	if item.OpenAPIRef != nil {
		req := model.Request{
			FileName:  item.FileName,
			URI:       item.URI,
			Method:    item.Method,
			About:     item.About,
			Tags:      item.Tags,
			ServerURL: serverURL,
			Servers:   servers,
			Body:      model.Body{Type: model.ApplicationJSON, Raw: ""},
			Headers:   map[string]string{},
			Params:    map[string]string{},
			Query:     map[string]string{},
		}
		p.request = &req
	} else {
		p.request = nil
	}
}

func (p *PreviewModel) CycleServer() {
	if p.request == nil || len(p.request.Servers) < 2 {
		return
	}
	p.serverIdx = (p.serverIdx + 1) % len(p.request.Servers)
	p.request.ServerURL = p.request.Servers[p.serverIdx]
}

func (p *PreviewModel) SetServerURL(url string) {
	if p.request == nil || url == "" {
		return
	}
	p.request.ServerURL = url
	for i, s := range p.request.Servers {
		if s == url {
			p.serverIdx = i
			return
		}
	}
}

func (p *PreviewModel) CurrentServerURL() string {
	if p.request == nil {
		return ""
	}
	return p.request.ServerURL
}

func (p *PreviewModel) Send() tea.Cmd {
	if p.request == nil {
		return nil
	}
	p.sending = true
	p.err = ""
	return func() tea.Msg {
		resp, body, err := p.request.Send()
		if err != nil {
			return model.FailureMsg{Message: err.Error()}
		}
		return model.SuccessMsg{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Header:     resp.Header.Clone(),
			Body:       body,
		}
	}
}

func (p *PreviewModel) SetResponse(statusCode int, status string, header http.Header, body string) {
	p.lastStatusCode = statusCode
	p.lastStatus = status
	p.lastHeader = header
	p.lastBody = body
	p.sending = false
	p.err = ""

	content := response.BuildContent(statusCode, status, header, body, p.contentWidth)
	p.responseView.SetContent(content)
}

func (p *PreviewModel) SetResponseError(err string) {
	p.err = err
	p.sending = false
	p.lastStatusCode = 0
	p.lastHeader = nil
	p.lastBody = ""
	p.responseView.SetContent(fmt.Sprintf("Error: %s", err))
}

func (p PreviewModel) Init() tea.Cmd {
	return nil
}

var (
	previewURIStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(config.Text))
	previewSummaryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Subtext0))
	previewServerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay0))
	previewSepStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Surface2))
	previewKeyHintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1))
	respLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(config.Peach))
)

func (p PreviewModel) View() tea.View {
	if p.item == nil {
		return tea.NewView("  ← Select a request to preview")
	}

	var b strings.Builder

	badge := p.item.methodStyle().Render(p.item.Method.Label())
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", previewURIStyle.Render(p.item.URI)))
	b.WriteString("\n")

	if p.item.About.Summary != "" {
		b.WriteString(previewSummaryStyle.Render(p.item.About.Summary))
		b.WriteString("\n")
	}

	if p.request != nil && p.request.ServerURL != "" {
		serverLine := "Server: " + p.request.ServerURL
		if len(p.request.Servers) > 1 {
			serverLine = "◀ " + serverLine + " ▶"
		}
		b.WriteString(previewServerStyle.Render(serverLine))
		b.WriteString("\n")
	}

	if p.item.About.Description != "" {
		b.WriteString("\n")
		b.WriteString(previewServerStyle.Render(p.item.About.Description))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(previewKeyHintStyle.Render("[s] Send  [o] Server  [Ctrl+T] Tags"))
	b.WriteString("\n")

	sep := strings.Repeat("─", max(10, p.width-2))
	b.WriteString(previewSepStyle.Render(sep))
	b.WriteString("\n")

	if p.sending {
		b.WriteString("  Sending request...")
	} else if p.err != "" {
		b.WriteString(previewServerStyle.Render("Error: " + p.err))
	} else if p.lastStatusCode > 0 {
		b.WriteString(respLabelStyle.Render("Response:"))
		b.WriteString("\n\n")
		b.WriteString(p.responseView.View())
	} else {
		b.WriteString(previewServerStyle.Render("  Press [s] to send this request"))
	}

	view := b.String()
	if p.width > 0 {
		view = lipgloss.NewStyle().Width(p.width).Render(view)
	}
	return tea.NewView(view)
}

func (p PreviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.contentWidth = max(0, p.width-2)
		p.responseView.SetWidth(max(0, p.contentWidth))
		p.responseView.SetHeight(max(0, p.height-12))

	case model.SuccessMsg:
		p.SetResponse(msg.StatusCode, msg.Status, msg.Header, msg.Body)

	case model.FailureMsg:
		p.SetResponseError(msg.Message)
	}

	p.responseView, cmd = p.responseView.Update(msg)
	return p, cmd
}
