package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/githiago-f/lazyapi/internals/config"
	"github.com/rivo/tview"
)

const (
	activeBg = tcell.ColorRebeccaPurple
)

func NewExplorer(configs *config.ConfigFile) *tview.List {
	collections := tview.NewList().ShowSecondaryText(false)
	collections.SetTitle("Collections")

	collections.SetBorder(true).SetBorderColor(activeBg)

	for _, req := range configs.Requests {
		name := []string{req.Method, req.Info.Title}
		collections.AddItem(strings.Join(name, " "), "", 0, func() {
		})
	}

	collections.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'a':
			View.Handle("\"Print A\"", 200)
		case 'd':
			View.Handle("\"Print B\"", 500)
		}

		return event
	})

	return collections
}
