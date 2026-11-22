// Package diff compares two texts line by line.
package diff

import (
	"fmt"
	"strings"
)

// Op is the kind of a diff line.
type Op int

const (
	Equal Op = iota
	Delete
	Insert
)

// Line is one line of a diff.
type Line struct {
	Op   Op
	Text string
}

// maxCells bounds the LCS table; larger inputs are compared by replacing
// the differing middle as a whole.
const maxCells = 4_000_000

// Lines returns the line diff turning a into b.
func Lines(a, b string) []Line {
	al, bl := split(a), split(b)
	// Common prefix and suffix are cheap and keep the table small.
	pre := 0
	for pre < len(al) && pre < len(bl) && al[pre] == bl[pre] {
		pre++
	}
	suf := 0
	for suf < len(al)-pre && suf < len(bl)-pre && al[len(al)-1-suf] == bl[len(bl)-1-suf] {
		suf++
	}
	var out []Line
	for _, l := range al[:pre] {
		out = append(out, Line{Equal, l})
	}
	out = append(out, middle(al[pre:len(al)-suf], bl[pre:len(bl)-suf])...)
	for _, l := range al[len(al)-suf:] {
		out = append(out, Line{Equal, l})
	}
	return out
}

func split(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func middle(a, b []string) []Line {
	var out []Line
	if len(a)*len(b) > maxCells {
		for _, l := range a {
			out = append(out, Line{Delete, l})
		}
		for _, l := range b {
			out = append(out, Line{Insert, l})
		}
		return out
	}
	// lcs[i][j] = LCS length of a[i:] and b[j:]
	lcs := make([][]int32, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int32, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, Line{Equal, a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, Line{Delete, a[i]})
			i++
		default:
			out = append(out, Line{Insert, b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		out = append(out, Line{Delete, a[i]})
	}
	for ; j < len(b); j++ {
		out = append(out, Line{Insert, b[j]})
	}
	return out
}

// Changed reports whether the diff has any insertion or deletion.
func Changed(lines []Line) bool {
	for _, l := range lines {
		if l.Op != Equal {
			return true
		}
	}
	return false
}

// Stats counts inserted and deleted lines.
func Stats(lines []Line) (added, removed int) {
	for _, l := range lines {
		switch l.Op {
		case Insert:
			added++
		case Delete:
			removed++
		}
	}
	return
}

// Hunk is a run of lines to show; Skipped equal lines precede it.
type Hunk struct {
	Skipped int
	Lines   []Line
}

// Hunks keeps context equal lines around each change and folds the rest.
func Hunks(lines []Line, context int) []Hunk {
	keep := make([]bool, len(lines))
	for i, l := range lines {
		if l.Op == Equal {
			continue
		}
		for k := max(0, i-context); k <= min(len(lines)-1, i+context); k++ {
			keep[k] = true
		}
	}
	var hunks []Hunk
	skipped := 0
	var cur *Hunk
	for i, l := range lines {
		if !keep[i] {
			skipped++
			cur = nil
			continue
		}
		if cur == nil {
			hunks = append(hunks, Hunk{Skipped: skipped})
			cur = &hunks[len(hunks)-1]
			skipped = 0
		}
		cur.Lines = append(cur.Lines, l)
	}
	if skipped > 0 {
		hunks = append(hunks, Hunk{Skipped: skipped})
	}
	return hunks
}

// Unified renders a plain unified-style diff (no file headers).
func Unified(lines []Line, context int) string {
	var b strings.Builder
	for _, h := range Hunks(lines, context) {
		if h.Skipped > 0 {
			fmt.Fprintf(&b, "@@ %d unchanged lines @@\n", h.Skipped)
		}
		for _, l := range h.Lines {
			switch l.Op {
			case Insert:
				b.WriteString("+ ")
			case Delete:
				b.WriteString("- ")
			default:
				b.WriteString("  ")
			}
			b.WriteString(l.Text + "\n")
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}
