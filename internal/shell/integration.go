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

	// Replace placeholder with actual binary path
	output := zshTemplate
	output = fmt.Sprintf("# Grove shell integration for zsh\n__GROVE_BIN=\"%s\"\n\n%s", binaryPath, output)

	return output, nil
}

// GenerateBashIntegration returns the bash shell integration code
func GenerateBashIntegration() (string, error) {
	binaryPath, err := os.Executable()
	if err != nil {
		// Fallback to grove in PATH
		binaryPath = "grove"
	}

	// Replace placeholder with actual binary path
	output := bashTemplate
	output = fmt.Sprintf("# Grove shell integration for bash\n__GROVE_BIN=\"%s\"\n\n%s", binaryPath, output)

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
