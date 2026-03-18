package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/config"
)

type Option struct {
	Keys  []string
	Label string
}

type OptionsPane struct {
	tea.Model
	Options []Option
	Style   lipgloss.Style
}

func (o OptionsPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return o, nil
}

var (
	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(config.Overlay2)).
			Bold(true)
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(config.Surface2))
)

func (o OptionsPane) View() string {
	opts := strings.Builder{}

	for i, opt := range o.Options {
		key := keyStyle.Render(strings.Join(opt.Keys, "/"))
		label := labelStyle.Render(opt.Label)

		separator := ""
		if i+1 != len(o.Options) {
			separator = labelStyle.Render(" • ")
		}
		fmt.Fprintf(&opts, "%s %s%s", key, label, separator)
	}

	return o.Style.Render(opts.String())
}

func NewOption(label string, keys ...string) Option {
	return Option{
		Label: label,
		Keys:  keys,
	}
}

func NewOptionsPane(opts ...Option) OptionsPane {
	return OptionsPane{
		Options: opts,
		Style: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false),
	}
}
