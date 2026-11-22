package scrollable

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

type Model struct {
	Content tea.Model
	Width   int
	Height  int
	YOffset int
}

func New(content tea.Model) Model {
	return Model{Content: content}
}

func (m Model) HelpBindings() []key.Binding {
	if h, ok := m.Content.(interface{ HelpBindings() []key.Binding }); ok {
		return h.HelpBindings()
	}
	return nil
}

func (m Model) Init() tea.Cmd {
	return m.Content.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Content, cmd = m.Content.Update(msg)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "pgup":
			m.YOffset -= m.Height
		case "pgdown":
			m.YOffset += m.Height
		default:
			m.Content, cmd = m.Content.Update(msg)
		}

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.YOffset -= 3
		case tea.MouseWheelDown:
			m.YOffset += 3
		}

	case tea.MouseMsg:
		m.Content, cmd = m.Content.Update(msg)

	default:
		m.Content, cmd = m.Content.Update(msg)
	}

	m.clampOffset()
	return m, cmd
}

// Completing forwards the content's completion state (see tabs).
func (m Model) Completing() bool {
	c, ok := m.Content.(interface{ Completing() bool })
	return ok && c.Completing()
}

func (m Model) IsActive() bool {
	if a, ok := m.Content.(interface{ IsActive() bool }); ok {
		return a.IsActive()
	}
	return false
}

// maxOffset is the largest useful YOffset for the current content.
func (m Model) maxOffset() int {
	return max(0, strings.Count(m.Content.View().Content, "\n")+1-m.Height)
}

func (m *Model) clampOffset() {
	m.YOffset = max(0, min(m.YOffset, m.maxOffset()))
}

func (m Model) View() tea.View {
	lines := strings.Split(m.Content.View().Content, "\n")
	start := max(0, min(m.YOffset, m.maxOffset()))
	end := min(len(lines), start+m.Height)
	return tea.NewView(strings.Join(lines[start:end], "\n"))
}
