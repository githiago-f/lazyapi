package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/githiago-f/lazyapi/internal/app"
	"github.com/githiago-f/lazyapi/internal/cli"
)

func main() {
	cli.SetTUIStarter(func(file, env string) {
		p := tea.NewProgram(app.NewTui(file, env), tea.WithMouseCellMotion())
		if _, err := p.Run(); err != nil {
			panic(err)
		}
	})
	cli.Execute()
}
