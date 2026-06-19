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
		case "create", "remove", "add", "send", "smoke":
			cli.Run(os.Args[1:])
			return
		}
	}

	var defaultFile, envFile string
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--env":
			i++
			if i < len(os.Args) {
				envFile = os.Args[i]
			}
		default:
			if defaultFile == "" {
				defaultFile = os.Args[i]
			}
		}
	}

	p := tea.NewProgram(app.NewTui(defaultFile, envFile), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
