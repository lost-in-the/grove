package tui

import (
	"strings"
	"testing"
)

func TestHelpFooterCompactHints(t *testing.T) {
	tests := []struct {
		name     string
		view     ActiveView
		wantKeys []string
	}{
		{"Dashboard hints", ViewDashboard, []string{"↑↓", "enter", "n", "d", "?", "f", "s", "c"}},
		{"Create hints", ViewCreate, []string{"enter", "esc"}},
		{"Delete hints", ViewDelete, []string{"y", "n", "space"}},
		{"Bulk hints", ViewBulk, []string{"space", "enter", "esc"}},
		{"PRs hints", ViewPRs, []string{"enter", "esc", "↑↓"}},
		{"Fork hints", ViewFork, []string{"enter", "esc"}},
		{"Sync hints", ViewSync, []string{"enter", "esc", "↑↓"}},
		{"Config hints", ViewConfig, []string{"enter", "esc", "tab"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hf := NewHelpFooter()
			hints := hf.CompactHints(tt.view)
			// Each expected key must appear in at least one hint
			for _, wantKey := range tt.wantKeys {
				found := false
				for _, hint := range hints {
					if hint.Key == wantKey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected key %q in hints for %v, got hints: %v", wantKey, tt.view, hints)
				}
			}
		})
	}
}

func TestCompactHintsDynamicLabel(t *testing.T) {
	t.Run("compact mode off shows compact label", func(t *testing.T) {
		hf := NewHelpFooter()
		hf.CompactMode = false
		hints := hf.CompactHints(ViewDashboard)
		for _, hint := range hints {
			if hint.Key == "v" {
				if hint.Description != "compact" {
					t.Errorf("expected 'compact' label when CompactMode=false, got %q", hint.Description)
				}
				return
			}
		}
		t.Error("expected 'v' key in dashboard hints")
	})

	t.Run("compact mode on shows detailed label", func(t *testing.T) {
		hf := NewHelpFooter()
		hf.CompactMode = true
		hints := hf.CompactHints(ViewDashboard)
		for _, hint := range hints {
			if hint.Key == "v" {
				if hint.Description != "detailed" {
					t.Errorf("expected 'detailed' label when CompactMode=true, got %q", hint.Description)
				}
				return
			}
		}
		t.Error("expected 'v' key in dashboard hints")
	})
}

func TestRenderExpandedDynamicLabel(t *testing.T) {
	t.Run("compact mode off shows compact in expanded help", func(t *testing.T) {
		hf := NewHelpFooter()
		hf.CompactMode = false
		hf.Expanded = true
		result := hf.RenderExpanded(80)
		if !strings.Contains(result, "compact") {
			t.Error("expected 'compact' in expanded help when CompactMode=false")
		}
		if strings.Contains(result, "detailed") {
			t.Error("should not contain 'detailed' when CompactMode=false")
		}
	})

	t.Run("compact mode on shows detailed in expanded help", func(t *testing.T) {
		hf := NewHelpFooter()
		hf.CompactMode = true
		hf.Expanded = true
		result := hf.RenderExpanded(80)
		if !strings.Contains(result, "detailed") {
			t.Error("expected 'detailed' in expanded help when CompactMode=true")
		}
	})
}

func TestHelpFooterRenderCompact(t *testing.T) {
	hf := NewHelpFooter()
	result := hf.RenderCompact(ViewDashboard, 200)

	if result == "" {
		t.Fatal("RenderCompact returned empty string")
	}

	// Should contain key hints as visible text
	if !strings.Contains(result, "navigate") {
		t.Error("expected 'navigate' in compact footer")
	}
	if !strings.Contains(result, "?") {
		t.Error("expected '?' key hint in compact footer")
	}
}

func TestHelpFooterRenderCompactTruncation(t *testing.T) {
	hf := NewHelpFooter()
	// Very narrow width should not panic
	result := hf.RenderCompact(ViewDashboard, 20)
	if result == "" {
		t.Fatal("RenderCompact returned empty at narrow width")
	}
}

func TestHelpFooterToggle(t *testing.T) {
	hf := NewHelpFooter()

	if hf.Expanded {
		t.Fatal("HelpFooter should start collapsed")
	}

	hf.Toggle()
	if !hf.Expanded {
		t.Error("expected Expanded=true after first toggle")
	}

	hf.Toggle()
	if hf.Expanded {
		t.Error("expected Expanded=false after second toggle")
	}
}

func TestHelpFooterRenderExpanded(t *testing.T) {
	hf := NewHelpFooter()
	hf.Expanded = true

	result := hf.RenderExpanded(80)

	if result == "" {
		t.Fatal("RenderExpanded returned empty string")
	}

	// Should contain three sections
	if !strings.Contains(result, "Navigation") {
		t.Error("expected 'Navigation' section in expanded help")
	}
	if !strings.Contains(result, "Actions") {
		t.Error("expected 'Actions' section in expanded help")
	}
	if !strings.Contains(result, "Views") {
		t.Error("expected 'Views' section in expanded help")
	}

	// Should contain close hint
	if !strings.Contains(result, "?") {
		t.Error("expected '?' close hint in expanded help")
	}
}

func TestHelpFooterRenderExpandedNarrow(t *testing.T) {
	hf := NewHelpFooter()
	hf.Expanded = true

	// Should not panic at narrow widths
	result := hf.RenderExpanded(40)
	if result == "" {
		t.Fatal("RenderExpanded returned empty at narrow width")
	}
}

func TestHintStruct(t *testing.T) {
	h := Hint{Key: "enter", Description: "switch"}
	if h.Key != "enter" || h.Description != "switch" {
		t.Errorf("Hint fields wrong: %+v", h)
	}
}
