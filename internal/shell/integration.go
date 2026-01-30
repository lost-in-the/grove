package shell

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed templates/grove.zsh
var zshTemplate string

//go:embed templates/grove.bash
var bashTemplate string

// GenerateZshIntegration returns the zsh shell integration code
func GenerateZshIntegration() (string, error) {
	binaryPath, err := os.Executable()
	if err != nil {
		// Fallback to grove in PATH
		binaryPath = "grove"
	}

	header := `# Grove shell integration for zsh
# ─────────────────────────────────────────────────────────────────────────────
# SETUP: Add this line to your ~/.zshrc, then restart your shell:
#   eval "$(grove install zsh)"
#
# WHAT THIS DOES:
#   1. Creates a 'grove' shell function that wraps the binary
#      - Intercepts 'cd:/path' output from commands like 'grove to'
#      - Actually changes your working directory (binaries can't do this)
#   2. Registers tab completion for grove commands and worktree names
#   3. Creates 'w' as an alias for 'grove'
#
# WHY A WRAPPER: Subprocesses cannot change the parent shell's directory.
# The wrapper captures grove's output, detects 'cd:' directives, and
# executes the directory change in your current shell.
# ─────────────────────────────────────────────────────────────────────────────

`
	output := fmt.Sprintf("%s__GROVE_BIN=\"%s\"\n\n%s", header, binaryPath, zshTemplate)

	return output, nil
}

// GenerateBashIntegration returns the bash shell integration code
func GenerateBashIntegration() (string, error) {
	binaryPath, err := os.Executable()
	if err != nil {
		// Fallback to grove in PATH
		binaryPath = "grove"
	}

	header := `# Grove shell integration for bash
# ─────────────────────────────────────────────────────────────────────────────
# SETUP: Add this line to your ~/.bashrc, then restart your shell:
#   eval "$(grove install bash)"
#
# WHAT THIS DOES:
#   1. Creates a 'grove' shell function that wraps the binary
#      - Intercepts 'cd:/path' output from commands like 'grove to'
#      - Actually changes your working directory (binaries can't do this)
#   2. Registers tab completion for grove commands and worktree names
#   3. Creates 'w' as an alias for 'grove'
#
# WHY A WRAPPER: Subprocesses cannot change the parent shell's directory.
# The wrapper captures grove's output, detects 'cd:' directives, and
# executes the directory change in your current shell.
# ─────────────────────────────────────────────────────────────────────────────

`
	output := fmt.Sprintf("%s__GROVE_BIN=\"%s\"\n\n%s", header, binaryPath, bashTemplate)

	return output, nil
}

// GetWorktreeNames returns a list of worktree names for completion.
// This function is intended for future use by server-side completion
// or programmatic access to worktree names. Shell completions currently
// use git commands directly in the shell templates for better performance.
func GetWorktreeNames() ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse worktree names
	names := []string{}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if len(line) >= 9 && strings.HasPrefix(line, "worktree ") {
			path := line[9:]
			// Extract just the directory name
			names = append(names, filepath.Base(path))
		}
	}

	return names, nil
}
