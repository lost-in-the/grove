package tui

import (
	"testing"
)

func TestCheckDuplicateWorktree(t *testing.T) {
	existing := []WorktreeItem{
		{ShortName: "feature-auth", Path: "/home/user/proj-feature-auth", Branch: "feature/authentication", IsDirty: true, DirtyFiles: []string{"main.go", "auth.go"}},
		{ShortName: "main", Path: "/home/user/proj", Branch: "main"},
		{ShortName: "testing", Path: "/home/user/proj-testing", Branch: "testing"},
	}

	tests := []struct {
		name      string
		input     string
		items     []WorktreeItem
		wantMatch bool
	}{
		{"exact match found", "feature-auth", existing, true},
		{"no match", "new-feature", existing, false},
		{"empty input", "", existing, false},
		{"case sensitive no match", "Feature-Auth", existing, true},
		{"matches main", "main", existing, true},
		{"empty items list", "anything", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkDuplicateWorktree(tt.input, tt.items)
			if tt.wantMatch && got == nil {
				t.Errorf("checkDuplicateWorktree(%q) = nil, want match", tt.input)
			}
			if !tt.wantMatch && got != nil {
				t.Errorf("checkDuplicateWorktree(%q) = %+v, want nil", tt.input, got)
			}
		})
	}
}

func TestCheckDuplicateWorktreeReturnsCorrectItem(t *testing.T) {
	existing := []WorktreeItem{
		{ShortName: "feature-auth", Path: "/home/user/proj-feature-auth", Branch: "feature/authentication", IsDirty: true, DirtyFiles: []string{"main.go", "auth.go"}},
	}

	got := checkDuplicateWorktree("feature-auth", existing)
	if got == nil {
		t.Fatal("expected match, got nil")
	}
	if got.ShortName != "feature-auth" {
		t.Errorf("ShortName = %q, want %q", got.ShortName, "feature-auth")
	}
	if got.Branch != "feature/authentication" {
		t.Errorf("Branch = %q, want %q", got.Branch, "feature/authentication")
	}
	if !got.IsDirty {
		t.Error("expected IsDirty = true")
	}
}
