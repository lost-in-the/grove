package tui

import (
	"strings"
	"testing"
)

func TestNewHelpOverlay(t *testing.T) {
	h := NewHelpOverlay()
	if h.Active {
		t.Error("expected Active to be false on new overlay")
	}
	if h.cache == nil {
		t.Error("expected cache to be initialized")
	}
}

func TestHelpOverlayOpen(t *testing.T) {
	h := NewHelpOverlay()
	h.Open(ViewDelete, 100, 40)

	if !h.Active {
		t.Error("expected Active to be true after Open")
	}
	if h.ForView != ViewDelete {
		t.Errorf("expected ForView=ViewDelete, got %d", h.ForView)
	}
}

func TestHelpOverlayClose(t *testing.T) {
	h := NewHelpOverlay()
	h.Open(ViewDashboard, 100, 40)
	h.Close()

	if h.Active {
		t.Error("expected Active to be false after Close")
	}
}

func TestHelpContentForAllViews(t *testing.T) {
	views := []ActiveView{
		ViewDashboard,
		ViewHelp,
		ViewDelete,
		ViewCreate,
		ViewBulk,
		ViewPRs,
		ViewIssues,
		ViewFork,
		ViewSync,
		ViewConfig,
		ViewRename,
		ViewCheckout,
	}

	for _, view := range views {
		content := helpContentFor(view)
		if content == "" {
			t.Errorf("helpContentFor(%d) returned empty string", view)
		}
		if !strings.Contains(content, "##") {
			t.Errorf("helpContentFor(%d) missing markdown header", view)
		}
	}
}

func TestHelpContentDashboardSections(t *testing.T) {
	content := helpContentFor(ViewDashboard)

	sections := []string{"Navigation", "Worktree Actions", "CLI Companions"}
	for _, section := range sections {
		if !strings.Contains(content, section) {
			t.Errorf("dashboard help missing section %q", section)
		}
	}
}

func TestCalcHelpOverlaySizeMinBounds(t *testing.T) {
	// Small terminal should hit minimums
	w, h := calcHelpOverlaySize(40, 10)
	if w != 60 {
		t.Errorf("expected min width 60, got %d", w)
	}
	if h != 15 {
		t.Errorf("expected min height 15, got %d", h)
	}
}

func TestCalcHelpOverlaySizeMaxBounds(t *testing.T) {
	// Large terminal should hit maximums
	w, h := calcHelpOverlaySize(200, 80)
	if w != 90 {
		t.Errorf("expected max width 90, got %d", w)
	}
	if h != 35 {
		t.Errorf("expected max height 35, got %d", h)
	}
}

func TestCalcHelpOverlaySizeNormal(t *testing.T) {
	// 100 * 70% = 70, within [60, 90]
	// 40 * 80% = 32, within [15, 35]
	w, h := calcHelpOverlaySize(100, 40)
	if w != 70 {
		t.Errorf("expected width 70, got %d", w)
	}
	if h != 32 {
		t.Errorf("expected height 32, got %d", h)
	}
}

func TestHelpOverlayCacheInvalidation(t *testing.T) {
	h := NewHelpOverlay()

	// Open at width 80
	h.Open(ViewDashboard, 80, 30)
	content80 := h.cache[ViewDashboard]

	// Open at width 120 — cache should be rebuilt
	h.Open(ViewDashboard, 120, 30)
	content120 := h.cache[ViewDashboard]

	if content80 == content120 {
		t.Error("expected different rendered content at different widths")
	}
}

func TestHelpOverlayViewNonEmpty(t *testing.T) {
	h := NewHelpOverlay()
	h.Open(ViewDashboard, 100, 40)

	view := h.View(100, 40)
	if view == "" {
		t.Error("expected non-empty View output")
	}
	if !strings.Contains(view, "scroll") {
		t.Error("expected footer hint containing 'scroll'")
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		val, lo, hi, want int
	}{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{15, 0, 10, 10},
		{0, 0, 10, 0},
		{10, 0, 10, 10},
	}
	for _, tt := range tests {
		got := clamp(tt.val, tt.lo, tt.hi)
		if got != tt.want {
			t.Errorf("clamp(%d, %d, %d) = %d, want %d", tt.val, tt.lo, tt.hi, got, tt.want)
		}
	}
}
