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
		{"Dashboard hints", ViewDashboard, []string{"↑↓", "enter", "U", "n", "/", "o", "?", "q"}},
		{"Issues hints", ViewIssues, []string{"↑↓", "enter", "/", "esc"}},
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

func TestHintStruct(t *testing.T) {
	h := Hint{Key: "enter", Description: "switch"}
	if h.Key != "enter" || h.Description != "switch" {
		t.Errorf("Hint fields wrong: %+v", h)
	}
}
