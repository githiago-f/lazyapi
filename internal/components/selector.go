package components

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/inmath"
)

// Selector is a select box: closed it shows the current label (up/down
// cycle, enter/space or a click opens it); open it renders a drop-down list
// (DropDownView, or Inline in the parent's flow) navigated with the arrows,
// home/end, pgup/pgdown, the mouse wheel and type-ahead; enter or a click
// picks, esc or a click outside cancels.
type Selector struct {
	Cursor int
	Labels []string
	Style  lipgloss.Style
	Width  int
	// ID names the selector's mouse zones; selectors without one only take
	// the keyboard.
	ID string

	Open           bool
	prevCursor     int
	DropDownHeight int
	scrollOffset   int
	typed          string
}

var selectorStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(config.Surface2)).
	Padding(0, 2).
	Align(lipgloss.Center)

var caretStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1))

func (s Selector) Value() string {
	if s.Cursor < 0 || s.Cursor >= len(s.Labels) {
		return ""
	}
	return s.Labels[s.Cursor]
}

// SetValue moves the cursor to label (no change when it is not listed).
func (s *Selector) SetValue(label string) bool {
	for i, l := range s.Labels {
		if l == label {
			s.Cursor = i
			return true
		}
	}
	return false
}

func (s Selector) Init() tea.Cmd {
	return nil
}

func (s Selector) zoneID() string {
	if s.ID == "" {
		return ""
	}
	return "select:" + s.ID
}

func (s Selector) itemZone(i int) string {
	if s.ID == "" {
		return ""
	}
	return "select:" + s.ID + ":" + strconv.Itoa(i)
}

// Zone is where the closed box was drawn.
func (s Selector) Zone() Zone { return Z.Get(s.zoneID()) }

func (s Selector) View() tea.View {
	label := s.Value()
	if len(s.Labels) > 1 {
		caret := "▾"
		if s.Open {
			caret = "▴"
		}
		label += " " + caretStyle.Render(caret)
	}
	style := s.Style.Inherit(selectorStyle)
	if s.Width > 0 {
		style = style.Width(s.Width)
	}
	return tea.NewView(Z.Mark(s.zoneID(), style.Render(label)))
}

// Update handles keys (when the parent gives it the focus) and mouse
// events. Clicks outside an open list close it.
func (s Selector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if s.Open {
			s.openKey(msg)
		} else {
			switch {
			case key.Matches(msg, config.DefaultKeyMap.Up):
				s.Cursor = inmath.Circle(s.Cursor-1, 0, len(s.Labels)-1)
			case key.Matches(msg, config.DefaultKeyMap.Down):
				s.Cursor = inmath.Circle(s.Cursor+1, 0, len(s.Labels)-1)
			case key.Matches(msg, config.DefaultKeyMap.Select) || key.Matches(msg, config.DefaultKeyMap.Ok):
				s.open()
			}
		}

	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			break
		}
		if s.Open {
			for i := range s.Labels {
				if Z.Get(s.itemZone(i)).InBounds(msg) {
					s.Cursor = i
					s.Open = false
					return s, nil
				}
			}
			if s.Zone().InBounds(msg) {
				s.Open = false // a second click on the box keeps the choice
			} else {
				s.Cursor, s.Open = s.prevCursor, false
			}
			return s, nil
		}
		if s.Zone().InBounds(msg) {
			s.open()
		}

	case tea.MouseWheelMsg:
		if !s.Open && !s.Zone().InBounds(msg) {
			break
		}
		switch msg.Button {
		case tea.MouseWheelUp:
			s.Cursor = max(0, s.Cursor-1)
		case tea.MouseWheelDown:
			s.Cursor = min(len(s.Labels)-1, s.Cursor+1)
		}
		s.clampScroll()
	}
	return s, nil
}

func (s *Selector) open() {
	if len(s.Labels) == 0 {
		return
	}
	s.Open = true
	s.prevCursor = s.Cursor
	s.typed = ""
	s.clampScroll()
}

func (s *Selector) openKey(msg tea.KeyPressMsg) {
	n := len(s.Labels)
	switch msg.String() {
	case "up", "ctrl+p", "shift+tab":
		s.Cursor = inmath.Circle(s.Cursor-1, 0, n-1)
	case "down", "ctrl+n":
		s.Cursor = inmath.Circle(s.Cursor+1, 0, n-1)
	case "home":
		s.Cursor = 0
	case "end":
		s.Cursor = n - 1
	case "pgup":
		s.Cursor = max(0, s.Cursor-s.dropDownHeight())
	case "pgdown":
		s.Cursor = min(n-1, s.Cursor+s.dropDownHeight())
	case "enter", "space", "tab":
		s.Open = false
	case "esc":
		s.Cursor, s.Open = s.prevCursor, false
	case "backspace":
		if s.typed != "" {
			s.typed = s.typed[:len(s.typed)-1]
		}
	default:
		if t := msg.Text; t != "" && msg.Mod == 0 {
			s.typeAhead(t)
		}
	}
	s.clampScroll()
}

// typeAhead jumps to the first label starting with what was typed (case
// insensitive); when nothing matches it starts over with the last key, so
// pressing the same letter again cycles through the labels with it.
func (s *Selector) typeAhead(t string) {
	find := func(prefix string, from int) int {
		prefix = strings.ToLower(prefix)
		for k := range len(s.Labels) {
			i := (from + k) % len(s.Labels)
			if strings.HasPrefix(strings.ToLower(s.Labels[i]), prefix) {
				return i
			}
		}
		return -1
	}
	if s.typed != "" {
		if i := find(s.typed+t, s.Cursor); i >= 0 {
			s.typed += t
			s.Cursor = i
			return
		}
	}
	s.typed = t
	if i := find(t, s.Cursor+1); i >= 0 {
		s.Cursor = i
	}
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
	BorderForeground(lipgloss.Color(config.Accent))

var (
	dropDownSel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(config.OnAccent)).
			Background(lipgloss.Color(config.Accent)).
			Padding(0, 1)
	dropDownUnsel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(config.Text)).
			Padding(0, 1)
	dropDownMore = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1)).Padding(0, 1)
)

// DropDownView renders the open list (empty when closed).
func (s Selector) DropDownView() string {
	if !s.Open || len(s.Labels) == 0 {
		return ""
	}

	h := min(s.dropDownHeight(), len(s.Labels))
	end := min(s.scrollOffset+h, len(s.Labels))

	// inner width: "▸ " + the label + the padding, at least the box's.
	inner := max(0, s.Width-2)
	for _, l := range s.Labels {
		inner = max(inner, lipgloss.Width(l)+4)
	}

	var items []string
	if s.scrollOffset > 0 {
		items = append(items, dropDownMore.Width(inner).Render("↑ "+strconv.Itoa(s.scrollOffset)+" more"))
	}
	for i := s.scrollOffset; i < end; i++ {
		st := dropDownUnsel
		label := "  " + s.Labels[i]
		if i == s.Cursor {
			st, label = dropDownSel, "▸ "+s.Labels[i]
		}
		items = append(items, Z.Mark(s.itemZone(i), st.Width(inner).Render(label)))
	}
	if rest := len(s.Labels) - end; rest > 0 {
		items = append(items, dropDownMore.Width(inner).Render("↓ "+strconv.Itoa(rest)+" more"))
	}
	return dropDownBox.Render(strings.Join(items, "\n"))
}
