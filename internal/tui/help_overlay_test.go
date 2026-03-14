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

func TestHelpSectionsForAllViews(t *testing.T) {
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
		sections := helpSectionsFor(view)
		if len(sections) == 0 {
			t.Errorf("helpSectionsFor(%d) returned no sections", view)
		}
		for _, sec := range sections {
			if sec.title == "" {
				t.Errorf("helpSectionsFor(%d) has section with empty title", view)
			}
			if len(sec.items) == 0 {
				t.Errorf("helpSectionsFor(%d) section %q has no items", view, sec.title)
			}
		}
	}
}

func TestHelpSectionsDashboardContent(t *testing.T) {
	sections := helpSectionsFor(ViewDashboard)

	titles := make(map[string]bool)
	for _, sec := range sections {
		titles[sec.title] = true
	}

	required := []string{"Navigation", "Worktree Actions", "CLI Companions"}
	for _, name := range required {
		if !titles[name] {
			t.Errorf("dashboard help missing section %q", name)
		}
	}
}

func TestRenderHelpContentNonEmpty(t *testing.T) {
	content := renderHelpContent(ViewDashboard, 70)
	if content == "" {
		t.Error("renderHelpContent returned empty string")
	}
	if !strings.Contains(content, "Navigation") {
		t.Error("rendered content missing 'Navigation' section title")
	}
	if !strings.Contains(content, "Move up") {
		t.Error("rendered content missing key descriptions")
	}
}

func TestRenderHelpContentMultipleWidths(t *testing.T) {
	// Verify rendering succeeds at various widths without panicking
	for _, w := range []int{20, 50, 70, 90} {
		content := renderHelpContent(ViewDashboard, w)
		if content == "" {
			t.Errorf("renderHelpContent at width %d returned empty string", w)
		}
	}
}

func TestHelpOverlayCacheInvalidation(t *testing.T) {
	h := NewHelpOverlay()

	h.Open(ViewDashboard, 80, 30)
	if _, ok := h.cache[ViewDashboard]; !ok {
		t.Fatal("expected cache entry after first Open")
	}

	// Opening at a different width should clear and rebuild the cache
	h.Open(ViewDelete, 120, 30)
	if _, ok := h.cache[ViewDashboard]; ok {
		t.Error("expected dashboard cache entry to be cleared after width change")
	}
	if _, ok := h.cache[ViewDelete]; !ok {
		t.Error("expected delete cache entry after Open at new width")
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

func TestCalcHelpOverlaySizeMinBounds(t *testing.T) {
	w, h := calcHelpOverlaySize(40, 10)
	if w != 60 {
		t.Errorf("expected min width 60, got %d", w)
	}
	if h != 15 {
		t.Errorf("expected min height 15, got %d", h)
	}
}

func TestCalcHelpOverlaySizeMaxBounds(t *testing.T) {
	w, h := calcHelpOverlaySize(200, 80)
	if w != 100 {
		t.Errorf("expected max width 100, got %d", w)
	}
	if h != 35 {
		t.Errorf("expected max height 35, got %d", h)
	}
}

func TestCalcHelpOverlaySizeNormal(t *testing.T) {
	w, h := calcHelpOverlaySize(100, 40)
	if w != 75 {
		t.Errorf("expected width 75, got %d", w)
	}
	if h != 32 {
		t.Errorf("expected height 32, got %d", h)
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
