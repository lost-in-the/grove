package worktree

import (
	"crypto/md5"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// TestEnvNumber derives a stable TEST_ENV_NUMBER in range [50, 99] from a worktree name.
// The same name always produces the same number (deterministic via MD5 hash).
func TestEnvNumber(name string) int {
	h := md5.Sum([]byte(name))
	num := binary.BigEndian.Uint32(h[:4])
	return int(num%50) + 50
}

// projectConfig represents the project-level configuration
type projectConfig struct {
	ProjectName string `toml:"project_name"`
}

// detectProjectName determines the project name using priority:
// 1. .grove/config.toml -> project_name (from main worktree)
// 2. Git remote origin URL -> repo name (from main worktree)
// 3. Main worktree directory name as fallback
func (m *Manager) detectProjectName() string {
	// Get the main worktree path (first in the list)
	mainWorktreePath := m.getMainWorktreePath()

	// Priority 1: Check .grove/config.toml in main worktree
	configPath := filepath.Join(mainWorktreePath, ".grove", "config.toml")
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg projectConfig
		if err := toml.Unmarshal(data, &cfg); err == nil && cfg.ProjectName != "" {
			return cfg.ProjectName
		}
	}

	// Priority 2: Extract from git remote URL
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = mainWorktreePath
	if output, err := cmd.Output(); err == nil {
		remoteURL := strings.TrimSpace(string(output))
		if projectName := extractProjectNameFromRemote(remoteURL); projectName != "" {
			return projectName
		}
	}

	// Priority 3: Use main worktree directory name
	return filepath.Base(mainWorktreePath)
}

// getMainWorktreePath returns the path to the main (first) worktree
func (m *Manager) getMainWorktreePath() string {
	// Try to get the list of worktrees
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.repoRoot

	output, err := cmd.Output()
	if err != nil {
		// Fallback to repoRoot if we can't get worktree list
		return m.repoRoot
	}

	// Parse the first worktree path
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			return path
		}
	}

	// Fallback to repoRoot
	return m.repoRoot
}

// extractProjectNameFromRemote extracts the repository name from a git remote URL.
// Handles both HTTPS and SSH formats:
//   - https://github.com/user/repo.git -> repo
//   - git@github.com:user/repo.git -> repo
//   - https://github.com/user/repo -> repo
//
// Returns empty string for invalid URLs.
func extractProjectNameFromRemote(remoteURL string) string {
	if remoteURL == "" {
		return ""
	}

	// Remove .git suffix if present
	remoteURL = strings.TrimSuffix(remoteURL, ".git")

	// Handle SSH format (git@github.com:user/repo)
	if strings.Contains(remoteURL, ":") && strings.Contains(remoteURL, "@") {
		parts := strings.Split(remoteURL, ":")
		if len(parts) >= 2 {
			path := parts[len(parts)-1]
			return filepath.Base(path)
		}
	}

	// Handle HTTPS format (https://github.com/user/repo)
	// Just get the last component of the path
	return filepath.Base(remoteURL)
}

// FullName returns the full worktree name with project prefix.
// Format: {project}-{name}
// Example: grove-cli-testing
func (m *Manager) FullName(name string) string {
	if m.projectName == "" {
		m.projectName = m.detectProjectName()
	}

	// Return project-name format
	return m.projectName + "-" + name
}
