package tui

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Fixed dashboard row columns, in cells (spec 5.1).
const (
	rowLeadW       = 2  // pin, agent, cursor, and status each take 2
	rowCountsW     = 11 // ↑n ↓n ~n, right-aligned
	rowCountsTight = 5  // one collapsed count, e.g. ↓123
	rowAgeW        = 3
	rowTailW       = 4 // session and container glyphs
	rowAliasFloor  = 16
	rowChipMin     = 6  // "⎇ " plus a few characters
	rowAliasCap    = 32 // longer aliases truncate rather than shed columns
)

// rowLayout is the shape every dashboard row takes at one list width. It is
// decided once per list, so columns line up from row to row.
type rowLayout struct {
	Lead      int  // optional lead columns before the cursor: pin, agent
	AliasW    int  // alias column width
	ChipW     int  // chip column width; 0 when the chip is shed
	Collapsed bool // counts show only the strongest signal
	ShowAge   bool
}

// countsW is the counts column width for the layout.
func (l rowLayout) countsW() int {
	if l.Collapsed {
		return rowCountsTight
	}
	return rowCountsW
}

// total is the row width the layout uses: lead, cursor, and status columns,
// then alias, chip, counts, age, and tail, each after a one-cell gap.
func (l rowLayout) total() int {
	w := (l.Lead+2)*rowLeadW + l.AliasW
	if l.ChipW > 0 {
		w += 1 + l.ChipW
	}
	w += 1 + l.countsW()
	if l.ShowAge {
		w += 1 + rowAgeW
	}
	return w + 1 + rowTailW
}

// layoutColumns applies the shed order (spec 5.2) for a list width: drop
// the chip, then the age, then collapse the counts, then truncate the alias
// down to 16 cells. lead is the number of optional lead columns (pin,
// agent); maxAlias is the widest alias in the list.
func layoutColumns(width, lead, maxAlias int, anyChip bool) rowLayout {
	need := min(max(maxAlias, rowAliasFloor), rowAliasCap)
	l := rowLayout{Lead: lead, ShowAge: true}
	room := func() int {
		l.AliasW, l.ChipW = 0, 0
		return width - l.total()
	}
	r := room()
	if r < need && l.ShowAge {
		l.ShowAge = false
		r = room()
	}
	if r < need {
		l.Collapsed = true
		r = room()
	}
	l.AliasW = max(rowAliasFloor, min(need, r))
	// The chip is first to go: it shows only while nothing else has shed
	// and the alias has all it needs, so it never squeezes the alias.
	if spare := r - need - 1; anyChip && l.ShowAge && !l.Collapsed && spare >= rowChipMin {
		l.ChipW = spare
	}
	return l
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
// "abc-1349-migrate-the-read-path" becomes "abc-1349-migrate-…-path".
func middleTruncate(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	i := strings.LastIndex(s, "-")
	if i <= 0 {
		return ansi.Truncate(s, w, "…")
	}
	tail := s[i:]
	budget := w - 1 - lipgloss.Width(tail)
	if budget < 1 {
		return ansi.Truncate(s, w, "…")
	}
	head := ansi.Truncate(s, budget, "")
	if j := strings.LastIndex(head, "-"); j > 0 && j+1 >= len(ticketID.FindString(s)) {
		head = head[:j+1]
	}
	return head + "…" + tail
}
