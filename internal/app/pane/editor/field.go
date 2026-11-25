package editor

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
)

type paramField struct {
	enabled bool
	name    components.Field
	value   components.Field
}

func createParam() paramField {
	return paramField{
		enabled: true,
		name:    components.InitField(config.T("editor.grid.name"), ""),
		value:   components.InitField(config.T("editor.grid.value"), ""),
	}
}

func createParamWith(name, value string) paramField {
	return paramField{
		enabled: true,
		name:    components.InitField(config.T("editor.grid.name"), name),
		value:   components.InitField(config.T("editor.grid.value"), value),
	}
}

// paramFieldsToMap returns the named rows that are (or are not) enabled.
func paramFieldsToMap(fields []paramField, enabled bool) map[string]string {
	result := make(map[string]string, len(fields))
	for _, pf := range fields {
		if pf.enabled != enabled {
			continue
		}
		name := pf.name.Value()
		value := pf.value.Value()
		if name != "" {
			result[name] = value
		}
	}
	return result
}

func (pf paramField) Init() tea.Cmd {
	return nil
}

func (pf paramField) View() tea.View {
	return tea.NewView(lipgloss.JoinHorizontal(lipgloss.Top, pf.name.View().Content, pf.value.View().Content))
}

func (pf paramField) Update(_ tea.Msg) (tea.Model, tea.Cmd) {
	return pf, nil
}
