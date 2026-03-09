package tui

import (
	"strings"
	"testing"
	"time"
)

func TestHighlightSetAndQuery(t *testing.T) {
	hf := NewHelpFooter()

	// No highlight initially
	if hf.IsHighlighted("n") {
		t.Error("expected no highlight before SetHighlight")
	}

	hf.SetHighlight("n")

	if !hf.IsHighlighted("n") {
		t.Error("expected 'n' to be highlighted after SetHighlight")
	}
	if hf.IsHighlighted("d") {
		t.Error("expected 'd' not to be highlighted")
	}
}

func TestHighlightExpiry(t *testing.T) {
	hf := NewHelpFooter()
	hf.SetHighlight("n")

	// Manually backdate the timestamp to simulate expiry
	hf.highlightedAt = time.Now().Add(-600 * time.Millisecond)

	if hf.IsHighlighted("n") {
		t.Error("expected highlight to be expired after 600ms")
	}
}

func TestHighlightClearExpired(t *testing.T) {
	hf := NewHelpFooter()
	hf.SetHighlight("d")

	// Not expired yet -- ClearExpiredHighlight should return false
	if hf.ClearExpiredHighlight() {
		t.Error("expected ClearExpiredHighlight to return false for fresh highlight")
	}
	if hf.highlightedKey != "d" {
		t.Error("expected key to remain after ClearExpiredHighlight on fresh highlight")
	}

	// Backdate to simulate expiry
	hf.highlightedAt = time.Now().Add(-600 * time.Millisecond)

	if !hf.ClearExpiredHighlight() {
		t.Error("expected ClearExpiredHighlight to return true for expired highlight")
	}
	if hf.highlightedKey != "" {
		t.Error("expected key to be cleared after ClearExpiredHighlight")
	}
}

func TestHighlightClearExpiredNoHighlight(t *testing.T) {
	hf := NewHelpFooter()

	// No highlight set -- should return false and not panic
	if hf.ClearExpiredHighlight() {
		t.Error("expected ClearExpiredHighlight to return false when no highlight is set")
	}
}

func TestHighlightOverwrite(t *testing.T) {
	hf := NewHelpFooter()
	hf.SetHighlight("n")
	hf.SetHighlight("d")

	if hf.IsHighlighted("n") {
		t.Error("expected 'n' highlight to be replaced by 'd'")
	}
	if !hf.IsHighlighted("d") {
		t.Error("expected 'd' to be highlighted after overwrite")
	}
}

func TestRenderCompactHighlightsMatchedKey(t *testing.T) {
	hf := NewHelpFooter()
	hf.SetHighlight("n")

	// Render without highlight -- "n" uses normal HelpKey style
	hfNoHighlight := NewHelpFooter()
	normalRender := hfNoHighlight.RenderCompact(ViewDashboard, 200)
	highlightRender := hf.RenderCompact(ViewDashboard, 200)

	// The two renders should differ (the highlighted one uses a different style)
	if normalRender == highlightRender {
		t.Error("expected highlighted render to differ from normal render")
	}

	// Both should still contain the key descriptions
	if !strings.Contains(highlightRender, "new") {
		t.Error("highlighted render should still contain 'new' description")
	}
	if !strings.Contains(highlightRender, "navigate") {
		t.Error("highlighted render should still contain 'navigate' description")
	}
}
