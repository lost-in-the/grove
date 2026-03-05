package tui

import (
	"strings"
	"testing"
)

func TestRenderDeleteV2_CleanWorktree(t *testing.T) {
	s := &DeleteState{
		Item: &WorktreeItem{
			ShortName: "feature-auth",
			FullName:  "myapp-feature-auth",
			Path:      "/home/dev/projects/myapp-feature-auth",
			Branch:    "feature/authentication",
			Commit:    "87f14f7",
			CommitAge: "2 hours",
		},
		Warnings: nil,
	}

	view := renderDeleteV2(s, 70)

	// Title present
	if !strings.Contains(view, "Delete Worktree") {
		t.Error("expected overlay title 'Delete Worktree'")
	}

	// Details section present
	if !strings.Contains(view, "feature-auth") {
		t.Error("expected worktree name in view")
	}
	if !strings.Contains(view, "feature/authentication") {
		t.Error("expected branch in details")
	}
	if !strings.Contains(view, "87f14f7") {
		t.Error("expected commit hash in details")
	}

	// No warning box for clean worktree
	if strings.Contains(view, "⚠") {
		t.Error("clean worktree should not show warning icon")
	}

	// Checkbox present
	if !strings.Contains(view, "Also delete branch") {
		t.Error("expected branch deletion checkbox")
	}

	// Footer present
	if !strings.Contains(view, "confirm") {
		t.Error("expected confirm action in footer")
	}
}

func TestRenderDeleteV2_DirtyWorktree(t *testing.T) {
	s := &DeleteState{
		Item: &WorktreeItem{
			ShortName:  "feature-auth",
			FullName:   "myapp-feature-auth",
			Path:       "/home/dev/projects/myapp-feature-auth",
			Branch:     "feature/authentication",
			Commit:     "87f14f7",
			CommitAge:  "2 hours",
			IsDirty:    true,
			DirtyFiles: []string{"M  src/auth.go", "M  src/main.go", "?? tmp.log"},
		},
		Warnings: []string{"Working tree is dirty"},
	}

	view := renderDeleteV2(s, 70)

	// Warning box present for dirty worktrees
	if !strings.Contains(view, "⚠") {
		t.Error("dirty worktree should show warning")
	}
	if !strings.Contains(view, "3") {
		t.Error("expected dirty file count")
	}
}

func TestRenderDeleteV2_CheckboxToggle(t *testing.T) {
	tests := []struct {
		name         string
		deleteBranch bool
		wantChecked  string
		wantEmpty    string
	}{
		{"unchecked", false, "[ ]", "[x]"},
		{"checked", true, "[x]", "[ ]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &DeleteState{
				Item: &WorktreeItem{
					ShortName: "test",
					Branch:    "test-branch",
				},
				DeleteBranch: tt.deleteBranch,
			}
			view := renderDeleteV2(s, 60)
			if !strings.Contains(view, tt.wantChecked) {
				t.Errorf("expected %s in view", tt.wantChecked)
			}
		})
	}
}

func TestRenderDeleteV2_ProtectedWarning(t *testing.T) {
	s := &DeleteState{
		Item: &WorktreeItem{
			ShortName:   "staging",
			Branch:      "staging",
			IsProtected: true,
		},
		Warnings: []string{"This worktree is protected"},
	}

	view := renderDeleteV2(s, 60)
	if !strings.Contains(view, "protected") {
		t.Error("expected protected warning")
	}
}

func TestRenderDeleteV2_EnvironmentWarning(t *testing.T) {
	s := &DeleteState{
		Item: &WorktreeItem{
			ShortName:     "production",
			Branch:        "production",
			IsEnvironment: true,
		},
		Warnings: []string{"This is an environment worktree"},
	}

	view := renderDeleteV2(s, 60)
	if !strings.Contains(view, "environment") {
		t.Error("expected environment warning")
	}
}

func TestRenderDeleteV2_NarrowWidth(t *testing.T) {
	s := &DeleteState{
		Item: &WorktreeItem{
			ShortName: "feature-auth",
			Path:      "/home/dev/projects/myapp-feature-auth",
			Branch:    "feature/authentication",
			Commit:    "87f14f7",
		},
	}

	// Should not panic at narrow widths
	view := renderDeleteV2(s, 40)
	if view == "" {
		t.Error("expected non-empty view at narrow width")
	}
}

func TestRenderDelete_DeletingState(t *testing.T) {
	s := &DeleteState{
		Item: &WorktreeItem{
			ShortName: "feature-auth",
			Branch:    "feature/authentication",
		},
		Deleting: true,
	}

	view := renderDelete(s, 70)

	if !strings.Contains(view, "Deleting") {
		t.Error("expected 'Deleting' text in V1 view during deletion")
	}
	if !strings.Contains(view, "feature-auth") {
		t.Error("expected worktree name in V1 deleting view")
	}
	// Should NOT contain confirmation UI
	if strings.Contains(view, "confirm") {
		t.Error("should not show confirmation UI while deleting")
	}
	if strings.Contains(view, "cancel") {
		t.Error("should not show cancel option while deleting")
	}
}

func TestRenderDeleteV2_DeletingState(t *testing.T) {
	s := &DeleteState{
		Item: &WorktreeItem{
			ShortName: "feature-auth",
			Branch:    "feature/authentication",
		},
		Deleting: true,
	}

	view := renderDeleteV2(s, 70)

	if !strings.Contains(view, "Deleting") {
		t.Error("expected 'Deleting' text in view during deletion")
	}
	if !strings.Contains(view, "feature-auth") {
		t.Error("expected worktree name in deleting view")
	}
	if !strings.Contains(view, "Please wait") {
		t.Error("expected 'Please wait' in footer during deletion")
	}
	// Should NOT contain confirmation UI
	if strings.Contains(view, "confirm") {
		t.Error("should not show confirmation UI while deleting")
	}
	if strings.Contains(view, "toggle") {
		t.Error("should not show toggle option while deleting")
	}
}

func TestRenderDeleteV2_FooterActions(t *testing.T) {
	s := &DeleteState{
		Item: &WorktreeItem{
			ShortName: "test",
			Branch:    "test-branch",
		},
	}

	view := renderDeleteV2(s, 70)

	// All footer actions present
	if !strings.Contains(view, "y") {
		t.Error("expected 'y' key in footer")
	}
	if !strings.Contains(view, "n") || !strings.Contains(view, "cancel") {
		t.Error("expected 'n' cancel in footer")
	}
	if !strings.Contains(view, "space") || !strings.Contains(view, "toggle") {
		t.Error("expected space toggle in footer")
	}
}
