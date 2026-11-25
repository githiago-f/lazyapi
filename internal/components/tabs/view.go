package tabs

import (
	"strconv"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/inmath"
)

type KeyMap struct {
	SelectLeft  key.Binding
	SelectRight key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		SelectLeft: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "prev"),
		),
		SelectRight: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "next"),
		),
	}
}

type Styles struct {
	Tab               lipgloss.Style
	ActiveTab         lipgloss.Style
	OverflowIndicator lipgloss.Style
}

func DefaultStyles() Styles {
	return Styles{
		Tab: lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color(config.Overlay1)).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(config.Surface1)),
		ActiveTab: lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Foreground(lipgloss.Color(config.Accent)).
			BorderBottom(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(config.Accent)),
		OverflowIndicator: lipgloss.NewStyle().
			Foreground(lipgloss.Color(config.Overlay2)).
			Padding(0, 1),
	}
}

type SetActiveTabMsg struct {
	Active bool
}

type Model struct {
	Tabs   []Tab
	Styles Styles
	KeyMap KeyMap

	Style lipgloss.Style

	cursor   int
	width    int
	selected bool

	content string
	start   int
	end     int

	zone string // mouse zone prefix
}

func New(tabs ...Tab) Model {
	return Model{
		Tabs:   tabs,
		cursor: 0,
		Styles: DefaultStyles(),
		KeyMap: DefaultKeyMap(),
		zone:   components.Z.NewPrefix(),
	}
}

func (m Model) labelZone(i int) string { return m.zone + "tab:" + strconv.Itoa(i) }
func (m Model) contentZone() string    { return m.zone + "content" }

// Selected reports whether the current tab's content has the focus.
func (m Model) Selected() bool { return m.selected }

// Deselect blurs the current tab's content.
func (m Model) Deselect() (Model, tea.Cmd) {
	if !m.selected {
		return m, nil
	}
	m.selected = false
	var cmd tea.Cmd
	m.Tabs[m.cursor].Content, cmd = m.Tabs[m.cursor].Content.Update(SetActiveTabMsg{Active: false})
	return m, cmd
}

// click switches tabs on a label and focuses the content when clicked
// inside it (the click is then forwarded so the content can focus a field).
func (m Model) click(msg tea.MouseClickMsg) (Model, tea.Cmd, bool) {
	for i := range m.Tabs {
		if !components.Z.Get(m.labelZone(i)).InBounds(msg) {
			continue
		}
		var cmd tea.Cmd
		if i != m.cursor {
			m, cmd = m.Deselect()
			m.cursor = i
			m.updateSize()
		}
		return m, cmd, true
	}
	if !components.Z.Get(m.contentZone()).InBounds(msg) {
		return m, nil, false
	}
	var cmds []tea.Cmd
	if !m.selected {
		m.selected = true
		var cmd tea.Cmd
		m.Tabs[m.cursor].Content, cmd = m.Tabs[m.cursor].Content.Update(SetActiveTabMsg{Active: true})
		cmds = append(cmds, cmd)
	}
	var cmd tea.Cmd
	m.Tabs[m.cursor].Content, cmd = m.Tabs[m.cursor].Content.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...), true
}

func (m Model) Cursor() int { return m.cursor }

func (m *Model) SetCursor(n int) {
	if n < 0 {
		n = 0
	}
	if n >= len(m.Tabs) {
		n = len(m.Tabs) - 1
	}
	m.cursor = n
	m.updateSize()
}

func (m Model) Width() int { return m.width }
func (m *Model) SetWidth(w int) {
	m.width = w
	m.updateSize()
}

func (m *Model) updateSize() {
	if m.width <= 0 || len(m.Tabs) == 0 {
		m.content = ""
		return
	}

	leftover := m.width
	itemsContent := ""

	currDirection := -1
	left := m.cursor
	lastLeft := left
	right := min(m.cursor+1, len(m.Tabs))
	lastRight := right
	for len(m.Tabs) > 0 && leftover > 0 && (left >= 0 || right < len(m.Tabs)) {
		if currDirection < 0 && left >= 0 {
			lItem := m.renderLabel(left, leftover)
			leftover -= lipgloss.Width(lItem)
			itemsContent = lipgloss.JoinHorizontal(lipgloss.Top, lItem, itemsContent)
			lastLeft = left
			left--
		} else if currDirection > 0 && right < len(m.Tabs) {
			rItem := m.renderLabel(right, leftover)
			leftover -= lipgloss.Width(rItem)
			itemsContent = lipgloss.JoinHorizontal(lipgloss.Top, itemsContent, rItem)
			lastRight = right
			right++
		}

		if left < 0 {
			currDirection = 1
		} else if right >= len(m.Tabs) {
			currDirection = -1
		} else {
			currDirection *= -1
		}
	}
	lastRight = min(lastRight, len(m.Tabs)-1)

	m.start = lastLeft
	m.end = lastRight

	l := m.width
	loIndicator, roIndicator := "", ""

	if lastLeft != 0 {
		loIndicator = m.Styles.OverflowIndicator.Render("<")
		l -= lipgloss.Width(loIndicator)
	}
	if lastRight != len(m.Tabs)-1 {
		roIndicator = m.Styles.OverflowIndicator.Render(">")
		l -= lipgloss.Width(roIndicator)
	}

	if loIndicator != "" {
		truncate := lipgloss.Width(itemsContent) - l + 1
		itemsContent = ansi.TruncateLeft(itemsContent, truncate, "")
		if truncate > 0 {
			itemsContent = lipgloss.JoinHorizontal(lipgloss.Center,
				m.Styles.Tab.Inline(true).Render("…"), itemsContent)
		}
	} else {
		w := lipgloss.Width(itemsContent)
		if w > l {
			itemsContent = ansi.Truncate(itemsContent, l, "")
			itemsContent = lipgloss.JoinHorizontal(lipgloss.Center, itemsContent,
				m.Styles.Tab.Inline(true).Render("…"))
		}
	}

	m.content = lipgloss.JoinHorizontal(lipgloss.Center, loIndicator, itemsContent, roIndicator)
}

func (m Model) renderLabel(idx int, maxWidth int) string {
	style := m.Styles.Tab
	if idx == m.cursor {
		style = m.Styles.ActiveTab
	}

	item := components.Z.Mark(m.labelZone(idx), style.Render(m.Tabs[idx].Label))

	if idx > m.cursor {
		w := lipgloss.Width(item)
		if w > maxWidth {
			item = ansi.Truncate(item, maxWidth, m.Styles.Tab.Inline(true).Render("…"))
		}
	} else {
		w := lipgloss.Width(item)
		truncate := w - maxWidth - 1
		if truncate > 0 {
			item = ansi.TruncateLeft(item, truncate, "")
			if truncate > 0 {
				item = lipgloss.JoinHorizontal(lipgloss.Center,
					m.Styles.Tab.Inline(true).Render("…"), item)
			}
		}
	}

	if idx < len(m.Tabs)-1 {
		item = lipgloss.JoinHorizontal(
			lipgloss.Center,
			item,
			" ",
		)
	}

	return item
}

func (m Model) View() tea.View {
	tabLabels := m.content
	tabContent := m.Tabs[m.cursor].Content

	return tea.NewView(m.Style.
		Inherit(style).
		Width(m.width).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				tabLabels,
				components.Z.Mark(m.contentZone(), tabContent.View().Content),
			),
		))
}

var style = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(config.Surface2))

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		for i := range m.Tabs {
			m.Tabs[i].Content, cmd = m.Tabs[i].Content.Update(msg)
			if cmd != nil {
				return m, cmd
			}
		}

	case tea.MouseClickMsg:
		m, cmd, _ = m.click(msg)
		return m, cmd

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.SelectLeft) && !m.selected:
			m.cursor = inmath.Circle(m.cursor-1, 0, len(m.Tabs)-1)
			m.updateSize()
		case key.Matches(msg, m.KeyMap.SelectRight) && !m.selected:
			m.cursor = inmath.Circle(m.cursor+1, 0, len(m.Tabs)-1)
			m.updateSize()
		case key.Matches(msg, config.DefaultKeyMap.Select) && !m.selected:
			m.selected = true
			activeMsg := SetActiveTabMsg{Active: true}
			m.Tabs[m.cursor].Content, cmd = m.Tabs[m.cursor].Content.Update(activeMsg)
			return m, cmd
		case key.Matches(msg, config.DefaultKeyMap.Back) && completing(m.Tabs[m.cursor].Content):
			// esc closes the content's completion menu first.
		case key.Matches(msg, config.DefaultKeyMap.Back):
			m.selected = false
			activeMsg := SetActiveTabMsg{Active: false}
			m.Tabs[m.cursor].Content, cmd = m.Tabs[m.cursor].Content.Update(activeMsg)
			return m, cmd
		}
	}

	m.Tabs[m.cursor].Content, cmd = m.Tabs[m.cursor].Content.Update(msg)
	return m, cmd
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// completing reports whether a tab's content has a completion menu open.
func completing(content tea.Model) bool {
	c, ok := content.(interface{ Completing() bool })
	return ok && c.Completing()
}
