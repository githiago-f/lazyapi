package requests

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/inmath"
	"github.com/githiago-f/lazyapi/internal/model"
)

type OpenRequestViewMsg struct {
	FileName   string
	OpenAPIRef *model.OpenAPIRef
	DraftPath  string
}

func OpenRequestView(fileName string, ref *model.OpenAPIRef) tea.Cmd {
	return func() tea.Msg {
		return OpenRequestViewMsg{
			FileName:   fileName,
			OpenAPIRef: ref,
		}
	}
}

func OpenDraftView(fileName, draftPath string) tea.Cmd {
	return func() tea.Msg {
		return OpenRequestViewMsg{
			FileName:  fileName,
			DraftPath: draftPath,
		}
	}
}

type CreateNewRequestMsg struct{}

type DuplicateRequestMsg struct {
	Item RequestItem
}

type DeleteRequestMsg struct {
	Item RequestItem
}

type SendRequestMsg struct{}

type EditTagsMsg struct{}

type CycleServerMsg struct{}

type CursorPosMsg struct {
	Y int
}

type RequestList struct {
	items    []RequestItem
	groups   []TagGroup
	filtered []RequestItem
	filteredGroups []TagGroup
	cursor   int
	scroll   int

	filter    textinput.Model
	filtering bool

	width  int
	height int
}

func NewRequestList() RequestList {
	f := textinput.New()
	f.Placeholder = "type / to filter..."
	f.Prompt = "🔍 "
	f.CharLimit = 100

	return RequestList{
		filter: f,
	}
}

func (rl RequestList) HelpBindings() []key.Binding {
	return []key.Binding{
		config.DefaultKeyMap.Up,
		config.DefaultKeyMap.Down,
		config.DefaultKeyMap.Select,
		config.DefaultKeyMap.Filter,
		config.DefaultKeyMap.CreateNew,
		config.DefaultKeyMap.Duplicate,
		config.DefaultKeyMap.Delete,
		config.DefaultKeyMap.SendRequest,
		config.DefaultKeyMap.EditTags,
		config.DefaultKeyMap.Quit,
	}
}

func (rl RequestList) CursorPosY() int {
	sum := 1
	groups := rl.filteredGroups
	if groups == nil {
		groups = rl.groups
	}
	for _, g := range groups {
		if len(g.Items) == 0 {
			continue
		}
		sum++
		for _, item := range g.Items {
			if rl.isCursorItem(item) {
				return sum
			}
			sum++
		}
	}
	return sum
}

func (rl RequestList) isCursorItem(item RequestItem) bool {
	if rl.cursor < 0 || rl.cursor >= len(rl.filtered) {
		return false
	}
	return rl.filtered[rl.cursor].URI == item.URI && rl.filtered[rl.cursor].Method == item.Method
}

func (rl RequestList) SelectedItem() (RequestItem, bool) {
	if rl.cursor < 0 || rl.cursor >= len(rl.filtered) {
		return RequestItem{}, false
	}
	return rl.filtered[rl.cursor], true
}

func (rl *RequestList) SetSize(w, h int) {
	rl.width = w
	rl.height = h
}

func (rl RequestList) SetItems(items []RequestItem) RequestList {
	items = dedupItems(items)
	rl.items = items
	rl.groups = GroupByTags(items)
	rl.applyFilter()
	if rl.cursor >= len(rl.filtered) {
		rl.cursor = max(0, len(rl.filtered)-1)
	}
	return rl
}

func dedupItems(items []RequestItem) []RequestItem {
	seen := make(map[string]bool)
	result := make([]RequestItem, 0, len(items))
	for _, item := range items {
		key := item.FileName + "|" + strings.ToLower(item.URI) + "|" + strings.ToLower(item.Method.Label())
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

func (rl *RequestList) applyFilter() {
	q := strings.TrimSpace(rl.filter.Value())
	if q == "" {
		rl.filteredGroups = rl.groups
	} else {
		qLower := strings.ToLower(q)
		var filtered []RequestItem
		for _, item := range rl.items {
			if strings.Contains(strings.ToLower(item.FilterValue()), qLower) {
				filtered = append(filtered, item)
			}
		}
		rl.filteredGroups = GroupByTags(filtered)
	}
	rl.filtered = flattenGroups(rl.filteredGroups)
}

func flattenGroups(groups []TagGroup) []RequestItem {
	var items []RequestItem
	for _, g := range groups {
		items = append(items, g.Items...)
	}
	return items
}

func (rl RequestList) Init() tea.Cmd {
	return nil
}

var (
	cursorMark = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Peach)).SetString("▸")
	paneBg = lipgloss.NewStyle().Background(lipgloss.Color(config.Base))
)

func (rl RequestList) View() tea.View {
	visible := rl.visibleItems()
	rendered := make([]string, 0, len(visible)+2)

	rendered = append(rendered, rl.filterView())

	for _, line := range visible {
		selected := rl.cursor >= 0 && rl.cursor < len(rl.filtered) && rl.filtered[rl.cursor].URI == line.item.URI && rl.filtered[rl.cursor].Method == line.item.Method
		if line.isHeader {
			rendered = append(rendered, "  "+lipgloss.NewStyle().Foreground(lipgloss.Color(config.Peach)).Bold(true).Render(fmt.Sprintf("── %s (%d) ──", line.header, line.count)))
			continue
		}
		prefix := " "
		if selected {
			prefix = cursorMark.String()
		}
		rendered = append(rendered, prefix+line.item.RenderCompact(rl.width, selected))
	}

	if len(rl.filtered) == 0 {
		if rl.filtering || rl.filter.Value() != "" {
			rendered = append(rendered, "  No matching requests")
		} else if len(rl.items) == 0 {
			rendered = append(rendered, "  No requests found")
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rendered...)
	if rl.height > 0 {
		used := lipgloss.Height(content)
		if used < rl.height {
			fill := strings.Repeat("\n", rl.height-used)
			content = lipgloss.JoinVertical(lipgloss.Left,
				content,
				paneBg.Render(fill),
			)
		}
	}
	return tea.NewView(content)
}

func (rl RequestList) filterView() string {
	fv := rl.filter.View()
	if rl.width > 0 {
		fv = lipgloss.NewStyle().Width(rl.width).Render(fv)
	}
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color(config.Surface2)).
		Render(fv)
}

func (rl RequestList) visibleItems() []itemLine {
	groups := rl.filteredGroups
	if groups == nil {
		groups = rl.groups
	}

	var lines []itemLine
	for _, g := range groups {
		if len(g.Items) == 0 {
			continue
		}
		lines = append(lines, itemLine{isHeader: true, header: g.Tag, count: len(g.Items)})
		for _, item := range g.Items {
			lines = append(lines, itemLine{item: item})
		}
	}
	return lines
}

type itemLine struct {
	isHeader bool
	header   string
	count    int
	item     RequestItem
}

func (rl RequestList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if rl.filtering {
		rl.filter, cmd = rl.filter.Update(msg)
		rl.applyFilter()
		if rl.cursor >= len(rl.filtered) {
			rl.cursor = max(0, len(rl.filtered)-1)
		}
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, config.DefaultKeyMap.Filter) && !rl.filtering {
			rl.filter.Focus()
			rl.filtering = true
			return rl, nil
		}

		if rl.filtering && key.Matches(msg, config.DefaultKeyMap.Back) {
			if rl.filter.Value() != "" {
				rl.filter.SetValue("")
				rl.applyFilter()
			} else {
				rl.filter.Blur()
				rl.filtering = false
			}
			return rl, nil
		}

		if rl.filtering && key.Matches(msg, config.DefaultKeyMap.Select) {
			rl.filter.Blur()
			rl.filtering = false
			return rl, nil
		}

		if rl.filtering {
			if key.Matches(msg, config.DefaultKeyMap.Up) {
				rl.cursor = inmath.Circle(rl.cursor-1, 0, max(0, len(rl.filtered)-1))
			}
			if key.Matches(msg, config.DefaultKeyMap.Down) {
				rl.cursor = inmath.Circle(rl.cursor+1, 0, max(0, len(rl.filtered)-1))
			}
			return rl, nil
		}

		if !rl.filtering {
			switch {
			case key.Matches(msg, config.DefaultKeyMap.Up):
				rl.cursor = inmath.Circle(rl.cursor-1, 0, max(0, len(rl.filtered)-1))
				rl.clampScroll()

			case key.Matches(msg, config.DefaultKeyMap.Down):
				rl.cursor = inmath.Circle(rl.cursor+1, 0, max(0, len(rl.filtered)-1))
				rl.clampScroll()

			case key.Matches(msg, config.DefaultKeyMap.Select):
				item, ok := rl.SelectedItem()
				if !ok {
					return rl, nil
				}
				if item.DraftPath != "" {
					return rl, OpenDraftView(item.FileName, item.DraftPath)
				}
				return rl, OpenRequestView(item.FileName, item.OpenAPIRef)

			case key.Matches(msg, config.DefaultKeyMap.CreateNew):
				return rl, func() tea.Msg { return CreateNewRequestMsg{} }

			case key.Matches(msg, config.DefaultKeyMap.Duplicate):
				item, ok := rl.SelectedItem()
				if !ok {
					return rl, nil
				}
				return rl, func() tea.Msg { return DuplicateRequestMsg{Item: item} }

			case key.Matches(msg, config.DefaultKeyMap.Delete):
				item, ok := rl.SelectedItem()
				if !ok {
					return rl, nil
				}
				return rl, func() tea.Msg { return DeleteRequestMsg{Item: item} }

			case key.Matches(msg, config.DefaultKeyMap.SendRequest):
				return rl, func() tea.Msg { return SendRequestMsg{} }

			case key.Matches(msg, config.DefaultKeyMap.EditTags):
				return rl, func() tea.Msg { return EditTagsMsg{} }

			case key.Matches(msg, config.DefaultKeyMap.CycleServer):
				return rl, func() tea.Msg { return CycleServerMsg{} }
			}
		}
	}

	return rl, cmd
}

func (rl *RequestList) clampScroll() {
	if rl.cursor < rl.scroll {
		rl.scroll = rl.cursor
	}
	maxScroll := max(0, len(rl.filtered)-rl.height+3)
	if rl.scroll > maxScroll {
		rl.scroll = maxScroll
	}
}


