package tui

import (
	"strings"
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
		// A last segment too long to keep whole, or no "-" at all: the
		// cut is still in the middle, never at the end.
		{"a-verylongtailsegmentthatisbig", 16, "a-veryl…hatisbig"},
		{"nodashesatalltoolong", 16, "nodashe…ltoolong"},
		// Keeping the last segment would leave too little of the start.
		{"fix/login-timeout", 10, "fix/…meout"},
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

// cutMiddle keeps the start and the end and measures in display cells, so
// wide glyphs never push it past w.
func TestCutMiddle(t *testing.T) {
	for _, c := range []struct {
		in   string
		w    int
		want string
	}{
		{"short", 10, "short"},
		{"exactly-ten", 11, "exactly-ten"},
		{"agent/payments-reconciliation-audit-x7k2p", 21, "agent/paym…udit-x7k2p"},
		{"nodashesatalltoolong", 16, "nodashe…ltoolong"},
		{"abcdef", 2, "…f"},
		{"abcdef", 1, "…"},
		{"abcdef", 0, ""},
		{"日本語のブランチ名", 9, "日本…チ名"},
		{"⎇⎇⎇⎇⎇⎇⎇⎇⎇⎇", 5, "⎇⎇…⎇⎇"},
	} {
		got := cutMiddle(c.in, c.w)
		if got != c.want {
			t.Errorf("cutMiddle(%q, %d) = %q, want %q", c.in, c.w, got, c.want)
		}
		if w := lipgloss.Width(got); w > c.w {
			t.Errorf("cutMiddle(%q, %d) is %d cells wide", c.in, c.w, w)
		}
	}
}

// Wide glyphs that cannot split still leave the start and the end, inside
// w, from 5 cells (two glyphs and the …).
func TestCutMiddle_WideGlyphsStayInside(t *testing.T) {
	in := "日本語のブランチ名"
	for w := 5; w < lipgloss.Width(in); w++ {
		got := cutMiddle(in, w)
		if lipgloss.Width(got) > w {
			t.Errorf("width %d: %q is %d cells", w, got, lipgloss.Width(got))
		}
		if !strings.HasPrefix(got, "日") || !strings.HasSuffix(got, "名") || !strings.Contains(got, "…") {
			t.Errorf("width %d: %q lost the start, the end, or the …", w, got)
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

// countsWidth measures rowCounts without building it.
func TestCountsWidth(t *testing.T) {
	for _, c := range [][3]int{{0, 0, 0}, {2, 11, 13}, {0, 0, 4}, {123, 0, 1}, {9, 10, 0}} {
		if got, want := countsWidth(c[0], c[1], c[2]), lipgloss.Width(rowCounts(c[0], c[1], c[2], false)); got != want {
			t.Errorf("countsWidth%v = %d, want %d", c, got, want)
		}
	}
}

// The name column is a third of the list width, between 12 and 32 cells,
// and no wider than the longest name needs.
func TestLayoutCompact_NameColumn(t *testing.T) {
	for _, c := range []struct {
		width, longest, want int
	}{
		{70, 29, 23}, // the list beside the pane at 120 columns
		{78, 29, 26}, // the list alone at 80 columns
		{94, 29, 29}, // the list beside the pane at 160 columns
		{120, 40, 32},
		{70, 6, 8}, // short names: the branch gets the room
	} {
		if got := layoutCompact(c.width, 0, c.longest, 10, true).NameW; got != c.want {
			t.Errorf("width %d longest %d: name column %d, want %d", c.width, c.longest, got, c.want)
		}
	}
}

// The columns fill the list exactly, so the state, age, and tail end at the
// right edge and every branch starts in one column.
func TestLayoutCompact_FillsTheWidth(t *testing.T) {
	for _, lead := range []int{0, 2} {
		for _, longest := range []int{4, 16, 29, 80} {
			for _, stateW := range []int{5, 10, 11} {
				for _, second := range []bool{true, false} {
					for w := 42; w <= 200; w++ {
						l := layoutCompact(w, lead, longest, stateW, second)
						if got := l.total(); got != w {
							t.Fatalf("lead %d longest %d state %d second %v width %d: row is %d cells (%+v)", lead, longest, stateW, second, w, got, l)
						}
						if l.NameW < rowNameMin {
							t.Fatalf("width %d: name column %d below %d", w, l.NameW, rowNameMin)
						}
						if second && l.SecondW < rowSecondMin {
							t.Fatalf("width %d lead %d: branch column %d below %d (%+v)", w, lead, l.SecondW, rowSecondMin, l)
						}
					}
				}
			}
		}
	}
}

// As the list narrows: the counts collapse first, then the age goes, and
// only then does the name column give way.
func TestLayoutCompact_ShedOrder(t *testing.T) {
	first := map[string]int{}
	for w := 200; w >= 42; w-- {
		l := layoutCompact(w, 2, 29, 10, true)
		note := func(event string, happened bool) {
			if _, seen := first[event]; happened && !seen {
				first[event] = w
			}
		}
		note("counts", l.Collapsed)
		note("age", !l.ShowAge)
		note("name", l.NameW < min(max(w/3, rowNameFloor), rowNameCap, 29))
	}
	order := []string{"counts", "age", "name"}
	for _, e := range order {
		if _, ok := first[e]; !ok {
			t.Fatalf("%s never shed between 200 and 42 (%v)", e, first)
		}
	}
	for i := 1; i < len(order); i++ {
		if a, b := order[i-1], order[i]; first[a] < first[b] {
			t.Errorf("%s shed at %d, after %s at %d; want %s first", a, first[a], b, first[b], a)
		}
	}
}
