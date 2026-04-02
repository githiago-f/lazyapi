package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/app"
)

func main() {
	p := tea.NewProgram(app.NewTui())

	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
