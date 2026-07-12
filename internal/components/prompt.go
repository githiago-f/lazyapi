package components

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/config"
)

type OkMsg struct {
	Answer string
}

type OpenMsg int

type pos struct {
	x, y int
}

type PromptModel struct {
	question string
	answer   Field
	width    int
	open     bool
	pos      pos
}

func Prompt(question string) PromptModel {
	return PromptModel{
		question: question,
		answer:   InitField("> ", ""),
	}
}

func (m *PromptModel) SetPosition(x, y int) {
	m.pos = pos{x: x - m.width, y: y}
}

func (m *PromptModel) SetQuestion(q string) {
	m.question = q
}

func (m *PromptModel) SetValue(v string) {
	m.answer.SetValue(v)
}

func (m PromptModel) IsOpen() bool {
	return m.open
}

func (m *PromptModel) SetWidth(w int) {
	m.width = w
}

func (m *PromptModel) Value() string {
	return m.answer.Value()
}

func (m PromptModel) Init() tea.Cmd {
	return nil
}

func (m PromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case OpenMsg:
		m.open = true
	case tea.KeyPressMsg:
		if !m.open {
			return m, nil
		}
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Ok):
			value := m.answer.Value()
			m.open = false
			m.answer.SetValue("")
			return m, func() tea.Msg {
				return OkMsg{Answer: value}
			}
		case key.Matches(msg, config.DefaultKeyMap.Back):
			m.open = false
			m.answer.SetValue("")
		default:
			model, _ := m.answer.Update(msg)
			m.answer, _ = model.(Field)
		}
	}
	return m, nil
}

var box = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(config.DefaultConfig.PrimaryColor())

func (m PromptModel) View() tea.View {
	if !m.open {
		return tea.NewView("")
	}

	modal := box.Render(fmt.Sprintf("%s\n%s", m.question, m.answer.View().Content))

	xView := lipgloss.PlaceHorizontal(m.pos.x, lipgloss.Center, modal)
	return tea.NewView(lipgloss.PlaceVertical(m.pos.y, lipgloss.Center, xView))
}
