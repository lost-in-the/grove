package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestDelegateColumnsV2_NarrowTerminal(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		wantName  int
		wantSmall bool // expect smaller columns than default
	}{
		{"very narrow 40 chars", 40, 0, true},
		{"narrow 60 chars", 60, 0, true},
		{"default 80 chars", 80, 16, false},
		{"medium 100 chars", 100, 20, false},
		{"wide 130 chars", 130, 24, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols := delegateColumnsV2(tt.width)
			if tt.wantSmall {
				if cols.Name >= 16 {
					t.Errorf("width=%d: expected Name < 16, got %d", tt.width, cols.Name)
				}
			}
			if tt.wantName > 0 && cols.Name != tt.wantName {
				t.Errorf("width=%d: expected Name=%d, got %d", tt.width, tt.wantName, cols.Name)
			}
		})
	}
}

func TestNarrowLayoutBreakpoints(t *testing.T) {
	tests := []struct {
		name          string
		width         int
		wantSideBySide bool
		wantCompact   bool
	}{
		{"very narrow", 40, false, true},
		{"narrow", 60, false, true},
		{"medium stacked", 80, false, false},
		{"medium stacked 100", 100, false, false},
		{"wide side-by-side", 130, true, false},
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
			h := Header{
				ProjectName:   "my-long-project-name",
				WorktreeCount: 5,
				CurrentBranch: "feature/very-long-branch-name-here",
				CurrentName:   "fix/another-long-name",
			}
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

func TestNarrowDelegateHidesBranchColumn(t *testing.T) {
	cols := delegateColumnsV2(40)
	// At very narrow widths, branch column should be 0 (hidden)
	if cols.Branch > 0 {
		t.Errorf("at width=40, expected Branch=0 (hidden), got %d", cols.Branch)
	}
}

func TestNarrowDelegateHidesAge(t *testing.T) {
	cols := delegateColumnsV2(40)
	if cols.Age > 0 {
		t.Errorf("at width=40, expected Age=0 (hidden), got %d", cols.Age)
	}
}

func TestDelegateColumnsV2_NarrowShowsBranch(t *testing.T) {
	cols := delegateColumnsV2(60)
	if cols.Branch == 0 {
		t.Error("at width=60, expected Branch > 0")
	}
	if cols.Name == 0 {
		t.Error("at width=60, expected Name > 0")
	}
}
