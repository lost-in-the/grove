package worktree

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"

	"github.com/lost-in-the/grove/internal/cmdexec"
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
	Naming      struct {
		Pattern string `toml:"pattern"`
	} `toml:"naming"`
}

// DefaultNamePattern is the worktree directory naming pattern used when
// [naming] pattern is unset or invalid.
const DefaultNamePattern = "{project}-{name}"

// namePatternLiterals restricts literal (non-placeholder) characters to ones
// that are safe in directory names, git branch names, tmux targets, and
// GitHub URLs.
var namePatternLiterals = regexp.MustCompile(`^[A-Za-z0-9._-]*$`)

// ValidateNamePattern checks that a worktree naming pattern is usable:
// exactly one {project} and one {name} placeholder (so full names stay
// project-identifiable and short names are recoverable), with literal
// characters limited to [A-Za-z0-9._-].
func ValidateNamePattern(pattern string) error {
	if strings.Count(pattern, "{project}") != 1 {
		return fmt.Errorf("pattern must contain {project} exactly once")
	}
	if strings.Count(pattern, "{name}") != 1 {
		return fmt.Errorf("pattern must contain {name} exactly once")
	}
	literals := strings.ReplaceAll(pattern, "{project}", "")
	literals = strings.ReplaceAll(literals, "{name}", "")
	if !namePatternLiterals.MatchString(literals) {
		return fmt.Errorf("literal characters must match [A-Za-z0-9._-] (got %q)", literals)
	}
	return nil
}

// invalidPatternWarning ensures the invalid-pattern warning prints at most
// once per process even when several Managers are created.
var invalidPatternWarning sync.Once

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
	if output, err := cmdexec.Output(context.TODO(), "git", []string{"remote", "get-url", "origin"}, mainWorktreePath, cmdexec.GitLocal); err == nil {
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
	output, err := cmdexec.Output(context.TODO(), "git", []string{"worktree", "list", "--porcelain"}, m.repoRoot, cmdexec.GitLocal)
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

// DeriveWorktreeName derives a suggested worktree name from a branch name.
// Strategy "last_segment" (default) takes the part after the last "/".
// E.g., "feat/agent-slot-db" → "agent-slot-db", "main" → "main".
func DeriveWorktreeName(branch, strategy string) string {
	if strategy == "" || strategy == "last_segment" {
		if idx := strings.LastIndex(branch, "/"); idx >= 0 {
			return branch[idx+1:]
		}
		return branch
	}
	return branch
}

// getNamePattern returns the worktree naming pattern for the project, read
// from [naming] pattern in the main worktree's .grove/config.toml (the same
// project-level file detectProjectName reads). Unset or invalid patterns fall
// back to DefaultNamePattern; invalid ones additionally warn on stderr so the
// fallback is visible.
func (m *Manager) getNamePattern() string {
	if m.namePattern != "" {
		return m.namePattern
	}
	m.namePattern = DefaultNamePattern

	configPath := filepath.Join(m.getMainWorktreePath(), ".grove", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return m.namePattern
	}
	var cfg projectConfig
	if err := toml.Unmarshal(data, &cfg); err != nil || cfg.Naming.Pattern == "" {
		return m.namePattern
	}
	if err := ValidateNamePattern(cfg.Naming.Pattern); err != nil {
		invalidPatternWarning.Do(func() {
			fmt.Fprintf(os.Stderr, "grove: warning: ignoring invalid [naming] pattern %q: %v (using %q)\n",
				cfg.Naming.Pattern, err, DefaultNamePattern)
		})
		return m.namePattern
	}
	m.namePattern = cfg.Naming.Pattern
	return m.namePattern
}

// FullName returns the full worktree directory name, built from the project
// naming pattern (default "{project}-{name}", e.g. grove-cli-testing).
// Tmux session names intentionally do NOT follow the pattern — they always
// use the canonical {project}-{name} form (see TmuxSessionName).
func (m *Manager) FullName(name string) string {
	if m.projectName == "" {
		m.projectName = m.detectProjectName()
	}

	full := strings.ReplaceAll(m.getNamePattern(), "{project}", m.projectName)
	return strings.ReplaceAll(full, "{name}", name)
}

// ShortName inverts the naming pattern: given a full worktree directory name,
// it extracts the short {name} part. Names that don't match the pattern are
// returned unchanged (e.g. worktrees created before a pattern change or by
// raw `git worktree add`).
func (m *Manager) ShortName(fullName string) string {
	if m.projectName == "" {
		m.projectName = m.detectProjectName()
	}
	short, _ := shortNameFromFull(fullName, m.projectName, m.getNamePattern())
	return short
}

// shortNameFromFull extracts the {name} segment from a full directory name by
// matching the pattern's literal prefix and suffix around {name}. Returns the
// input unchanged (and false) when it doesn't match the pattern.
func shortNameFromFull(fullName, projectName, pattern string) (string, bool) {
	before, after, found := strings.Cut(pattern, "{name}")
	if !found {
		return fullName, false
	}
	prefix := strings.ReplaceAll(before, "{project}", projectName)
	suffix := strings.ReplaceAll(after, "{project}", projectName)
	if len(fullName) > len(prefix)+len(suffix) &&
		strings.HasPrefix(fullName, prefix) &&
		strings.HasSuffix(fullName, suffix) {
		return fullName[len(prefix) : len(fullName)-len(suffix)], true
	}
	return fullName, false
}
