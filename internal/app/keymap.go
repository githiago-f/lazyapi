// Package app implements the main model and it's configurations
package app

import tea "github.com/charmbracelet/bubbletea"

type KeyMap struct{}

func (k KeyMap) Quit() string {
	return "q"
}

func (k KeyMap) Kill() string {
	return tea.KeyCtrlC.String()
}
