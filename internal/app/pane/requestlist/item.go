package requestlist

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
)

var (
	anyStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(config.Overlay2)).
			Foreground(lipgloss.Color(config.Crust)).
			Bold(true).
			Padding(0, 2).
			Width(10).
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
	Method      model.Method `yaml:"method"`
	URI         string       `yaml:"uri"`
	Summary     string       `yaml:"summary"`
	FileName    string
	RequestTime float32 `yaml:"request_time"`
}

func (ri RequestItem) Title() string {
	var methodStyle lipgloss.Style
	switch ri.Method {
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
	return lipgloss.JoinHorizontal(lipgloss.Left, methodStyle.Render(ri.Method.Label()), " ", ri.URI)
}

func (ri RequestItem) Description() string {
	return lipgloss.JoinHorizontal(lipgloss.Left, ri.Summary, " - ", fmt.Sprintf("%.1fms", ri.RequestTime))
}

func (ri RequestItem) FilterValue() string {
	return fmt.Sprintf("%s %s", ri.URI, ri.Summary)
}
