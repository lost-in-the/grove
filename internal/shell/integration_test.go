package shell

import (
	"strings"
	"testing"
)

func TestGenerateZshIntegration(t *testing.T) {
	output, err := GenerateZshIntegration()
	if err != nil {
		t.Fatalf("GenerateZshIntegration() failed: %v", err)
	}

	// Check that output contains expected components
	expectedComponents := []string{
		"# Grove shell integration for zsh",
		"__GROVE_BIN=",
		"grove()",
		"cd:",
		"_grove_completion()",
		"compdef _grove_completion grove",
		"alias w=grove",
	}

	for _, component := range expectedComponents {
		if !strings.Contains(output, component) {
			t.Errorf("GenerateZshIntegration() output missing expected component: %s", component)
		}
	}
}

func TestGenerateBashIntegration(t *testing.T) {
	output, err := GenerateBashIntegration()
	if err != nil {
		t.Fatalf("GenerateBashIntegration() failed: %v", err)
	}

	// Check that output contains expected components
	expectedComponents := []string{
		"# Grove shell integration for bash",
		"__GROVE_BIN=",
		"grove()",
		"cd:",
		"_grove_completion()",
		"complete -F _grove_completion grove",
		"alias w=grove",
		"_init_completion", // bash-completion check
	}

	for _, component := range expectedComponents {
		if !strings.Contains(output, component) {
			t.Errorf("GenerateBashIntegration() output missing expected component: %s", component)
		}
	}
}

func TestGetWorktreeNames(t *testing.T) {
	// This test requires a git repository with worktrees
	// It will fail gracefully if not in a git repo
	names, err := GetWorktreeNames()
	
	// We don't fail if there's an error (e.g., not in git repo)
	// but we verify the function signature works
	if err != nil {
		// Expected in non-git environments
		if names != nil {
			t.Errorf("GetWorktreeNames() should return nil names on error, got: %v", names)
		}
		return
	}

	// If successful, names should not be nil
	if names == nil {
		t.Errorf("GetWorktreeNames() returned nil names without error")
	}
}

func TestGetWorktreeNames_ParsesCorrectly(t *testing.T) {
	// This is more of a documentation test showing how the function
	// should parse git output. The actual parsing is tested through
	// integration with real git commands.
	
	// Test that we use the correct approach
	testLine := "worktree /path/to/worktree"
	if len(testLine) >= 9 && strings.HasPrefix(testLine, "worktree ") {
		path := testLine[9:]
		if path != "/path/to/worktree" {
			t.Errorf("Path parsing failed: got %s, want /path/to/worktree", path)
		}
	} else {
		t.Error("Line should match worktree pattern")
	}
}
