package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
)

func TestComputeDelegateWidthsV2_NarrowTerminal(t *testing.T) {
	items := []list.Item{
		WorktreeItem{ShortName: "my-feature-branch", Branch: "feature/my-feature"},
		WorktreeItem{ShortName: "main", Branch: "main"},
	}

	tests := []struct {
		name          string
		width         int
		wantDirHidden bool // expect directory (NameWidth) hidden at very narrow
	}{
		{"very narrow 40 chars", 40, true},
		{"narrow 55 chars", 55, true},
		{"default 80 chars", 80, false},
		{"medium 100 chars", 100, false},
		{"wide 130 chars", 130, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := ComputeDelegateWidthsV2(items, tt.width)
			// Branch is always shown (primary identifier)
			if d.BranchWidth == 0 {
				t.Errorf("width=%d: expected BranchWidth > 0, got 0", tt.width)
			}
			// Directory is hidden at narrow widths
			if tt.wantDirHidden && d.NameWidth > 0 {
				t.Errorf("width=%d: expected NameWidth=0 (hidden), got %d", tt.width, d.NameWidth)
			}
			if !tt.wantDirHidden && d.NameWidth == 0 {
				t.Errorf("width=%d: expected NameWidth > 0, got 0", tt.width)
			}
		})
	}
}

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

func TestComputeDelegateWidthsV2_HidesDirectoryNarrow(t *testing.T) {
	items := []list.Item{
		WorktreeItem{ShortName: "feature", Branch: "feature/test"},
	}
	d := ComputeDelegateWidthsV2(items, 40)
	if d.NameWidth > 0 {
		t.Errorf("at width=40, expected NameWidth=0 (hidden), got %d", d.NameWidth)
	}
	if d.BranchWidth == 0 {
		t.Errorf("at width=40, expected BranchWidth > 0 (always shown), got 0")
	}
}

func TestComputeDelegateWidthsV2_ShowsBothAtWidth60(t *testing.T) {
	items := []list.Item{
		WorktreeItem{ShortName: "feature", Branch: "feature/test"},
	}
	d := ComputeDelegateWidthsV2(items, 60)
	if d.BranchWidth == 0 {
		t.Error("at width=60, expected BranchWidth > 0")
	}
	if d.NameWidth == 0 {
		t.Error("at width=60, expected NameWidth > 0")
	}
}
