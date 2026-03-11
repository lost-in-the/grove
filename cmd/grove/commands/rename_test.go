package commands

import (
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/worktree"
)

func TestRenameCmd(t *testing.T) {
	if renameCmd == nil {
		t.Fatal("renameCmd is nil")
	}

	if renameCmd.Use != "rename <old> <new>" {
		t.Errorf("renameCmd.Use = %v, want 'rename <old> <new>'", renameCmd.Use)
	}

	if renameCmd.RunE == nil {
		t.Error("renameCmd.RunE is nil")
	}
}

func TestRenameCmdArgs(t *testing.T) {
	// Should require exactly 2 args
	if renameCmd.Args == nil {
		t.Fatal("renameCmd.Args is nil")
	}

	// Verify it errors with wrong number of args
	err := renameCmd.Args(renameCmd, []string{"only-one"})
	if err == nil {
		t.Error("should error with only 1 arg")
	}

	err = renameCmd.Args(renameCmd, []string{"one", "two"})
	if err != nil {
		t.Errorf("should accept 2 args, got error: %v", err)
	}

	err = renameCmd.Args(renameCmd, []string{"one", "two", "three"})
	if err == nil {
		t.Error("should error with 3 args")
	}
}

func TestRenameCmdHelp(t *testing.T) {
	long := renameCmd.Long
	if long == "" {
		t.Fatal("renameCmd.Long is empty")
	}

	required := []struct {
		label string
		text  string
	}{
		{"directory", "directory"},
		{"tmux session", "tmux session"},
		{"protected", "Protected"},
	}

	for _, tt := range required {
		if !strings.Contains(long, tt.text) {
			t.Errorf("renameCmd.Long should mention %s (missing %q)", tt.label, tt.text)
		}
	}
}

func TestRenameCmdRegistered(t *testing.T) {
	// Verify the rename command is registered on the root command
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "rename" {
			found = true
			break
		}
	}
	if !found {
		t.Error("rename command not registered on root")
	}
}

// mockProtection implements the IsProtected interface for testing.
type mockProtection struct {
	protected map[string]bool
}

func (m *mockProtection) IsProtected(name string) bool {
	return m.protected[name]
}

func TestValidateRename(t *testing.T) {
	normalWT := &worktree.Worktree{ShortName: "feature", Path: "/repo/feature", IsMain: false}
	mainWT := &worktree.Worktree{ShortName: "root", Path: "/repo/main", IsMain: true}
	otherWT := &worktree.Worktree{ShortName: "other", Path: "/repo/other", IsMain: false}
	noCfg := &mockProtection{protected: map[string]bool{}}

	tests := []struct {
		name      string
		wt        *worktree.Worktree
		existing  *worktree.Worktree
		current   *worktree.Worktree
		cfg       *mockProtection
		oldName   string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "valid rename",
			wt:      normalWT,
			current: otherWT,
			cfg:     noCfg,
			oldName: "feature",
			wantErr: false,
		},
		{
			name:      "cannot rename main worktree",
			wt:        mainWT,
			current:   otherWT,
			cfg:       noCfg,
			oldName:   "root",
			wantErr:   true,
			errSubstr: "cannot rename the main worktree",
		},
		{
			name:      "cannot rename protected worktree",
			wt:        normalWT,
			current:   otherWT,
			cfg:       &mockProtection{protected: map[string]bool{"feature": true}},
			oldName:   "feature",
			wantErr:   true,
			errSubstr: "protected",
		},
		{
			name:      "cannot rename to existing name",
			wt:        normalWT,
			existing:  otherWT,
			current:   nil,
			cfg:       noCfg,
			oldName:   "feature",
			wantErr:   true,
			errSubstr: "already exists",
		},
		{
			name:      "cannot rename current worktree",
			wt:        normalWT,
			current:   normalWT, // same path = current
			cfg:       noCfg,
			oldName:   "feature",
			wantErr:   true,
			errSubstr: "cannot rename current worktree",
		},
		{
			name:    "nil current worktree is fine",
			wt:      normalWT,
			current: nil,
			cfg:     noCfg,
			oldName: "feature",
			wantErr: false,
		},
		{
			name:    "nil config is fine",
			wt:      normalWT,
			current: otherWT,
			cfg:     nil,
			oldName: "feature",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg interface{ IsProtected(string) bool }
			if tt.cfg != nil {
				cfg = tt.cfg
			}
			err := validateRename(tt.wt, tt.existing, tt.current, cfg, tt.oldName)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errSubstr)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
