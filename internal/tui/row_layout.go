package tui

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Fixed dashboard row columns, in cells.
const (
	rowLeadW       = 2  // pin, agent, cursor, and status each take 2
	rowCountsW     = 11 // ↑n ↓n ~n, right-aligned
	rowCountsTight = 5  // one collapsed count, e.g. ↓123; also "STATE"
	rowAgeW        = 4  // git-style age, up to "11mo"
	// rowTailW is the session glyph, a gap, the container glyph, a gap, and
	// one cell kept blank for a pull request mark.
	rowTailW = 5

	rowNameFloor = 12 // compact name column: a third of the list, at least this
	rowNameCap   = 32 // and at most this
	rowNameMin   = 8  // narrowest name column, when the list is narrow or names are short
	rowSecondMin = 12 // compact branch column before anything else gives way
)

// compactLayout is the shape every compact row takes at one list width. It
// is decided once per list, so columns line up from row to row. A row is:
// lead, cursor, status, name, gap, branch, gap, state, gap, age, gap, tail.
type compactLayout struct {
	Lead      int  // agent column cells: 0, or rowLeadW under herdr
	NameW     int  // name column, the ≠ mark included
	Second    bool // a branch (or folder) column; false when list_secondary is off
	SecondW   int
	StateW    int
	Collapsed bool // the state shows only the strongest count
	ShowAge   bool
}

// fixed is the row width without the branch column's own cells.
func (l compactLayout) fixed() int {
	w := l.Lead + 2*rowLeadW + l.NameW + 1 + l.StateW + 1 + rowTailW
	if l.Second {
		w++
	}
	if l.ShowAge {
		w += 1 + rowAgeW
	}
	return w
}

// total is the row width the layout uses.
func (l compactLayout) total() int { return l.fixed() + l.SecondW }

// layoutCompact sizes the compact columns for a list width. The name column
// is a third of the width, between rowNameFloor and rowNameCap, and no
// wider than the longest name needs (at least rowNameMin); the branch column
// takes the rest. When the branch column would drop under rowSecondMin, the
// state collapses to its strongest count, then the age goes, then the name
// column narrows to rowNameMin. With second false (list_secondary off) the
// name column takes the branch column's room.
func layoutCompact(width, lead, longestName, stateW int, second bool) compactLayout {
	l := compactLayout{Lead: lead, Second: second, ShowAge: true, StateW: min(max(stateW, rowCountsTight), rowCountsW)}
	if !second {
		l.NameW = max(rowNameMin, width-l.fixed())
		return l
	}
	l.NameW = min(min(max(width/3, rowNameFloor), rowNameCap), max(longestName, rowNameMin))
	room := func() int { return width - l.fixed() }
	if room() < rowSecondMin && l.StateW > rowCountsTight {
		l.StateW, l.Collapsed = rowCountsTight, true
	}
	if room() < rowSecondMin {
		l.ShowAge = false
	}
	if short := rowSecondMin - room(); short > 0 {
		l.NameW = max(rowNameMin, l.NameW-short)
	}
	l.SecondW = max(0, room())
	return l
}

// countsWidth is the width of rowCounts(ahead, behind, dirty, false),
// computed without building the string.
func countsWidth(ahead, behind, dirty int) int {
	w, n := 0, 0
	for _, c := range [3]int{ahead, behind, dirty} {
		if c > 0 {
			w += 1 + digits(c)
			n++
		}
	}
	if n > 1 {
		w += n - 1
	}
	return w
}

func digits(n int) int {
	d := 1
	for n >= 10 {
		n /= 10
		d++
	}
	return d
}

// rowCounts formats ahead, behind, and dirty counts. Full form lists all
// that are nonzero as ↑ ↓ ~; collapsed keeps only the strongest signal:
// behind, then dirty, then ahead.
func rowCounts(ahead, behind, dirty int, collapsed bool) string {
	if collapsed {
		switch {
		case behind > 0:
			return fmt.Sprintf("↓%d", behind)
		case dirty > 0:
			return fmt.Sprintf("~%d", dirty)
		case ahead > 0:
			return fmt.Sprintf("↑%d", ahead)
		}
		return ""
	}
	var parts []string
	if ahead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", ahead))
	}
	if behind > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", behind))
	}
	if dirty > 0 {
		parts = append(parts, fmt.Sprintf("~%d", dirty))
	}
	return strings.Join(parts, " ")
}

var ticketID = regexp.MustCompile(`^[A-Za-z]+-\d+`)

// middleTruncate shortens s to at most w cells by cutting from the middle.
// It keeps the last "-" segment and as much of the head as fits, trimmed
// back to a "-" boundary but never shorter than a leading ticket id, so
// "abc-1349-migrate-the-read-path" becomes "abc-1349-migrate-…-path". When
// keeping the last segment would leave the head less than half, it cuts
// evenly instead (cutMiddle).
func middleTruncate(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	i := strings.LastIndex(s, "-")
	if i <= 0 {
		return cutMiddle(s, w)
	}
	tail := s[i:]
	budget := w - 1 - lipgloss.Width(tail)
	if budget < max(1, (w-1)/2) {
		return cutMiddle(s, w)
	}
	head := ansi.Truncate(s, budget, "")
	// Trim back to a "-" boundary when that costs only a few cells, never
	// cutting into a leading ticket id.
	if j := strings.LastIndex(head, "-"); j > 0 && len(head)-(j+1) <= 3 && j+1 >= len(ticketID.FindString(s)) {
		head = head[:j+1]
	}
	return head + "…" + tail
}

// cutMiddle shortens s to at most w display cells, keeping its start and
// its end around "…". The end gets the odd cell.
func cutMiddle(s string, w int) string {
	sw := lipgloss.Width(s)
	switch {
	case sw <= w:
		return s
	case w <= 0:
		return ""
	case w == 1:
		return "…"
	}
	head := ansi.Truncate(s, (w-1)/2, "")
	keep := w - 1 - lipgloss.Width(head)
	tail := ansi.TruncateLeft(s, sw-keep, "")
	for lipgloss.Width(tail) > keep {
		_, size := firstRune(tail)
		tail = tail[size:]
	}
	return head + "…" + tail
}

func firstRune(s string) (rune, int) {
	for i, r := range s {
		if i > 0 {
			return r, i
		}
	}
	return 0, len(s)
}
