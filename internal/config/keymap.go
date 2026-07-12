package config

import "charm.land/bubbles/v2/key"

type KeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding

	Select    key.Binding
	Filter    key.Binding
	Delete    key.Binding
	CreateNew key.Binding
	Duplicate key.Binding
	Ok        key.Binding

	Back key.Binding
	Save key.Binding

	SaveExample key.Binding

	Next key.Binding
	Prev key.Binding

	Quit key.Binding
	Kill key.Binding

	HelpToggle key.Binding

	// Response pane
	CopyResponse key.Binding

	// Tab-specific actions (editor)
	AddQueryParam  key.Binding
	AddPathParam   key.Binding
	AddHeader      key.Binding
	AddAuth        key.Binding
	DelAuth        key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return nil
}

func (k KeyMap) FullHelp() [][]key.Binding {
	if DefaultConfig.Active == RequestEditor {
		return [][]key.Binding{
			{k.Next, k.Prev},
			{k.Back, k.Save, k.SaveExample, k.CopyResponse, k.Quit, k.Kill},
			{k.AddQueryParam, k.AddPathParam, k.AddHeader, k.AddAuth, k.DelAuth},
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
		key.WithKeys("left"),
		key.WithHelp("←", "move left"),
	),
	Right: key.NewBinding(
		key.WithKeys("l", "right"),
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
		key.WithKeys("enter", "space"),
		key.WithHelp("enter/space", "select"),
	),
	Ok: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "Accept"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
	Kill: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "exit"),
	),
	Next: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next"),
	),
	Prev: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Save: key.NewBinding(
		key.WithKeys("ctrl+o"),
		key.WithHelp("ctrl+o", "save"),
	),
	SaveExample: key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("ctrl+e", "save example"),
	),
	CopyResponse: key.NewBinding(
		key.WithKeys("ctrl+y"),
		key.WithHelp("ctrl+y", "copy response"),
	),
	CreateNew: key.NewBinding(
		key.WithKeys("ctrl+n"),
		key.WithHelp("ctrl+n", "new request"),
	),
	Duplicate: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "duplicate"),
	),
	Delete: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "delete"),
	),
	HelpToggle: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	AddQueryParam: key.NewBinding(
		key.WithKeys("ctrl+q"),
		key.WithHelp("ctrl+q", "add query"),
	),
	AddPathParam: key.NewBinding(
		key.WithKeys("ctrl+p"),
		key.WithHelp("ctrl+p", "add path param"),
	),
	AddHeader: key.NewBinding(
		key.WithKeys("ctrl+g"),
		key.WithHelp("ctrl+g", "add header"),
	),
	AddAuth: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "add auth"),
	),
	DelAuth: key.NewBinding(
		key.WithKeys("bksp", "del"),
		key.WithHelp("⌫/del", "delete"),
	),
}
