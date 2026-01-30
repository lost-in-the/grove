package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
)

func TestWorktreeDelegateV2_Indicators(t *testing.T) {
	tests := []struct {
		name          string
		item          WorktreeItem
		selected      bool
		wantIndicator string
	}{
		{
			name:          "Current worktree shows green dot",
			item:          WorktreeItem{IsCurrent: true, ShortName: "main", Branch: "main"},
			selected:      false,
			wantIndicator: "●",
		},
		{
			name:          "Dirty worktree shows yellow dot",
			item:          WorktreeItem{IsDirty: true, ShortName: "feature", Branch: "feat"},
			selected:      false,
			wantIndicator: "●",
		},
		{
			name:          "Selected shows cursor",
			item:          WorktreeItem{ShortName: "test", Branch: "test"},
			selected:      true,
			wantIndicator: "❯",
		},
		{
			name:          "Normal shows space",
			item:          WorktreeItem{ShortName: "other", Branch: "other"},
			selected:      false,
			wantIndicator: " ",
		},
		{
			name:          "Current and selected shows green dot",
			item:          WorktreeItem{IsCurrent: true, ShortName: "main", Branch: "main"},
			selected:      true,
			wantIndicator: "●",
		},
		{
			name:          "Dirty and selected shows yellow dot",
			item:          WorktreeItem{IsDirty: true, ShortName: "feature", Branch: "feat"},
			selected:      true,
			wantIndicator: "●",
		},
		{
			name:          "Stale worktree shows stale indicator",
			item:          WorktreeItem{IsPrunable: true, ShortName: "old", Branch: "old"},
			selected:      false,
			wantIndicator: "✗",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indicator := worktreeIndicator(tt.item, tt.selected)
			// The indicator string contains styling, but must include the expected rune
			if !strings.Contains(indicator, tt.wantIndicator) {
				t.Errorf("worktreeIndicator() = %q, want to contain %q", indicator, tt.wantIndicator)
			}
		})
	}
}

func TestWorktreeDelegateV2_StatusText(t *testing.T) {
	tests := []struct {
		name string
		item WorktreeItem
		want string
	}{
		{"Clean", WorktreeItem{}, "clean"},
		{"Dirty with files", WorktreeItem{IsDirty: true, DirtyFiles: []string{"a.go", "b.go"}}, "dirty"},
		{"Stale", WorktreeItem{IsPrunable: true}, "stale"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := worktreeStatusTextV2(tt.item)
			if !strings.Contains(text, tt.want) {
				t.Errorf("worktreeStatusTextV2() = %q, want to contain %q", text, tt.want)
			}
		})
	}
}

func TestWorktreeDelegateV2_TmuxBadge(t *testing.T) {
	tests := []struct {
		name string
		item WorktreeItem
		want string
	}{
		{"No tmux", WorktreeItem{TmuxStatus: "none"}, ""},
		{"Attached", WorktreeItem{TmuxStatus: "attached"}, "tmux"},
		{"Detached", WorktreeItem{TmuxStatus: "detached"}, "tmux"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			badge := worktreeTmuxBadgeV2(tt.item)
			if tt.want == "" && badge != "" {
				t.Errorf("worktreeTmuxBadgeV2() = %q, want empty", badge)
			}
			if tt.want != "" && !strings.Contains(badge, tt.want) {
				t.Errorf("worktreeTmuxBadgeV2() = %q, want to contain %q", badge, tt.want)
			}
		})
	}
}

func TestWorktreeDelegateV2_ResponsiveColumns(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		wantNameMin int
		wantNameMax int
	}{
		{"Narrow (80)", 80, 14, 18},
		{"Medium (100)", 100, 18, 22},
		{"Wide (140)", 140, 22, 28},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols := delegateColumnsV2(tt.width)
			if cols.Name < tt.wantNameMin || cols.Name > tt.wantNameMax {
				t.Errorf("delegateColumnsV2(%d).Name = %d, want in [%d, %d]",
					tt.width, cols.Name, tt.wantNameMin, tt.wantNameMax)
			}
		})
	}
}

func TestWorktreeDelegateV2_Height(t *testing.T) {
	d := NewWorktreeDelegateV2()
	if d.Height() != 1 {
		t.Errorf("Height() = %d, want 1", d.Height())
	}
}

func TestWorktreeDelegateV2_Spacing(t *testing.T) {
	d := NewWorktreeDelegateV2()
	if d.Spacing() != 0 {
		t.Errorf("Spacing() = %d, want 0", d.Spacing())
	}
}

func TestWorktreeDelegateV2_Update(t *testing.T) {
	d := NewWorktreeDelegateV2()
	l := list.New(nil, d, 80, 20)
	cmd := d.Update(nil, &l)
	if cmd != nil {
		t.Errorf("Update() returned non-nil cmd")
	}
}
