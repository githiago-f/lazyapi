package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/components/complete"
	"github.com/githiago-f/lazyapi/internal/config"
)

// Completion is an open completion menu: what to replace and the choice.
type Completion struct {
	complete.Result
	Sel int
}

// Move changes the selection (wrapping).
func (c *Completion) Move(d int) {
	if n := len(c.Items); n > 0 {
		c.Sel = ((c.Sel+d)%n + n) % n
	}
}

// Apply inserts the selected item into text, returning the new text and
// cursor (rune offsets).
func (c Completion) Apply(text []rune) ([]rune, int) {
	it := c.Items[c.Sel]
	ins := []rune(it.Insert)
	out := make([]rune, 0, len(text)+len(ins))
	out = append(out, text[:c.Start]...)
	out = append(out, ins...)
	out = append(out, text[min(c.End, len(text)):]...)
	return out, c.Start + len(ins) - it.CursorBack
}

var (
	menuBox    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(config.Mauve)).Background(lipgloss.Color(config.Mantle))
	menuItem   = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Text)).Background(lipgloss.Color(config.Mantle))
	menuSel    = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Crust)).Background(lipgloss.Color(config.Mauve)).Bold(true)
	menuDetail = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay1)).Background(lipgloss.Color(config.Mantle))
	menuSelDet = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Surface0)).Background(lipgloss.Color(config.Mauve))
	menuHint   = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Overlay0)).Background(lipgloss.Color(config.Mantle))
)

// MenuRows is how many items a menu shows at once.
const MenuRows = 6

// View renders the menu (at most width columns, borders included).
func (c Completion) View(width int) string {
	if len(c.Items) == 0 {
		return ""
	}
	labelW, detailW := 0, 0
	for _, it := range c.Items {
		labelW = max(labelW, ansi.StringWidth(it.Label))
		detailW = max(detailW, ansi.StringWidth(it.Detail))
	}
	inner := min(max(24, labelW+detailW+5), max(12, width-2))
	first := max(0, min(c.Sel-MenuRows/2, len(c.Items)-MenuRows))
	var lines []string
	for i := first; i < len(c.Items) && i < first+MenuRows; i++ {
		it := c.Items[i]
		mark, label, detail := "  ", menuItem, menuDetail
		if i == c.Sel {
			mark, label, detail = "❯ ", menuSel, menuSelDet
		}
		left := mark + it.Label
		gap := inner - ansi.StringWidth(left) - ansi.StringWidth(it.Detail) - 1
		line := label.Render(ansi.Truncate(left, inner, "…"))
		if gap >= 1 && it.Detail != "" {
			line += label.Render(strings.Repeat(" ", gap)) + detail.Render(it.Detail+" ")
		} else {
			line += label.Render(strings.Repeat(" ", max(0, inner-ansi.StringWidth(left))))
		}
		lines = append(lines, line)
	}
	hint := "tab accept · ↑↓ · esc"
	if len(c.Items) > MenuRows {
		hint = itoa(c.Sel+1) + "/" + itoa(len(c.Items)) + " · " + hint
	}
	lines = append(lines, menuHint.Render(ansi.Truncate(hint+strings.Repeat(" ", max(0, inner-ansi.StringWidth(hint))), inner, "")))
	return menuBox.Render(strings.Join(lines, "\n"))
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
