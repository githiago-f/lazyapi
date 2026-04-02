// Package tabs define a tab component
package tabs

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Tab struct {
	label string

	Active  bool
	Content tea.Model
}

func NewTab(label string, content tea.Model) Tab {
	return Tab{label: label, Content: content}
}
