package scrollable

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyPgUp:
			m.YOffset -= m.Height
		case tea.KeyPgDown:
			m.YOffset += m.Height
		default:
			m.Content, cmd = m.Content.Update(msg)
		}

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelUp {
			m.YOffset -= 3
		} else if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelDown {
			m.YOffset += 3
		} else {
			m.Content, cmd = m.Content.Update(msg)
		}

	default:
		m.Content, cmd = m.Content.Update(msg)
	}

	m.clampOffset()
	return m, cmd
}

func (m Model) IsActive() bool {
	if a, ok := m.Content.(interface{ IsActive() bool }); ok {
		return a.IsActive()
	}
	return false
}

func (m *Model) clampOffset() {
	content := m.Content.View()
	lines := strings.Split(content, "\n")
	contentHeight := len(lines)
	maxOffset := contentHeight - m.Height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.YOffset > maxOffset {
		m.YOffset = maxOffset
	}
	if m.YOffset < 0 {
		m.YOffset = 0
	}
}

func (m Model) View() string {
	content := m.Content.View()
	lines := strings.Split(content, "\n")
	contentHeight := len(lines)

	yOffset := m.YOffset
	maxOffset := contentHeight - m.Height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if yOffset > maxOffset {
		yOffset = maxOffset
	}
	if yOffset < 0 {
		yOffset = 0
	}

	end := yOffset + m.Height
	if end > len(lines) {
		end = len(lines)
	}
	if yOffset >= len(lines) {
		return ""
	}

	return strings.Join(lines[yOffset:end], "\n")
}
