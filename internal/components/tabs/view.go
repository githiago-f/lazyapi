package tabs

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(config.Overlay1)),
		ActiveTab: lipgloss.NewStyle().
			Padding(0, 2).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(config.Peach)),
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
}

func New(tabs ...Tab) Model {
	return Model{
		Tabs:   tabs,
		cursor: 0,
		Styles: DefaultStyles(),
		KeyMap: DefaultKeyMap(),
	}
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

func (m Model) Width() int    { return m.width }
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

	item := style.Render(m.Tabs[idx].Label)

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
				tabContent.View().Content,
			),
		))
}

var style = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder())

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
