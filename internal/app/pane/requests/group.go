package requests

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
)

type GroupHeader struct {
	Resource string
}

func (h GroupHeader) Title() string       { return h.Resource }
func (h GroupHeader) Description() string { return "" }
func (h GroupHeader) FilterValue() string { return "" }

type TreeDelegate struct{}

func (d TreeDelegate) Height() int {
	return 2
}

func (d TreeDelegate) Spacing() int {
	return 1
}

func (d TreeDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d TreeDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	switch item := item.(type) {
	case GroupHeader:
		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(config.Peach)).
			Padding(0, 1)
		_, _ = fmt.Fprintf(w, "%s", headerStyle.Render(item.Resource))

	case RequestItem:
		isSelected := index == m.Index()
		indent := lipgloss.NewStyle().PaddingLeft(2)

		var methodStyle lipgloss.Style
		switch item.Method {
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

		title := lipgloss.JoinHorizontal(
			lipgloss.Left,
			methodStyle.Render(item.Method.Label()),
			" ",
			item.URI,
		)

		if isSelected {
			selStyle := lipgloss.NewStyle().
				Background(lipgloss.Color(config.Surface0)).
				Padding(0, 1)
			_, _ = fmt.Fprintf(w, "%s", selStyle.Render(indent.Render(title)))
		} else {
			_, _ = fmt.Fprintf(w, "%s", indent.Render(title))
		}

		if item.About.Summary != "" {
			_, _ = fmt.Fprintf(w, "\n")
			descStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(config.Overlay0)).
				PaddingLeft(2)
			_, _ = fmt.Fprintf(w, "%s", descStyle.Render(item.About.Summary))
		}
	}
}

func resourceRoot(path string) string {
	trimmed := strings.TrimPrefix(path, "/")
	segments := strings.SplitN(trimmed, "/", 2)
	if trimmed == "" || segments[0] == "" {
		return "/"
	}
	return "/" + segments[0]
}

func GroupByResource(items []list.Item) []list.Item {
	if len(items) == 0 {
		return items
	}

	var result []list.Item
	var lastRoot string
	for _, item := range items {
		ri, ok := item.(RequestItem)
		if !ok {
			result = append(result, item)
			continue
		}
		root := resourceRoot(ri.URI)
		if root != lastRoot {
			result = append(result, GroupHeader{Resource: root})
			lastRoot = root
		}
		result = append(result, item)
	}
	return result
}
