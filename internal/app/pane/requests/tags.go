package requests

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
)

type SaveTagsMsg struct {
	Tags      []string
	Ref       *model.OpenAPIRef
	FileName  string
	DraftPath string
}

type CancelTagsMsg struct{}

type TagsOverlay struct {
	item     *RequestItem
	tags     []string
	input    textinput.Model
	width    int
	focusDel bool
	delIdx   int
}

func NewTagsOverlay(item RequestItem) TagsOverlay {
	t := textinput.New()
	t.Placeholder = "add tag..."
	t.Prompt = ""
	t.CharLimit = 50
	t.Focus()

	tags := make([]string, len(item.Tags))
	copy(tags, item.Tags)

	return TagsOverlay{
		item: &item,
		tags: tags,
		input: t,
	}
}

func (to TagsOverlay) Init() tea.Cmd {
	return nil
}

var (
	tagChip    = lipgloss.NewStyle().Background(lipgloss.Color(config.Surface1)).Foreground(lipgloss.Color(config.Text)).Padding(0, 1)
	tagDel     = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Red)).SetString(" ×")
	tagSelChip = lipgloss.NewStyle().Background(lipgloss.Color(config.Surface2)).Foreground(lipgloss.Color(config.Peach)).Padding(0, 1)
	tagSelDel  = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Red)).Bold(true).SetString(" ×")
	tagBox     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(config.Peach)).Padding(1, 2)
	tagHint    = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay0))
)

func (to TagsOverlay) View() tea.View {
	var b strings.Builder

	b.WriteString("Edit Tags\n\n")

	if len(to.tags) > 0 {
		var chips []string
		for i, t := range to.tags {
			chip := tagChip.Render(t) + tagDel.String()
			if to.focusDel && i == to.delIdx {
				chip = tagSelChip.Render(t) + tagSelDel.String()
			}
			chips = append(chips, chip)
		}
		b.WriteString(strings.Join(chips, "  "))
		b.WriteString("\n\n")
	}

	b.WriteString("Add tag: " + to.input.View())
	b.WriteString("\n\n")
	b.WriteString(tagHint.Render("Enter: add tag  Tab: delete  Esc: save & close"))

	rendered := tagBox.Render(b.String())

	if to.width > 0 {
		rendered = lipgloss.NewStyle().Width(to.width).Render(rendered)
	}

	return tea.NewView(rendered)
}

func (to *TagsOverlay) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Back):
			if to.focusDel {
				to.focusDel = false
				return to, nil
			}
			return to, func() tea.Msg { return SaveTagsMsg{to.tags, to.item.OpenAPIRef, to.item.FileName, to.item.DraftPath} }

		case key.Matches(msg, config.DefaultKeyMap.Next):
			if to.focusDel {
				to.delIdx = (to.delIdx + 1) % max(1, len(to.tags))
			} else if len(to.tags) > 0 {
				to.focusDel = true
				to.delIdx = 0
			}
			return to, nil

		case key.Matches(msg, config.DefaultKeyMap.Select), key.Matches(msg, config.DefaultKeyMap.Ok):
			if !to.focusDel {
				v := strings.TrimSpace(to.input.Value())
				if v != "" {
					for _, existing := range to.tags {
						if strings.EqualFold(existing, v) {
							to.input.SetValue("")
							return to, nil
						}
					}
					to.tags = append(to.tags, v)
					to.input.SetValue("")
					if !to.input.Focused() {
						to.input.Focus()
					}
					return to, nil
				}
			}

			if to.focusDel && len(to.tags) > 0 && to.delIdx < len(to.tags) {
				to.tags = append(to.tags[:to.delIdx], to.tags[to.delIdx+1:]...)
				to.focusDel = len(to.tags) > 0
				if to.delIdx >= len(to.tags) && to.delIdx > 0 {
					to.delIdx = len(to.tags) - 1
				}
				return to, nil
			}

			return to, nil

		case to.focusDel && key.Matches(msg, config.DefaultKeyMap.Delete):
			if len(to.tags) > 0 && to.delIdx < len(to.tags) {
				to.tags = append(to.tags[:to.delIdx], to.tags[to.delIdx+1:]...)
				to.focusDel = len(to.tags) > 0
				if to.delIdx >= len(to.tags) && to.delIdx > 0 {
					to.delIdx = len(to.tags) - 1
				}
			}
			return to, nil

		case to.focusDel && key.Matches(msg, config.DefaultKeyMap.Right):
			to.delIdx = (to.delIdx + 1) % max(1, len(to.tags))
			return to, nil

		case to.focusDel && key.Matches(msg, config.DefaultKeyMap.Left):
			to.delIdx = (to.delIdx - 1 + len(to.tags)) % max(1, len(to.tags))
			return to, nil

		default:
			to.input, _ = to.input.Update(msg)
			if !to.input.Focused() {
				to.input.Focus()
			}
		}
	}

	return to, nil
}
