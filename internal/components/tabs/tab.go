// Package tabs implements tab component
package tabs

import tea "charm.land/bubbletea/v2"

type Tab struct {
	Label   string
	Content tea.Model
}

func NewTab(label string, content tea.Model) Tab {
	return Tab{Label: label, Content: content}
}
