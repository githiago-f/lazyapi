package requests

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
)

var anyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(config.Crust)).
	Bold(true).
	Padding(0, 1).
	Width(9).
	Align(lipgloss.Center, lipgloss.Center)

// RequestItem is the list entry (data lives in model so store does not
// depend on this UI package).
type RequestItem = model.RequestItem

// methodStyle is the badge style of an HTTP method (config.MethodColor).
func methodStyle(m model.Method) lipgloss.Style {
	return anyStyle.Background(lipgloss.Color(config.MethodColor(m.Label())))
}

var (
	selItemBg   = lipgloss.NewStyle().Background(lipgloss.Color(config.Surface1))
	hoverItemBg = lipgloss.NewStyle().Background(lipgloss.Color(config.Surface0))
	draftPre    = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay0)).SetString("✎ ")
	sumStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay0))
)

// RenderCompact renders an item as one list line: selected rows get the
// selection background, hovered ones (mouse pointer over them) a lighter one.
func RenderCompact(ri RequestItem, width int, selected, hovered bool) string {
	methodLabel := ri.Method.Label()
	if methodLabel == "" {
		methodLabel = "ANY"
	}

	badge := methodStyle(ri.Method).Render(methodLabel)
	uriText := ri.URI
	if ri.DraftPath != "" {
		uriText = draftPre.String() + uriText
		if uriText == "" {
			uriText = draftPre.String() + "<new>"
		}
	}

	summary := ri.About.Summary
	if summary != "" {
		summary = "  " + sumStyle.Render(summary)
	}

	line := badge + " " + uriText + summary
	if width > 2 {
		line = ansi.Truncate(line, width-2, "…") // leave room for the cursor prefix
	}

	switch {
	case selected:
		line = selItemBg.Render(line)
	case hovered:
		line = hoverItemBg.Render(line)
	}
	return line
}
