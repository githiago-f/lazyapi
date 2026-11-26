package requests

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/components"
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
	items          []RequestItem
	groups         []TagGroup
	filtered       []RequestItem
	filteredGroups []TagGroup
	cursor         int
	lastClick      int // row of the last click (double click opens)
	hover          int // row under the mouse pointer (-1: none)
	lastClickAt    time.Time
	scroll         int // first line (tag header or request) on screen

	filter    textinput.Model
	filtering bool

	width  int
	height int
}

func NewRequestList() RequestList {
	f := textinput.New()
	f.Placeholder = "press / to filter by path, method, tag or summary"
	f.Prompt = "🔍 "
	f.CharLimit = 100

	return RequestList{
		filter: f,
		hover:  -1,
	}
}

func (rl RequestList) HelpBindings() []key.Binding {
	return []key.Binding{
		config.DefaultKeyMap.Up,
		config.DefaultKeyMap.Down,
		config.DefaultKeyMap.ListPageDown,
		config.DefaultKeyMap.ListPageUp,
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

// CursorPosY is the row of the selected request below the filter, on
// screen (the list may be scrolled).
func (rl RequestList) CursorPosY() int {
	return 1 + rl.cursorLine() - rl.scroll
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
	rl.filter.SetWidth(max(1, w-4))
	rl.clampScroll()
}

// Filtering reports whether the filter input is capturing keys.
func (rl RequestList) Filtering() bool { return rl.filtering }

// Items returns every item in the list (unfiltered).
func (rl RequestList) Items() []RequestItem { return rl.items }

func (rl RequestList) SetItems(items []RequestItem) RequestList {
	items = dedupItems(items)
	rl.items = items
	rl.groups = GroupByTags(items)
	rl.applyFilter()
	if rl.cursor >= len(rl.filtered) {
		rl.cursor = max(0, len(rl.filtered)-1)
	}
	rl.clampScroll()
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
	rl.hover = -1 // rows move: the pointer is over another one now
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
	cursorMark = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Accent)).SetString("▸")
	paneBg     = lipgloss.NewStyle().Background(lipgloss.Color(config.Base))
)

func (rl RequestList) View() tea.View {
	lines := rl.visibleItems()
	rows, overflow := rl.rows(len(lines))
	from := min(rl.scroll, len(lines))
	to := min(from+rows, len(lines))
	rendered := make([]string, 0, rows+2)

	rendered = append(rendered, rl.filterView())

	for _, line := range lines[from:to] {
		selected := rl.cursor >= 0 && rl.cursor < len(rl.filtered) && rl.filtered[rl.cursor].URI == line.item.URI && rl.filtered[rl.cursor].Method == line.item.Method
		if line.isHeader {
			rendered = append(rendered, "  "+lipgloss.NewStyle().Foreground(lipgloss.Color(config.Lavender)).Bold(true).Render(fmt.Sprintf("── %s (%d) ──", line.header, line.count)))
			continue
		}
		prefix := " "
		if selected {
			prefix = cursorMark.String()
		}
		i := rl.indexOf(line.item)
		row := prefix + RenderCompact(line.item, rl.width, selected, i >= 0 && i == rl.hover)
		if i >= 0 {
			row = components.Z.Mark(rowZone(i), row)
		}
		rendered = append(rendered, row)
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
		if overflow {
			used++ // the position line
		}
		if used < rl.height {
			fill := strings.Repeat("\n", rl.height-used-1)
			content = lipgloss.JoinVertical(lipgloss.Left,
				content,
				paneBg.Render(fill),
			)
		}
		if overflow {
			content = lipgloss.JoinVertical(lipgloss.Left, content, rl.positionLine(lines[from:to], lines))
		}
	}
	return tea.NewView(components.Z.Mark(paneZone, content))
}

// filterHeight is the filter input plus its bottom border.
const filterHeight = 2

// rows is how many list lines fit under the filter, and whether the list
// overflows (then the last row is the position line).
func (rl RequestList) rows(lines int) (int, bool) {
	if rl.height <= 0 {
		return lines, false // not sized yet: everything
	}
	rows := max(1, rl.height-filterHeight)
	if lines <= rows {
		return rows, false
	}
	return max(1, rows-1), true
}

var positionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1))

// positionLine says where the selection is and how much is out of view.
func (rl RequestList) positionLine(shown, all []itemLine) string {
	onScreen := 0
	for _, l := range shown {
		if !l.isHeader {
			onScreen++
		}
	}
	above := 0
	for _, l := range all[:min(rl.scroll, len(all))] {
		if !l.isHeader {
			above++
		}
	}
	below := len(rl.filtered) - above - onScreen
	parts := []string{config.T("list.position", "pos", strconv.Itoa(rl.cursor+1), "total", strconv.Itoa(len(rl.filtered)))}
	if above > 0 {
		parts = append(parts, config.T("list.above", "n", strconv.Itoa(above)))
	}
	if below > 0 {
		parts = append(parts, config.T("list.below", "n", strconv.Itoa(below)))
	}
	parts = append(parts, config.T("list.paging",
		"up", config.DefaultKeyMap.ListPageUp.Help().Key, "down", config.DefaultKeyMap.ListPageDown.Help().Key))
	line := "  " + strings.Join(parts, " · ")
	if rl.width > 0 {
		line = ansi.Truncate(line, rl.width, "…")
	}
	return positionStyle.Render(line)
}

const paneZone = "list:pane"

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

func rowZone(i int) string { return "list:" + strconv.Itoa(i) }

// indexOf is the position of an item in the filtered list (-1: none).
func (rl RequestList) indexOf(item RequestItem) int {
	for i, it := range rl.filtered {
		if it.URI == item.URI && it.Method == item.Method && it.FileName == item.FileName && it.DraftPath == item.DraftPath {
			return i
		}
	}
	return -1
}

// doubleClick is the longest gap between the clicks of a double click.
const doubleClick = 400 * time.Millisecond

// open opens the selected request in the editor.
func (rl RequestList) open() tea.Cmd {
	item, ok := rl.SelectedItem()
	if !ok {
		return nil
	}
	if item.DraftPath != "" {
		return OpenDraftView(item.FileName, item.DraftPath)
	}
	return OpenRequestView(item.FileName, item.OpenAPIRef)
}

func (rl RequestList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := rl.update(msg)
	out := m.(RequestList)
	out.clampScroll() // the cursor moved, the filter changed: keep it in view
	return out, cmd
}

func (rl RequestList) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return rl, nil
		}
		for i := range rl.filtered {
			if components.Z.Get(rowZone(i)).InBounds(msg) {
				// a click selects, a second click on it soon after opens it
				double := i == rl.lastClick && time.Since(rl.lastClickAt) < doubleClick
				rl.cursor, rl.lastClick, rl.lastClickAt = i, i, time.Now()
				if double {
					rl.lastClick = -1
					return rl, rl.open()
				}
				return rl, nil
			}
		}
		return rl, nil
	case tea.MouseMotionMsg:
		rl.hover = -1
		for i := range rl.filtered {
			if components.Z.Get(rowZone(i)).InBounds(msg) {
				rl.hover = i
				break
			}
		}
		return rl, nil
	case tea.MouseWheelMsg:
		if !components.Z.Get(paneZone).InBounds(msg) {
			return rl, nil
		}
		switch msg.Button {
		case tea.MouseWheelUp:
			rl.cursor = max(0, rl.cursor-1)
		case tea.MouseWheelDown:
			rl.cursor = min(max(0, len(rl.filtered)-1), rl.cursor+1)
		}
		rl.clampScroll()
		return rl, nil
	}

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

			case key.Matches(msg, config.DefaultKeyMap.ListPageUp):
				rl.cursor = max(0, rl.cursor-rl.page())

			case key.Matches(msg, config.DefaultKeyMap.ListPageDown):
				rl.cursor = min(max(0, len(rl.filtered)-1), rl.cursor+rl.page())

			case key.Matches(msg, config.DefaultKeyMap.ListTop):
				rl.cursor = 0

			case key.Matches(msg, config.DefaultKeyMap.ListBottom):
				rl.cursor = max(0, len(rl.filtered)-1)

			case key.Matches(msg, config.DefaultKeyMap.Select):
				return rl, rl.open()

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

// page is how many requests a page key moves: the rows on screen.
func (rl RequestList) page() int {
	rows, _ := rl.rows(len(rl.visibleItems()))
	return max(1, rows-1)
}

// cursorLine is the selected request's line among the tag headers and
// requests (0 when there is none).
func (rl RequestList) cursorLine() int {
	for n, l := range rl.visibleItems() {
		if !l.isHeader && rl.isCursorItem(l.item) {
			return n
		}
	}
	return 0
}

// clampScroll scrolls just enough to show the selected request (and its
// tag header when it is the first of its group), and never past the end.
func (rl *RequestList) clampScroll() {
	lines := rl.visibleItems()
	rows, _ := rl.rows(len(lines))
	cur := rl.cursorLine()
	top := cur
	if cur > 0 && lines[cur-1].isHeader {
		top = cur - 1
	}
	if top < rl.scroll {
		rl.scroll = top
	}
	if cur >= rl.scroll+rows {
		rl.scroll = cur - rows + 1
	}
	rl.scroll = max(0, min(rl.scroll, len(lines)-rows))
}
