package main

import (
	"github.com/githiago-f/lazyapi/internals/config"
	"github.com/githiago-f/lazyapi/internals/tui"
	"github.com/rivo/tview"
)

func main() {
	conf := config.LoadConfigFile()

	app := tview.NewApplication()

	request := tview.NewBox()

	requestLayout := tview.NewFlex().SetDirection(0)
	requestLayout.AddItem(request, 0, 2, false)
	requestLayout.AddItem(tui.View.ResponseView, 0, 1, false)

	layout := tview.NewFlex()

	layout.AddItem(tui.NewExplorer(conf), 0, 1, true)
	layout.AddItem(requestLayout, 0, 2, false)

	if err := app.SetRoot(layout, true).Run(); err != nil {
		panic(err)
	}
}
