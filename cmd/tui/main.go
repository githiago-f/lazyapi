package main

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/app"
)

func main() {
	defaultFile := ""
	if len(os.Args) > 1 {
		defaultFile = os.Args[1]
	}

	p := tea.NewProgram(app.NewTui(defaultFile))

	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
