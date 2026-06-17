package main

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/app"
	"github.com/githiago-f/lazyapi/internal/cli"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "create", "remove", "add", "smoke":
			cli.Run(os.Args[1:])
			return
		}
	}

	defaultFile := ""
	if len(os.Args) > 1 {
		defaultFile = os.Args[1]
	}

	p := tea.NewProgram(app.NewTui(defaultFile))
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
