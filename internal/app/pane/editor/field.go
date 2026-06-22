package editor

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/components"
)

var checkStyle = lipgloss.NewStyle().Width(4).Align(lipgloss.Center)

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

func paramFieldsEnabled(fields []paramField) map[string]bool {
	r := make(map[string]bool, len(fields))
	for _, pf := range fields {
		name := pf.name.Value()
		if name != "" {
			r[name] = pf.enabled
		}
	}
	return r
}

func paramFieldsToMap(fields []paramField) map[string]string {
	result := make(map[string]string, len(fields))
	for _, pf := range fields {
		if !pf.enabled {
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

func (pf paramField) View() string {
	indicator := "[ ]"
	if pf.enabled {
		indicator = "[x]"
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		checkStyle.Render(indicator),
		pf.name.View(),
		pf.value.View(),
	)
}

func (pf paramField) Update(_ tea.Msg) (tea.Model, tea.Cmd) {
	return pf, nil
}
