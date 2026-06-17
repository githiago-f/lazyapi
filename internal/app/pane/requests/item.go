package requests

import (
	"fmt"
	"math"
	"strconv"
	"strings"

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
	About       model.About  `yaml:"about"`
	FileName    string
	RequestTime float64 `yaml:"request_time"`
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

func getDecimalPart(f float64) string {
	_, decimal := math.Modf(f)
	formatted := fmt.Sprintf("%.10g", decimal)
	if strings.Contains(formatted, ".") {
		return formatted[2:]
	}
	return formatted
}

func (ri RequestItem) Description() string {
	metric := ""

	if ri.RequestTime >= 0.0001 {
		decimal := getDecimalPart(ri.RequestTime)
		value, err := strconv.Atoi(decimal)
		if err != nil {
			return lipgloss.JoinHorizontal(lipgloss.Left, ri.About.Summary)
		}

		switch {
		case value == 0:
			metric = fmt.Sprintf(" - %.0f", ri.RequestTime)
		case value >= 1:
			metric = fmt.Sprintf(" - %.1f", ri.RequestTime)
		case value >= 10:
			metric = fmt.Sprintf(" - %.2f", ri.RequestTime)
		case value >= 100:
			metric = fmt.Sprintf(" - %.3f", ri.RequestTime)
		case value >= 1000:
			metric = fmt.Sprintf(" - %.4f", ri.RequestTime)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, ri.About.Summary, metric)
}

func (ri RequestItem) FilterValue() string {
	return fmt.Sprintf("%s %s", ri.URI, ri.About.Summary)
}
