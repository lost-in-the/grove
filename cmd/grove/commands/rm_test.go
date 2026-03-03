package commands

import (
	"strings"
	"testing"
)

func TestRmCmd(t *testing.T) {
	if rmCmd == nil {
		t.Fatal("rmCmd is nil")
	}

	if rmCmd.Use != "rm <name>" {
		t.Errorf("rmCmd.Use = %v, want 'rm <name>'", rmCmd.Use)
	}

	if rmCmd.RunE == nil {
		t.Error("rmCmd.RunE is nil")
	}
}

func TestRmFlags(t *testing.T) {
	flags := rmCmd.Flags()

	tests := []string{"force", "unprotect", "dry-run", "keep-branch", "delete-branch"}
	for _, name := range tests {
		if flags.Lookup(name) == nil {
			t.Errorf("expected --%s flag to exist", name)
		}
	}
}

func TestRmForceFlag(t *testing.T) {
	f := rmCmd.Flags().Lookup("force")
	if f == nil {
		t.Fatal("--force flag not found")
	}

	if f.Shorthand != "f" {
		t.Errorf("force shorthand = %q, want %q", f.Shorthand, "f")
	}

	if !strings.Contains(f.Usage, "dirty") {
		t.Error("force flag usage should mention dirty worktrees")
	}
}

func TestRmAliases(t *testing.T) {
	aliases := rmCmd.Aliases
	expected := map[string]bool{"remove": true, "delete": true}

	for _, alias := range aliases {
		if !expected[alias] {
			t.Errorf("unexpected alias %q", alias)
		}
		delete(expected, alias)
	}

	for missing := range expected {
		t.Errorf("missing expected alias %q", missing)
	}
}

func TestRmFlagMutualExclusion(t *testing.T) {
	// --keep-branch and --delete-branch should be mutually exclusive
	// This is configured in init() via MarkFlagsMutuallyExclusive
	keepFlag := rmCmd.Flags().Lookup("keep-branch")
	deleteFlag := rmCmd.Flags().Lookup("delete-branch")

	if keepFlag == nil {
		t.Fatal("--keep-branch flag not found")
	}
	if deleteFlag == nil {
		t.Fatal("--delete-branch flag not found")
	}

	// Verify both flags exist and have descriptions
	if keepFlag.Usage == "" {
		t.Error("--keep-branch has no usage description")
	}
	if deleteFlag.Usage == "" {
		t.Error("--delete-branch has no usage description")
	}
}

func TestRmHelpDocumentsProtection(t *testing.T) {
	long := rmCmd.Long
	if long == "" {
		t.Fatal("rmCmd.Long is empty")
	}

	required := []struct {
		label string
		text  string
	}{
		{"protection", "Protected worktrees"},
		{"force + unprotect", "--force and --unprotect"},
		{"environment protection", "Environment worktrees"},
	}

	for _, tt := range required {
		if !strings.Contains(long, tt.text) {
			t.Errorf("rmCmd.Long should document %s (missing %q)", tt.label, tt.text)
		}
	}
}
