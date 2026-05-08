package tui

import (
	"regexp"
	"strings"
	"testing"
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
	u.Open("0.7.0-dev", "99.0.0", "https://github.com/lost-in-the/grove/releases/tag/v99.0.0")

	if !u.Active {
		t.Error("expected Active to be true after Open")
	}
	if u.currentVersion != "0.7.0-dev" {
		t.Errorf("currentVersion = %q, want %q", u.currentVersion, "0.7.0-dev")
	}
	if u.latestVersion != "99.0.0" {
		t.Errorf("latestVersion = %q, want %q", u.latestVersion, "99.0.0")
	}
	if u.updateCommand == "" {
		t.Error("expected updateCommand to be populated after Open")
	}
	if u.updateLabel == "" {
		t.Error("expected updateLabel to be populated after Open")
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

	// Label is contextual: "Run" for brew/go-install, "Download" for binary.
	// In tests the running binary lives under go test cache → InstallBinary →
	// "Download". Just check that one or the other is present.
	if !strings.Contains(stripUpdateView(view), "Run") && !strings.Contains(stripUpdateView(view), "Download") {
		t.Errorf("expected View to contain command label (Run or Download)\n--- plain ---\n%s", stripUpdateView(view))
	}

	wantSubstrings := []string{
		"Update available",
		"Current",
		"Latest",
		"0.6.0",
		"0.7.0",
		"Changelog",
		"close",
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(plain, s) {
			t.Errorf("expected View to contain %q\n--- plain ---\n%s", s, plain)
		}
	}

	// URL may wrap onto multiple cells (and across the border glyph). Strip
	// border glyphs + whitespace before checking continuity.
	flattened := plain
	for _, r := range []string{"│", "╭", "╮", "╰", "╯", "─"} {
		flattened = strings.ReplaceAll(flattened, r, "")
	}
	flattened = strings.Join(strings.Fields(flattened), "")
	if !strings.Contains(flattened, "github.com/lost-in-the/grove/releases/tag/v0.7.0") {
		t.Errorf("expected View to contain release URL\n--- plain ---\n%s", plain)
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
		// Floor
		{40, 10, 50, 11, 11},
		// Mid-range
		{120, 40, 72, 16, 16},
		// Ceiling
		{300, 100, 80, 16, 16},
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
