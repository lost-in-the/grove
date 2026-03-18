package shell

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lost-in-the/grove/internal/cmdexec"
)

//go:embed templates/grove.zsh
var zshTemplate string

//go:embed templates/grove.bash
var bashTemplate string

// GenerateZshIntegration returns the zsh shell integration code
func GenerateZshIntegration() (string, error) {
	return generateIntegration("zsh", "~/.zshrc", "grove install zsh", zshTemplate)
}

// GenerateBashIntegration returns the bash shell integration code
func GenerateBashIntegration() (string, error) {
	return generateIntegration("bash", "~/.bashrc", "grove install bash", bashTemplate)
}

// generateIntegration produces shell integration code for the given shell.
func generateIntegration(shell, rcFile, installCmd, template string) (string, error) {
	// Resolve the binary path dynamically. If grove isn't on PATH when the
	// shell sources this (e.g. PATH not yet set up in .zshrc), bail out
	// instead of falling back to the bare name "grove" — that would cause
	// the grove() function to call itself recursively (infinite loop).
	binResolver := `__GROVE_BIN="$(command -v grove 2>/dev/null)" || {
    echo "grove: binary not found on PATH — shell integration disabled" >&2
    return 0 2>/dev/null || true
}`

	header := fmt.Sprintf(`# Grove shell integration for %s
# ─────────────────────────────────────────────────────────────────────────────
# SETUP: Add this line to your %s, then restart your shell:
#   eval "$(%s)"
#
# WHAT THIS DOES:
#   1. Creates a 'grove' shell function that wraps the binary
#      - Directive commands (to, last, fork, fetch, attach, open, up, run,
#        restart): output is captured and parsed for cd:/tmux-attach:/env:
#        directives
#      - All other commands: run directly (streaming-safe for logs, test, etc.)
#   2. Registers tab completion for grove commands and worktree names
#   3. Creates 'w' as an alias for 'grove'
#
# WHY A WRAPPER: Subprocesses cannot change the parent shell's directory.
# The wrapper captures directive-producing commands, detects 'cd:' directives,
# and executes the directory change in your current shell. Non-directive
# commands run directly for proper streaming output.
# ─────────────────────────────────────────────────────────────────────────────

`, shell, rcFile, installCmd)

	output := fmt.Sprintf("%s%s\n__GROVE_SHELL_VERSION=%d\n\n%s", header, binResolver, ShellVersion, template)
	return output, nil
}

// GetWorktreeNames returns a list of worktree names for completion.
// This function is intended for future use by server-side completion
// or programmatic access to worktree names. Shell completions currently
// use git commands directly in the shell templates for better performance.
func GetWorktreeNames() ([]string, error) {
	output, err := cmdexec.Output(context.TODO(), "git", []string{"worktree", "list", "--porcelain"}, "", cmdexec.GitLocal)
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
