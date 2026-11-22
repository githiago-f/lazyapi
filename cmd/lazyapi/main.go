package main

import (
	"os"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/githiago-f/lazyapi/internal/app"
	"github.com/githiago-f/lazyapi/internal/cli"
	"github.com/githiago-f/lazyapi/internal/components"
)

func main() {
	if len(os.Args) > 1 && (slices.Contains(cli.Commands, os.Args[1]) || os.Args[1] == "-h" || os.Args[1] == "--help") {
		cli.Run(os.Args[1:])
		return
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

	components.Z.NewGlobal()
	defer components.Z.Close()

	p := tea.NewProgram(app.NewTui(defaultFile, envFile))
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
