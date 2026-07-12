package components

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/inmath"
)

type Selector struct {
	Cursor int
	Labels []string
	Style  lipgloss.Style
	Width  int

	Open           bool
	prevCursor     int
	DropDownHeight int
	scrollOffset   int
}

var selectorStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	Padding(0, 2).
	Align(lipgloss.Center)

func (s Selector) Value() string {
	if s.Cursor < 0 || s.Cursor >= len(s.Labels) {
		return ""
	}
	return s.Labels[s.Cursor]
}

func (s Selector) Init() tea.Cmd {
	return nil
}

func (s Selector) View() tea.View {
	label := s.Value()
	style := s.Style.Inherit(selectorStyle)
	if s.Width > 0 {
		style = style.Width(s.Width)
	}
	return tea.NewView(style.Render(label))
}

func (s Selector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if s.Open {
			switch {
			case key.Matches(msg, config.DefaultKeyMap.Up):
				s.Cursor = inmath.Circle(s.Cursor-1, 0, len(s.Labels)-1)
				s.clampScroll()
			case key.Matches(msg, config.DefaultKeyMap.Down):
				s.Cursor = inmath.Circle(s.Cursor+1, 0, len(s.Labels)-1)
				s.clampScroll()
			case key.Matches(msg, config.DefaultKeyMap.Select):
				s.Open = false
			case key.Matches(msg, config.DefaultKeyMap.Back):
				s.Cursor = s.prevCursor
				s.Open = false
			}
		} else {
			switch {
			case key.Matches(msg, config.DefaultKeyMap.Up):
				s.Cursor = inmath.Circle(s.Cursor-1, 0, len(s.Labels)-1)
			case key.Matches(msg, config.DefaultKeyMap.Down):
				s.Cursor = inmath.Circle(s.Cursor+1, 0, len(s.Labels)-1)
			case key.Matches(msg, config.DefaultKeyMap.Select) || key.Matches(msg, config.DefaultKeyMap.Ok):
				if len(s.Labels) > 0 {
					s.Open = true
					s.prevCursor = s.Cursor
					s.clampScroll()
				}
			}
		}
	}
	return s, nil
}

func (s *Selector) clampScroll() {
	h := s.dropDownHeight()
	if h <= 0 {
		return
	}
	if s.Cursor < s.scrollOffset {
		s.scrollOffset = s.Cursor
	}
	if s.Cursor >= s.scrollOffset+h {
		s.scrollOffset = s.Cursor - h + 1
	}
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
}

func (s Selector) dropDownHeight() int {
	if s.DropDownHeight > 0 {
		return s.DropDownHeight
	}
	return 7
}

var dropDownBox = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color(config.Peach))

var (
	dropDownSel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(config.Peach)).
			Padding(0, 1)
	dropDownUnsel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(config.Text)).
			Padding(0, 1)
)

func (s Selector) DropDownView() string {
	if !s.Open || len(s.Labels) == 0 {
		return ""
	}

	h := s.dropDownHeight()
	n := len(s.Labels)
	if h > n {
		h = n
	}
	end := s.scrollOffset + h
	if end > n {
		end = n
	}

	var items []string
	for i := s.scrollOffset; i < end; i++ {
		label := "  " + s.Labels[i]
		if i == s.Cursor {
			label = "▸ " + s.Labels[i]
			items = append(items, dropDownSel.Render(label))
		} else {
			items = append(items, dropDownUnsel.Render(label))
		}
	}

	w := s.Width
	for _, l := range s.Labels {
		lw := lipgloss.Width(l)
		if lw+4 > w {
			w = lw + 4
		}
	}

	return Z.Mark(s.zoneID(), dropDownBox.Width(w).Render(strings.Join(items, "\n")))
}

func (s Selector) zoneID() string {
	return "select-dd-" + s.Value()
}
