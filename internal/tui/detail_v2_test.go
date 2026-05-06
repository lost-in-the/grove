package tui

import (
	"fmt"
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

func TestRenderDetailV2_CommitCount(t *testing.T) {
	tests := []struct {
		name       string
		item       WorktreeItem
		wantText   string
		wantAbsent string
	}{
		{
			name:     "shows commits ahead",
			item:     WorktreeItem{ShortName: "t", Branch: "feat", Commit: "abc1234", CommitCount: 5},
			wantText: "5 commits",
		},
		{
			name:       "hides when zero",
			item:       WorktreeItem{ShortName: "t", Branch: "main", Commit: "abc1234", CommitCount: 0},
			wantAbsent: "Ahead",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderDetailV2(&tt.item, 60)
			if tt.wantText != "" && !strings.Contains(got, tt.wantText) {
				t.Errorf("expected %q in output, got:\n%s", tt.wantText, got)
			}
			if tt.wantAbsent != "" && strings.Contains(got, tt.wantAbsent) {
				t.Errorf("did not expect %q in output, got:\n%s", tt.wantAbsent, got)
			}
		})
	}
}

func TestRenderDetailV2_RecentCommits(t *testing.T) {
	item := &WorktreeItem{
		ShortName: "test",
		Branch:    "main",
		RecentCommits: []RecentCommit{
			{SHA: "abc1234", Message: "fix: resolve bug"},
			{SHA: "def5678", Message: "feat: add login"},
			{SHA: "ghi9012", Message: "chore: update deps"},
		},
	}
	got := renderDetailV2(item, 60)

	if !strings.Contains(got, "Recent") {
		t.Errorf("expected 'Recent' section header, got:\n%s", got)
	}
	for _, c := range item.RecentCommits {
		if !strings.Contains(got, c.SHA) {
			t.Errorf("expected commit SHA %q in output, got:\n%s", c.SHA, got)
		}
		if !strings.Contains(got, c.Message) {
			t.Errorf("expected commit message %q in output, got:\n%s", c.Message, got)
		}
	}
}

func TestRenderDetailV2_NoRecentCommitsWhenEmpty(t *testing.T) {
	item := &WorktreeItem{
		ShortName: "test",
		Branch:    "main",
	}
	got := renderDetailV2(item, 60)
	if strings.Contains(got, "Recent") {
		t.Errorf("did not expect 'Recent' section for empty commits, got:\n%s", got)
	}
}

func TestRenderDetailV2_StashCount(t *testing.T) {
	tests := []struct {
		name       string
		stashCount int
		wantText   string
		wantAbsent string
	}{
		{"shows stash count", 3, "3 stashed", ""},
		{"hides when zero", 0, "", "Stash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &WorktreeItem{ShortName: "t", Branch: "main", StashCount: tt.stashCount}
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

func TestRenderDetailV2_AssociatedPR(t *testing.T) {
	tests := []struct {
		name     string
		pr       *PRInfo
		wantText []string
	}{
		{
			name: "shows PR with review decision",
			pr: &PRInfo{
				Number:         42,
				Title:          "Add authentication",
				ReviewDecision: "APPROVED",
			},
			wantText: []string{"PR", "#42", "Add authentication", "Approved"},
		},
		{
			name: "shows PR with changes requested",
			pr: &PRInfo{
				Number:         99,
				Title:          "Refactor API",
				ReviewDecision: "CHANGES_REQUESTED",
			},
			wantText: []string{"#99", "Refactor API", "Changes requested"},
		},
		{
			name: "shows PR without review decision",
			pr: &PRInfo{
				Number: 10,
				Title:  "WIP feature",
			},
			wantText: []string{"#10", "WIP feature"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &WorktreeItem{ShortName: "t", Branch: "feat", AssociatedPR: tt.pr}
			got := renderDetailV2(item, 60)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("expected %q in output, got:\n%s", want, got)
				}
			}
		})
	}
}

func TestRenderDetailV2_NoPRSection(t *testing.T) {
	item := &WorktreeItem{ShortName: "t", Branch: "main"}
	got := renderDetailV2(item, 60)
	if strings.Contains(got, " PR ") {
		t.Errorf("did not expect PR section without associated PR, got:\n%s", got)
	}
}

func TestRenderDetailV2_TrackingBranch(t *testing.T) {
	item := &WorktreeItem{
		ShortName:      "test",
		Branch:         "feat/auth",
		TrackingBranch: "origin/feat/foo",
	}
	got := renderDetailV2(item, 60)
	if !strings.Contains(got, "origin/feat/foo") {
		t.Errorf("expected 'origin/feat/foo' in output, got:\n%s", got)
	}
}

func TestRenderDetailV2_NoTracking(t *testing.T) {
	item := &WorktreeItem{
		ShortName:      "test",
		Branch:         "feat/auth",
		IsMain:         false,
		HasRemote:      false,
		TrackingBranch: "",
	}
	got := renderDetailV2(item, 60)
	if !strings.Contains(got, "not tracking") {
		t.Errorf("expected 'not tracking' in output, got:\n%s", got)
	}
}

func TestRenderDetailV2_UnpushedSync(t *testing.T) {
	item := &WorktreeItem{
		ShortName:  "test",
		Branch:     "feat/auth",
		HasRemote:  true,
		AheadCount: 3,
	}
	got := renderDetailV2(item, 60)
	if !strings.Contains(got, "unpushed") {
		t.Errorf("expected 'unpushed' in output, got:\n%s", got)
	}
}

func TestRenderDetailV2_FullChanges(t *testing.T) {
	var files []string
	for i := 0; i < 25; i++ {
		files = append(files, fmt.Sprintf("M  file%d.go", i))
	}
	item := &WorktreeItem{
		ShortName:  "test",
		Branch:     "main",
		IsDirty:    true,
		DirtyFiles: files,
	}
	got := renderDetailV2(item, 80)
	for i := 0; i < 25; i++ {
		name := fmt.Sprintf("file%d.go", i)
		if !strings.Contains(got, name) {
			t.Errorf("expected %q in output (no truncation), got:\n%s", name, got)
		}
	}
}

func TestRenderDetailV2_CommitSHAStyle(t *testing.T) {
	item := &WorktreeItem{
		ShortName: "test",
		Branch:    "main",
		RecentCommits: []RecentCommit{
			{SHA: "abc1234", Message: "test msg"},
		},
	}
	got := renderDetailV2(item, 60)
	if !strings.Contains(got, "abc1234") {
		t.Errorf("expected commit SHA 'abc1234' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "test msg") {
		t.Errorf("expected commit message 'test msg' in output, got:\n%s", got)
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
