package tui

import (
	"strings"
	"testing"
)

func TestRenderDetailV2_NilItem(t *testing.T) {
	got := renderDetailV2(nil, 60)
	if got != "" {
		t.Errorf("expected empty string for nil item, got %q", got)
	}
}

func TestRenderDetailV2_NarrowWidth(t *testing.T) {
	item := &WorktreeItem{ShortName: "test"}
	got := renderDetailV2(item, 10)
	if got != "" {
		t.Errorf("expected empty string for narrow width, got %q", got)
	}
}

func TestRenderDetailV2_ContainsTitle(t *testing.T) {
	item := &WorktreeItem{
		ShortName: "feature-auth",
		Branch:    "feature/auth",
		Commit:    "abc1234",
		CommitAge: "2 hours ago",
	}
	got := renderDetailV2(item, 60)
	if !strings.Contains(got, "feature-auth") {
		t.Errorf("expected detail to contain title %q, got:\n%s", "feature-auth", got)
	}
}

func TestRenderDetailV2_MetadataGrid(t *testing.T) {
	tests := []struct {
		name       string
		item       WorktreeItem
		wantLabels []string
	}{
		{
			name: "shows branch and commit",
			item: WorktreeItem{
				ShortName: "test",
				Branch:    "main",
				Commit:    "abc1234",
				CommitAge: "1 day ago",
			},
			wantLabels: []string{"Branch", "Commit", "Status"},
		},
		{
			name: "shows dirty status with file count",
			item: WorktreeItem{
				ShortName:  "test",
				Branch:     "main",
				Commit:     "abc1234",
				IsDirty:    true,
				DirtyFiles: []string{"M  foo.go", "?? bar.go", "D  baz.go"},
			},
			wantLabels: []string{"Status"},
		},
		{
			name: "shows tmux indicator when active",
			item: WorktreeItem{
				ShortName:  "test",
				Branch:     "main",
				TmuxStatus: "attached",
			},
			wantLabels: []string{"Tmux"},
		},
		{
			name: "no tmux row when no session",
			item: WorktreeItem{
				ShortName:  "test",
				Branch:     "main",
				TmuxStatus: "none",
			},
			wantLabels: []string{"Branch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderDetailV2(&tt.item, 60)
			for _, label := range tt.wantLabels {
				if !strings.Contains(got, label) {
					t.Errorf("expected detail to contain label %q, got:\n%s", label, got)
				}
			}
		})
	}
}

func TestRenderDetailV2_TmuxIndicator(t *testing.T) {
	tests := []struct {
		name       string
		tmuxStatus string
		wantText   string
		wantAbsent string
	}{
		{"attached session", "attached", "active", ""},
		{"detached session", "detached", "detached", ""},
		{"no session", "none", "", "Tmux"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &WorktreeItem{
				ShortName:  "test",
				Branch:     "main",
				TmuxStatus: tt.tmuxStatus,
			}
			got := renderDetailV2(item, 60)
			if tt.wantText != "" && !strings.Contains(got, tt.wantText) {
				t.Errorf("expected %q in output, got:\n%s", tt.wantText, got)
			}
			if tt.wantAbsent != "" && strings.Contains(got, tt.wantAbsent) {
				t.Errorf("did not expect %q in output, got:\n%s", tt.wantAbsent, got)
			}
		})
	}
}

func TestRenderDetailV2_ChangedFiles(t *testing.T) {
	item := &WorktreeItem{
		ShortName:  "test",
		Branch:     "main",
		IsDirty:    true,
		DirtyFiles: []string{"M  foo.go", "?? bar.go", "D  baz.go"},
	}
	got := renderDetailV2(item, 60)

	// Should contain a changes section header
	if !strings.Contains(got, "Changes") {
		t.Errorf("expected 'Changes' section header, got:\n%s", got)
	}

	// Should contain file names
	for _, f := range []string{"foo.go", "bar.go", "baz.go"} {
		if !strings.Contains(got, f) {
			t.Errorf("expected file %q in output, got:\n%s", f, got)
		}
	}
}

func TestRenderDetailV2_NoChangesSection(t *testing.T) {
	item := &WorktreeItem{
		ShortName: "test",
		Branch:    "main",
		IsDirty:   false,
	}
	got := renderDetailV2(item, 60)
	if strings.Contains(got, "Changes") {
		t.Errorf("did not expect 'Changes' section for clean worktree, got:\n%s", got)
	}
}

func TestRenderDetailV2_StatusText(t *testing.T) {
	tests := []struct {
		name     string
		item     WorktreeItem
		wantText string
	}{
		{
			name:     "clean worktree",
			item:     WorktreeItem{ShortName: "t", Branch: "main"},
			wantText: "clean",
		},
		{
			name:     "dirty worktree shows file count",
			item:     WorktreeItem{ShortName: "t", Branch: "main", IsDirty: true, DirtyFiles: []string{"M foo", "M bar"}},
			wantText: "dirty",
		},
		{
			name:     "stale worktree",
			item:     WorktreeItem{ShortName: "t", Branch: "main", IsPrunable: true},
			wantText: "stale",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderDetailV2(&tt.item, 60)
			if !strings.Contains(got, tt.wantText) {
				t.Errorf("expected %q in output, got:\n%s", tt.wantText, got)
			}
		})
	}
}

func TestRenderDetailV2_BorderPresent(t *testing.T) {
	item := &WorktreeItem{
		ShortName: "feature-auth",
		Branch:    "feature/auth",
		Commit:    "abc1234",
	}
	got := renderDetailV2(item, 60)
	// Rounded border uses ╭ and ╰ characters
	if !strings.Contains(got, "╭") || !strings.Contains(got, "╰") {
		t.Errorf("expected bordered card with rounded corners, got:\n%s", got)
	}
}

func TestRenderDetailV2_SyncStatus(t *testing.T) {
	tests := []struct {
		name     string
		item     WorktreeItem
		wantText string
	}{
		{
			name:     "ahead commits",
			item:     WorktreeItem{ShortName: "t", Branch: "main", HasRemote: true, AheadCount: 3},
			wantText: "↑3",
		},
		{
			name:     "behind commits",
			item:     WorktreeItem{ShortName: "t", Branch: "main", HasRemote: true, BehindCount: 2},
			wantText: "↓2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderDetailV2(&tt.item, 60)
			if !strings.Contains(got, tt.wantText) {
				t.Errorf("expected %q in output, got:\n%s", tt.wantText, got)
			}
		})
	}
}
