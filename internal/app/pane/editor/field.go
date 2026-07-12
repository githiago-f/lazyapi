package editor

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/components"
)

type paramField struct {
	enabled bool
	name    components.Field
	value   components.Field
}

func createParam() paramField {
	return paramField{
		enabled: true,
		name:    components.InitField("Name", ""),
		value:   components.InitField("Value", ""),
	}
}

func createParamWith(name, value string) paramField {
	return paramField{
		enabled: true,
		name:    components.InitField("Name", name),
		value:   components.InitField("Value", value),
	}
}

func mapToParamFields(m map[string]string) []paramField {
	result := make([]paramField, 0, len(m))
	for k, v := range m {
		result = append(result, createParamWith(k, v))
	}
	return result
}

func paramFieldsToMap(fields []paramField) map[string]string {
	result := make(map[string]string, len(fields))
	for _, pf := range fields {
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
