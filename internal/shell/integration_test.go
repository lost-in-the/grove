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
		"GROVE_SHELL=1",
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
		"GROVE_SHELL=1",
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

func TestZshDirectiveParsing(t *testing.T) {
	// Test that the zsh template correctly handles cd: directives
	output, err := GenerateZshIntegration()
	if err != nil {
		t.Fatalf("GenerateZshIntegration() failed: %v", err)
	}

	// Verify the parsing logic is present
	expectedPatterns := []string{
		"cd:*",              // Pattern matching for cd directive
		"cd_target=",        // Variable to store target directory
		"should_cd=",        // Flag to track if cd should be executed
		"cd \"$cd_target\"", // Actual cd execution
		"GROVE_SHELL=1",     // Environment variable set
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(output, pattern) {
			t.Errorf("GenerateZshIntegration() missing expected pattern: %s", pattern)
		}
	}
}

func TestBashDirectiveParsing(t *testing.T) {
	// Test that the bash template correctly handles cd: directives
	output, err := GenerateBashIntegration()
	if err != nil {
		t.Fatalf("GenerateBashIntegration() failed: %v", err)
	}

	// Verify the parsing logic is present
	expectedPatterns := []string{
		"cd:*",              // Pattern matching for cd directive
		"cd_target=",        // Variable to store target directory
		"should_cd=",        // Flag to track if cd should be executed
		"cd \"$cd_target\"", // Actual cd execution
		"GROVE_SHELL=1",     // Environment variable set
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(output, pattern) {
			t.Errorf("GenerateBashIntegration() missing expected pattern: %s", pattern)
		}
	}
}

func TestBinaryResolutionUsesDynamicLookup(t *testing.T) {
	output, err := GenerateZshIntegration()
	if err != nil {
		t.Fatalf("GenerateZshIntegration() failed: %v", err)
	}

	// Should use command -v for dynamic resolution, not a hardcoded path
	if !strings.Contains(output, "command -v grove") {
		t.Error("shell integration should use 'command -v grove' for binary resolution")
	}

	// Should NOT contain a hardcoded absolute path for __GROVE_BIN
	// (The old approach used os.Executable() which produces paths like /usr/local/bin/grove)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "__GROVE_BIN=\"/") {
			t.Errorf("shell integration should not hardcode absolute path in __GROVE_BIN, found: %s", line)
		}
	}
}

func TestBashBinaryResolutionUsesDynamicLookup(t *testing.T) {
	output, err := GenerateBashIntegration()
	if err != nil {
		t.Fatalf("GenerateBashIntegration() failed: %v", err)
	}

	if !strings.Contains(output, "command -v grove") {
		t.Error("bash integration should use 'command -v grove' for binary resolution")
	}
}
