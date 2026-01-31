package tui

import (
	"strings"
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

func TestRenderCreateNameV2WithDuplicate(t *testing.T) {
	tests := []struct {
		name       string
		state      *CreateState
		wantStrs   []string
		dontWant   []string
	}{
		{
			name: "duplicate found shows warning and details",
			state: &CreateState{
				Step:        CreateStepName,
				Name:        "feature-auth",
				ProjectName: "acupoll",
				ExistingWorktree: &WorktreeItem{
					ShortName: "feature-auth",
					Path:      "/home/user/acupoll-feature-auth",
					Branch:    "feature/authentication",
					IsDirty:   true,
					DirtyFiles: []string{"main.go", "auth.go"},
				},
			},
			wantStrs: []string{
				"already exists",
				"feature/authentication",
				"/home/user/acupoll-feature-auth",
			},
			dontWant: []string{
				"✓ valid name",
			},
		},
		{
			name: "no duplicate shows valid name",
			state: &CreateState{
				Step:             CreateStepName,
				Name:             "new-feature",
				ProjectName:      "acupoll",
				ExistingWorktree: nil,
			},
			wantStrs: []string{
				"✓ valid name",
			},
			dontWant: []string{
				"already exists",
			},
		},
		{
			name: "duplicate with clean worktree",
			state: &CreateState{
				Step:        CreateStepName,
				Name:        "testing",
				ProjectName: "acupoll",
				ExistingWorktree: &WorktreeItem{
					ShortName: "testing",
					Path:      "/home/user/acupoll-testing",
					Branch:    "testing",
					IsDirty:   false,
				},
			},
			wantStrs: []string{
				"already exists",
				"testing",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderCreateNameV2(tt.state, 80)
			for _, want := range tt.wantStrs {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q in output:\n%s", want, got)
				}
			}
			for _, dontWant := range tt.dontWant {
				if strings.Contains(got, dontWant) {
					t.Errorf("should not contain %q in output:\n%s", dontWant, got)
				}
			}
		})
	}
}

func TestRenderDuplicateFooterHints(t *testing.T) {
	state := &CreateState{
		Step:        CreateStepName,
		Name:        "feature-auth",
		ProjectName: "acupoll",
		ExistingWorktree: &WorktreeItem{
			ShortName: "feature-auth",
		},
	}

	got := renderCreateNameV2(state, 80)
	// When duplicate exists, footer should offer switching
	if !strings.Contains(got, "switch") && !strings.Contains(got, "Switch") {
		t.Errorf("duplicate footer should mention switching, got:\n%s", got)
	}
}
