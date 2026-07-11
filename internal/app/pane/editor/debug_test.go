package editor

import (
	"fmt"
	"strings"
	"testing"

	zone "github.com/lrstanley/bubblezone"
	"github.com/charmbracelet/x/ansi"
)

func visible(s string) string {
	return ansi.Truncate(s, ansi.StringWidth(s), "")
}

func TestDebugLayout(t *testing.T) {
	zone.NewGlobal()
	defer zone.Close()
	rendered := DebugRenderPane(194, 46)
	lines := strings.Split(rendered, "\n")

	fmt.Printf("Total rendered lines: %d\n", len(lines))

	for i, line := range lines {
		w := ansi.StringWidth(line)
		v := visible(line)
		_ = v

		switch i {
		case 0:
			// Method row: method + server + uri + send
			parts := strings.Split(v, "┐")
			methodW := ansi.StringWidth(parts[0] + "┐")
			fmt.Printf("  method row: %d (method + server + uri + send)\n", w)
			fmt.Printf("  method selector: %d\n", methodW)

		case 3:
			// Top borders of tabs + response panels
			// Split at ┐┌ boundary
			rest := v
			tabW := 0
			for i, ch := range rest {
				if ch == '┐' {
					tabW = i + 1
					rest = rest[i+1:]
					break
				}
			}
			// Count next box from ┌ to ┐
			respW := 0
			for i, ch := range rest {
				if ch == '┐' {
					respW = i + 1
					break
				}
			}
			fmt.Printf("  tabs top border: %d, response top border: %d, sum: %d\n", tabW, respW, tabW+respW)

		case 40:
			// Inner bottom borders
			rest := v
			tabInner := 0
			for i, ch := range rest {
				if ch == '┘' {
					tabInner = i + 1
					rest = rest[i+1:]
					break
				}
			}
			respInner := 0
			for i, ch := range rest {
				if ch == '┘' {
					respInner = i + 1
					break
				}
			}
			fmt.Printf("  tabs inner bottom: %d, response inner bottom: %d\n", tabInner, respInner)

		case 42:
			fmt.Printf("  outer tabs bottom border: %d\n", w)
		}
	}

	// Count description content area
	descStart, descEnd := -1, -1
	for i, line := range lines {
		v := visible(line)
		if i >= 9 && descStart < 0 && strings.HasPrefix(v, "││") {
			descStart = 9
		}
		if descStart >= 0 && strings.Contains(v, "└") && i > descStart {
			descEnd = i - 1
			break
		}
	}

	fmt.Printf("\n=== MEASUREMENTS ===\n")
	fmt.Printf("Pane:         194 x  46\n")
	fmt.Printf("Tabs width:      97  (inner: 95)\n")
	fmt.Printf("Response width:  97  (inner: 95)\n")
	fmt.Printf("Summary field:   95  (border) → textinput: 93\n")
	descLines := descEnd - descStart + 1
	fmt.Printf("Description:     %d lines (content) → %d with border\n", descLines, descLines+2)
	fmt.Printf("Tabs content:    %d lines (tab bar 2 + doc %d)\n", 2+descLines+2+3, descLines+2+3)
	fmt.Printf("Expected tabsHeight: 38\n")
}
