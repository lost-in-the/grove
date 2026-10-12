package tui

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestMiddleTruncate(t *testing.T) {
	for _, c := range []struct {
		in   string
		w    int
		want string
	}{
		{"fix-bug", 16, "fix-bug"},
		{"abc-1349-migrate-the-read-path", 24, "abc-1349-migrate-…-path"},
		{"refactor-the-whole-thing", 16, "refactor-…-thing"},
		{"abc-1349-x-y-z-long-ending", 16, "abc-1349…-ending"},
		{"a-verylongtailsegmentthatisbig", 16, "a-verylongtails…"},
		{"nodashesatalltoolong", 16, "nodashesatallto…"},
	} {
		got := middleTruncate(c.in, c.w)
		if got != c.want {
			t.Errorf("middleTruncate(%q, %d) = %q, want %q", c.in, c.w, got, c.want)
		}
		if w := lipgloss.Width(got); w > c.w {
			t.Errorf("middleTruncate(%q, %d) is %d cells wide", c.in, c.w, w)
		}
	}
}

// The chip goes first, then the age, then the counts collapse, and only
// then does the alias shrink, down to 16.
func TestLayoutColumns_ShedOrder(t *testing.T) {
	first := map[string]int{}
	note := func(event string, w int) {
		if _, ok := first[event]; !ok {
			first[event] = w
		}
	}
	for w := 160; w >= 44; w-- {
		l := layoutColumns(w, 2, 30, true)
		if l.ChipW == 0 {
			note("chip", w)
		}
		if !l.ShowAge {
			note("age", w)
		}
		if l.Collapsed {
			note("counts", w)
		}
		if l.AliasW < 30 {
			note("alias", w)
		}
		if l.AliasW < 16 {
			t.Fatalf("width %d: alias %d below the 16 floor", w, l.AliasW)
		}
	}
	order := []string{"chip", "age", "counts", "alias"}
	for i := 1; i < len(order); i++ {
		a, b := order[i-1], order[i]
		if first[a] < first[b] {
			t.Errorf("%s shed at %d, after %s at %d; want %s first", a, first[a], b, first[b], a)
		}
	}
	for _, e := range order {
		if _, ok := first[e]; !ok {
			t.Errorf("%s never shed between 160 and 44", e)
		}
	}
}

// The chip sheds before the alias would drop below 24 cells.
func TestLayoutColumns_ChipNeverSqueezesAlias(t *testing.T) {
	for w := 44; w <= 200; w++ {
		l := layoutColumns(w, 2, 30, true)
		if l.ChipW > 0 && l.AliasW < 24 {
			t.Errorf("width %d: chip shown with alias at %d", w, l.AliasW)
		}
	}
}

func TestLayoutColumns_NeverOverflows(t *testing.T) {
	for _, lead := range []int{0, 1, 2} {
		for _, maxAlias := range []int{4, 16, 30, 80} {
			for _, chip := range []bool{false, true} {
				for w := 44; w <= 200; w++ {
					l := layoutColumns(w, lead, maxAlias, chip)
					if got := l.total(); got > w {
						t.Fatalf("lead %d alias %d chip %v width %d: row is %d cells", lead, maxAlias, chip, w, got)
					}
					if l.AliasW < 16 {
						t.Fatalf("lead %d alias %d width %d: alias %d below floor", lead, maxAlias, w, l.AliasW)
					}
				}
			}
		}
	}
}

func TestRowCounts(t *testing.T) {
	if got := rowCounts(2, 11, 13, false); got != "↑2 ↓11 ~13" {
		t.Errorf("full = %q", got)
	}
	for _, c := range []struct {
		ahead, behind, dirty int
		want                 string
	}{
		{2, 11, 13, "↓11"}, {2, 0, 13, "~13"}, {2, 0, 0, "↑2"}, {0, 0, 0, ""},
	} {
		if got := rowCounts(c.ahead, c.behind, c.dirty, true); got != c.want {
			t.Errorf("collapsed %v = %q, want %q", c, got, c.want)
		}
	}
}

// A chip too short to name the branch is dropped instead of shown.
func TestLayoutColumns_ChipIsReadableOrGone(t *testing.T) {
	for w := 44; w <= 200; w++ {
		if l := layoutColumns(w, 0, 16, true); l.ChipW > 0 && l.ChipW < 10 {
			t.Errorf("width %d: chip is %d cells", w, l.ChipW)
		}
	}
}

// With room to spare, a long alias gets its full width; the 32 cap only
// governs shedding.
func TestLayoutColumns_LongAliasUsesSpareRoom(t *testing.T) {
	for _, w := range []int{120, 200} {
		if l := layoutColumns(w, 0, 47, true); l.AliasW != 47 {
			t.Errorf("width %d: alias %d, want 47 (chip %d)", w, l.AliasW, l.ChipW)
		}
	}
}

// Trimming the head back to a "-" boundary never wastes more than a few
// cells.
func TestMiddleTruncate_DoesNotWasteSpace(t *testing.T) {
	for w := 16; w <= 28; w++ {
		got := middleTruncate("abc-1349-migrate-the-read-path", w)
		if lipgloss.Width(got) < w-3 {
			t.Errorf("width %d: %q is %d cells", w, got, lipgloss.Width(got))
		}
	}
}
