package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestComposeOverlay_KeepsBackgroundBesideOverlay(t *testing.T) {
	bg := strings.TrimSuffix(strings.Repeat("abcdefghij\n", 5), "\n")
	lines := strings.Split(ansi.Strip(composeOverlay(bg, "XX", 10, 5)), "\n")

	// "XX" is 2 wide in a 10x5 screen: column (10-2)/2 = 4, row (5-1)/2 = 2.
	if lines[2] != "abcdXXghij" {
		t.Errorf("overlay row = %q, want %q", lines[2], "abcdXXghij")
	}
	for _, i := range []int{0, 1, 3, 4} {
		if lines[i] != "abcdefghij" {
			t.Errorf("row %d = %q, want the background unchanged", i, lines[i])
		}
	}
}

func TestComposeOverlay_DimsWholeBackgroundUniformly(t *testing.T) {
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	bold := lipgloss.NewStyle().Bold(true)
	styled := red.Render("alpha") + " plain " + bold.Render("beta")
	plain := "alpha plain beta"

	got := composeOverlay(styled, "X", 20, 3)
	want := composeOverlay(plain, "X", 20, 3)
	if got != want {
		t.Errorf("background styling leaked through the dim\n got: %q\nwant: %q", got, want)
	}
	if !strings.Contains(got, "\x1b[2m") {
		t.Errorf("background is not faint: %q", got)
	}
}

func TestComposeOverlay_PadsShortBackgroundToScreenHeight(t *testing.T) {
	got := composeOverlay("one line", "X", 12, 5)
	if n := len(strings.Split(got, "\n")); n != 5 {
		t.Errorf("got %d lines, want 5", n)
	}
}

func TestComposeOverlay_ClipsOversizedOverlayToScreen(t *testing.T) {
	row := strings.Repeat("O", 30)
	overlay := strings.TrimSuffix(strings.Repeat(row+"\n", 10), "\n")
	lines := strings.Split(composeOverlay("bg", overlay, 20, 6), "\n")
	if len(lines) != 6 {
		t.Fatalf("got %d lines, want 6", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 20 {
			t.Errorf("line %d is %d wide, want <= 20", i, w)
		}
	}
}

func TestComposeOverlay_WideGlyphsStayWithinWidth(t *testing.T) {
	row := strings.Repeat("⬢界⎇ab", 4)
	bg := row + "\n" + row + "\n" + row
	for _, width := range []int{9, 10, 11} {
		for i, l := range strings.Split(composeOverlay(bg, "XYZ", width, 3), "\n") {
			if w := lipgloss.Width(l); w > width {
				t.Errorf("width %d: line %d is %d wide", width, i, w)
			}
		}
	}
}

func TestComposeOverlay_ZeroSizeReturnsOverlay(t *testing.T) {
	if got := composeOverlay("bg", "X", 0, 0); got != "X" {
		t.Errorf("got %q, want the overlay unchanged", got)
	}
}

func TestFitLines_TrimsAndPads(t *testing.T) {
	got := fitLines("a\nb\nc\nd", 5, 2)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("trim: got %q", got)
	}
	got = fitLines("a", 5, 3)
	if len(got) != 3 || got[0] != "a" || got[1] != "" || got[2] != "" {
		t.Errorf("pad: got %q", got)
	}
	got = fitLines("abcdefgh", 3, 1)
	if got[0] != "abc" {
		t.Errorf("truncate: got %q", got[0])
	}
}

// An overlay edge that lands inside a wide glyph blanks the orphaned half.
func TestComposeOverlay_BlanksHalfCutWideGlyph(t *testing.T) {
	got := ansi.Strip(composeOverlay("界界界界界", "XXX", 10, 1))
	if got != "界 XXX界界" {
		t.Errorf("got %q, want %q", got, "界 XXX界界")
	}
}
