package requests

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
)

var (
	anyStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(config.Overlay2)).
			Foreground(lipgloss.Color(config.Crust)).
			Bold(true).
			Padding(0, 1).
			Width(9).
			Align(lipgloss.Center, lipgloss.Center)
	getStyle = anyStyle.
			Background(lipgloss.Color(config.Saphire)).
			Foreground(lipgloss.Color(config.Crust))
	postStyle = anyStyle.
			Background(lipgloss.Color(config.Green)).
			Foreground(lipgloss.Color(config.Crust))
	patchStyle = anyStyle.
			Background(lipgloss.Color(config.Yellow)).
			Foreground(lipgloss.Color(config.Crust))
	deleteStyle = anyStyle.
			Background(lipgloss.Color(config.Red)).
			Foreground(lipgloss.Color(config.Crust))
	putStyle = anyStyle.
			Background(lipgloss.Color(config.Yellow)).
			Foreground(lipgloss.Color(config.Crust))
)

type RequestItem struct {
	Method      model.Method
	URI         string
	About       model.About
	FileName    string
	Tags        []string
	RequestTime float64
	DraftPath   string

	OpenAPIRef *model.OpenAPIRef
}

func (ri RequestItem) methodStyle() lipgloss.Style {
	switch ri.Method {
	case model.GET:
		return getStyle
	case model.POST:
		return postStyle
	case model.PATCH:
		return patchStyle
	case model.PUT:
		return putStyle
	case model.DELETE:
		return deleteStyle
	default:
		return anyStyle
	}
}

var (
	selItemBg = lipgloss.NewStyle().Background(lipgloss.Color(config.Surface1))
	draftPre  = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay0)).SetString("✎ ")
	sumStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay0))
)

func (ri RequestItem) RenderCompact(width int, selected bool) string {
	methodLabel := ri.Method.Label()
	if methodLabel == "" {
		methodLabel = "ANY"
	}

	badge := ri.methodStyle().Render(methodLabel)
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

	if selected {
		line = selItemBg.Render(line)
	}

	_ = width // reserved for future truncation
	return line
}

func (ri RequestItem) FilterValue() string {
	return fmt.Sprintf("%s %s %s", ri.URI, ri.About.Summary, strings.Join(ri.Tags, " "))
}
