package requests

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
)

// PreviewModel shows the selected request. The TUI renders the response
// below it (see HeaderHeight), so this package stays free of the runner.
type PreviewModel struct {
	item *RequestItem

	servers   []string
	serverURL string
	serverIdx int

	width  int
	height int
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

func NewPreview() PreviewModel {
	return PreviewModel{}
}

func (p *PreviewModel) SetItem(item RequestItem, servers []string, serverURL string) {
	p.item = &item
	p.servers = servers
	p.serverURL = serverURL
	p.serverIdx = 0
	for i, s := range servers {
		if s == serverURL {
			p.serverIdx = i
		}
	}
}

// Item returns the previewed item.
func (p *PreviewModel) Item() *RequestItem { return p.item }

func (p *PreviewModel) CycleServer() {
	if len(p.servers) < 2 {
		return
	}
	p.serverIdx = (p.serverIdx + 1) % len(p.servers)
	p.serverURL = p.servers[p.serverIdx]
}

func (p *PreviewModel) SetServerURL(url string) {
	if url == "" {
		return
	}
	p.serverURL = url
	for i, s := range p.servers {
		if s == url {
			p.serverIdx = i
			return
		}
	}
}

func (p *PreviewModel) CurrentServerURL() string {
	return p.serverURL
}

func (p PreviewModel) Init() tea.Cmd {
	return nil
}

var (
	previewURIStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(config.Text))
	previewSummaryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Subtext0))
	previewServerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay0))
	previewSepStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Surface2))
	previewKeyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender))
	previewHintStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1))
)

func (p PreviewModel) header() string {
	var b strings.Builder
	badge := methodStyle(p.item.Method).Render(p.item.Method.Label())
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", previewURIStyle.Render(p.item.URI)))
	b.WriteString("\n")

	if p.item.About.Summary != "" {
		b.WriteString(previewSummaryStyle.Render(p.item.About.Summary) + "\n")
	}
	if p.serverURL != "" {
		line := "Server: " + p.serverURL
		if len(p.servers) > 1 {
			line += "  (" + previewKeyStyle.Render("o") + previewServerStyle.Render(" to switch)")
		}
		b.WriteString(previewServerStyle.Render(line) + "\n")
	}
	if p.item.About.Description != "" {
		b.WriteString(previewServerStyle.Render(p.item.About.Description) + "\n")
	}

	hint := func(k, label string) string { return previewKeyStyle.Render(k) + " " + previewHintStyle.Render(label) }
	b.WriteString(strings.Join([]string{hint("s", "send"), hint("enter", "edit"), hint("[ ]", "response tab"),
		hint("ctrl+t", "tags")}, previewHintStyle.Render("  ·  ")) + "\n")
	b.WriteString(previewSepStyle.Render(strings.Repeat("─", max(10, p.width-2))))
	return b.String()
}

// HeaderHeight is the number of lines View uses.
func (p PreviewModel) HeaderHeight() int {
	if p.item == nil {
		return 0
	}
	return lipgloss.Height(p.header())
}

// View renders the request header.
func (p PreviewModel) View() tea.View {
	if p.item == nil {
		return tea.NewView(previewHintStyle.Render("  ← Select a request to preview"))
	}
	return tea.NewView(lipgloss.NewStyle().Width(p.width).Render(p.header()))
}

func (p PreviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p, nil
}
