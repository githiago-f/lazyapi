package requestlist

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
)

var (
	getStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(config.Saphire)).
			Foreground(lipgloss.Color(config.Crust)).
			Bold(true).
			Padding(0, 4, 0, 1)
	postStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(config.Green)).
			Foreground(lipgloss.Color(config.Crust)).
			Bold(true).
			Padding(0, 3, 0, 1)
	patchStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(config.Yellow)).
			Foreground(lipgloss.Color(config.Crust)).
			Bold(true).
			Padding(0, 2, 0, 1)
	deleteStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(config.Red)).
			Foreground(lipgloss.Color(config.Crust)).
			Bold(true).
			Padding(0, 1)
	putStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(config.Yellow)).
			Foreground(lipgloss.Color(config.Crust)).
			Bold(true).
			Padding(0, 4, 0, 1)
	anyStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(config.Overlay2)).
			Foreground(lipgloss.Color(config.Crust)).
			Bold(true).
			Padding(0, 1)
)

type RequestItem struct {
	method       model.Method
	uri, summary string
	requestTime  float32
}

func Item(method model.Method, uri, summary string, requestTime float32) RequestItem {
	return RequestItem{
		method:      method,
		uri:         uri,
		summary:     summary,
		requestTime: requestTime,
	}
}

func (ri RequestItem) Title() string {
	var methodStyle lipgloss.Style
	switch ri.method {
	case model.GET:
		methodStyle = getStyle
	case model.POST:
		methodStyle = postStyle
	case model.PATCH:
		methodStyle = patchStyle
	case model.PUT:
		methodStyle = putStyle
	case model.DELETE:
		methodStyle = deleteStyle
	default:
		methodStyle = anyStyle
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, methodStyle.Render(ri.method.Label()), " ", ri.uri)
}

func (ri RequestItem) Description() string {
	return lipgloss.JoinHorizontal(lipgloss.Left, ri.summary, " - ", fmt.Sprintf("%.1fms", ri.requestTime))
}

func (ri RequestItem) FilterValue() string {
	return fmt.Sprintf("%s %s", ri.uri, ri.summary)
}
