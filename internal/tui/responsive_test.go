package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestNarrowLayoutBreakpoints(t *testing.T) {
	tests := []struct {
		name           string
		width          int
		wantSideBySide bool
		wantCompact    bool
	}{
		{"very narrow", 40, false, true},
		{"narrow", 60, false, true},
		{"medium stacked", 80, false, false},
		{"medium stacked 100", 100, false, false},
		{"wide side-by-side", 110, true, false},
		{"wide side-by-side 130", 130, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := LayoutBreakpoint(tt.width)
			if tt.wantSideBySide && bp != LayoutWide {
				t.Errorf("width=%d: expected LayoutWide, got %v", tt.width, bp)
			}
			if tt.wantCompact && bp != LayoutNarrow {
				t.Errorf("width=%d: expected LayoutNarrow, got %v", tt.width, bp)
			}
			if !tt.wantSideBySide && !tt.wantCompact && bp != LayoutMedium {
				t.Errorf("width=%d: expected LayoutMedium, got %v", tt.width, bp)
			}
		})
	}
}

func TestHeaderViewNoOverflow(t *testing.T) {
	tests := []struct {
		name  string
		width int
	}{
		{"40 chars", 40},
		{"60 chars", 60},
		{"80 chars", 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Header{ProjectName: "my-long-project-name", WorktreeCount: 5, SortLabel: "dirty", SortReversed: true, Cursor: 12, Total: 15}
			view := h.View(tt.width)
			lines := strings.Split(view, "\n")
			for i, line := range lines {
				w := lipgloss.Width(line)
				if w > tt.width {
					t.Errorf("line %d overflows: width=%d > %d, content=%q", i, w, tt.width, line)
				}
			}
		})
	}
}

func TestRenderDetailV2_NarrowNoOverflow(t *testing.T) {
	item := &WorktreeItem{
		ShortName:  "my-feature",
		Branch:     "feature/very-long-branch-name",
		Commit:     "abc1234",
		CommitAge:  "2 hours ago",
		IsDirty:    true,
		DirtyFiles: []string{"M  README.md", "?? newfile.txt"},
		HasRemote:  true,
	}

	tests := []struct {
		name  string
		width int
	}{
		{"very narrow 30", 30},
		{"narrow 50", 50},
		{"default 60", 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := renderDetailV2(item, tt.width)
			lines := strings.Split(view, "\n")
			for i, line := range lines {
				w := lipgloss.Width(line)
				if w > tt.width {
					t.Errorf("line %d overflows: width=%d > %d", i, w, tt.width)
				}
			}
		})
	}
}
