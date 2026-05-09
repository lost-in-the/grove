package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/version"
)

// stripANSIUpdate removes ANSI escape codes for substring assertions.
var stripANSIUpdate = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripUpdateView(s string) string {
	return stripANSIUpdate.ReplaceAllString(s, "")
}

func TestNewUpdateOverlay(t *testing.T) {
	u := NewUpdateOverlay()
	if u.Active {
		t.Error("expected Active to be false on new overlay")
	}
}

func TestUpdateOverlayOpen(t *testing.T) {
	u := NewUpdateOverlay()
	// Use the live version.Version so this test doesn't silently break on the
	// next dev-cycle version bump. Whatever the current build's version is,
	// Open should round-trip it through currentVersion unchanged.
	u.Open(version.Version, "99.0.0", "https://github.com/lost-in-the/grove/releases/tag/v99.0.0")

	if !u.Active {
		t.Error("expected Active to be true after Open")
	}
	if u.currentVersion != version.Version {
		t.Errorf("currentVersion = %q, want %q", u.currentVersion, version.Version)
	}
	if u.latestVersion != "99.0.0" {
		t.Errorf("latestVersion = %q, want %q", u.latestVersion, "99.0.0")
	}
	if u.latestURL != "https://github.com/lost-in-the/grove/releases/tag/v99.0.0" {
		t.Errorf("latestURL = %q, want full release tag URL", u.latestURL)
	}
}

func TestUpdateOverlayClose(t *testing.T) {
	u := NewUpdateOverlay()
	u.Open("0.7.0", "0.8.0", "https://x")
	u.Close()
	if u.Active {
		t.Error("expected Active to be false after Close")
	}
}

func TestUpdateOverlayViewContent(t *testing.T) {
	u := NewUpdateOverlay()
	u.Open("0.6.0", "0.7.0", "https://github.com/lost-in-the/grove/releases/tag/v0.7.0")

	view := u.View(120, 40)
	if view == "" {
		t.Fatal("expected non-empty View output")
	}

	plain := stripUpdateView(view)

	wantSubstrings := []string{
		"Update available",
		"Current",
		"Latest",
		"0.6.0",
		"0.7.0",
		// All three install methods are always shown; user picks the one
		// matching their setup.
		"Brew",
		"Go",
		"Binary",
		"Changelog",
		"close",
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(plain, s) {
			t.Errorf("expected View to contain %q\n--- plain ---\n%s", s, plain)
		}
	}

	// Verify the actual install commands are rendered. The URL/command may
	// wrap onto multiple cells (and across the border glyph). Strip border
	// glyphs + whitespace before checking continuity.
	flattened := plain
	for _, r := range []string{"│", "╭", "╮", "╰", "╯", "─"} {
		flattened = strings.ReplaceAll(flattened, r, "")
	}
	flattened = strings.Join(strings.Fields(flattened), "")

	wantFlattened := []string{
		"github.com/lost-in-the/grove/releases/tag/v0.7.0",
		"brewupgradelost-in-the/tap/grove",
		"goinstallgithub.com/lost-in-the/grove/cmd/grove@latest",
		"github.com/lost-in-the/grove/releases",
	}
	for _, s := range wantFlattened {
		if !strings.Contains(flattened, s) {
			t.Errorf("expected flattened View to contain %q\n--- plain ---\n%s", s, plain)
		}
	}

	// Footer should combine both keys with a single "close" action label —
	// rendered as "esc/u   close" (slash convention for compound keys).
	closeCount := strings.Count(plain, "close")
	if closeCount != 1 {
		t.Errorf("expected footer to render the word \"close\" exactly once, got %d\n--- plain ---\n%s", closeCount, plain)
	}
	if !strings.Contains(plain, "esc/u") {
		t.Errorf("expected footer to render compound close keys as \"esc/u\"\n--- plain ---\n%s", plain)
	}
}

func TestUpdateOverlayViewOmitsChangelogWhenURLEmpty(t *testing.T) {
	u := NewUpdateOverlay()
	u.Open("0.6.0", "0.7.0", "")

	view := stripUpdateView(u.View(120, 40))
	if strings.Contains(view, "Changelog") {
		t.Errorf("expected View to omit Changelog row when URL empty\n--- view ---\n%s", view)
	}
}

func TestUpdateOverlayViewMultipleWidths(t *testing.T) {
	u := NewUpdateOverlay()
	u.Open("0.6.0", "0.7.0", "https://x")
	for _, w := range []int{60, 80, 120, 200} {
		view := u.View(w, 40)
		if view == "" {
			t.Errorf("View at width %d returned empty", w)
		}
	}
}

func TestCalcUpdateOverlaySize_Bounds(t *testing.T) {
	tests := []struct {
		termW, termH       int
		wantW              int
		wantHMin, wantHMax int
	}{
		// Tiny terminal — w clamps to termW so it doesn't overflow.
		{40, 10, 40, 14, 14},
		// Standard 80x24 — target 74 to fit go-install command without wrap.
		{80, 24, 74, 14, 14},
		// Mid-range — target stays 74 until termW*70/100 exceeds it.
		{120, 40, 80, 22, 22},
		// Ceiling
		{300, 100, 80, 22, 22},
	}
	for _, tt := range tests {
		w, h := calcUpdateOverlaySize(tt.termW, tt.termH)
		if w != tt.wantW {
			t.Errorf("calcUpdateOverlaySize(%d,%d) w = %d, want %d", tt.termW, tt.termH, w, tt.wantW)
		}
		if h < tt.wantHMin || h > tt.wantHMax {
			t.Errorf("calcUpdateOverlaySize(%d,%d) h = %d, want in [%d,%d]", tt.termW, tt.termH, h, tt.wantHMin, tt.wantHMax)
		}
	}
}
