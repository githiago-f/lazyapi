package config

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type KeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding

	Select    key.Binding
	Filter    key.Binding
	Delete    key.Binding
	New       key.Binding
	CreateNew key.Binding
	Duplicate key.Binding
	Ok        key.Binding

	Back key.Binding
	Save key.Binding

	Next key.Binding
	Prev key.Binding

	Quit key.Binding
	Kill key.Binding

	Help key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	if DefaultConfig.Active == RequestEditor {
		return []key.Binding{k.Back, k.Save, k.Next, k.Prev, k.Help}
	}

	return []key.Binding{k.Select, k.Up, k.Down, k.Filter, k.CreateNew, k.Help}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	if DefaultConfig.Active == RequestEditor {
		return [][]key.Binding{
			{k.Next, k.Prev},
			{k.Back, k.Save, k.Kill},
		}
	}

	return [][]key.Binding{
		{k.Select, k.Up, k.Down, k.Filter, k.CreateNew},
		{k.Duplicate, k.Delete, k.Quit, k.Kill},
	}
}

var DefaultKeyMap = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("↑/k", "move up"),
	),
	Left: key.NewBinding(
		key.WithKeys("h", tea.KeyLeft.String()),
		key.WithHelp("h", "move left"),
	),
	Right: key.NewBinding(
		key.WithKeys("l", tea.KeyRight.String()),
		key.WithHelp("l", "move right"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Select: key.NewBinding(
		key.WithKeys(tea.KeyEnter.String(), tea.KeySpace.String()),
		key.WithHelp("enter/space", "select"),
	),
	Ok: key.NewBinding(
		key.WithKeys(tea.KeyEnter.String()),
		key.WithHelp("enter", "Accept"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
	Kill: key.NewBinding(
		key.WithKeys(tea.KeyCtrlC.String()),
		key.WithHelp("ctrl+c", "exit"),
	),
	Next: key.NewBinding(
		key.WithKeys(tea.KeyTab.String()),
		key.WithHelp("tab", "next"),
	),
	Prev: key.NewBinding(
		key.WithKeys(tea.KeyShiftTab.String()),
		key.WithHelp("shift+tab", "prev"),
	),
	Back: key.NewBinding(
		key.WithKeys(tea.KeyEsc.String()),
		key.WithHelp("esc", "back"),
	),
	Save: key.NewBinding(
		key.WithKeys(tea.KeyCtrlS.String()),
		key.WithHelp("ctrl+s", "save"),
	),
	New: key.NewBinding(
		key.WithKeys("n", "+"),
		key.WithHelp("+/n", "add"),
	),
	CreateNew: key.NewBinding(
		key.WithKeys(tea.KeyCtrlN.String()),
		key.WithHelp("ctrl+n", "new request"),
	),
	Duplicate: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "duplicate request"),
	),
	Delete: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "eXclude request"),
	),
	Help: key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "help"),
	),
}
