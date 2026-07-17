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
	"github.com/lost-in-the/grove/internal/config"
)

// ValidateWorktreeName reports why name is unusable as a worktree short name,
// or "" if it is valid (an empty name is treated as valid here — callers reject
// empties separately). It rejects path separators and shell/tmux
// metacharacters (so `grove new ../escape` can't place a worktree outside the
// project, and names stay safe in tmux targets), control characters including
// newlines (which would corrupt the cd: shell-integration protocol), a leading
// - or . (flag parsing / hidden files), and the reserved name "root" (the main
// worktree's display name). Shared by the CLI create/rename paths and the TUI.
func ValidateWorktreeName(name string) string {
	if name == "" {
		return ""
	}
	if name == "root" {
		return `"root" is reserved for the main worktree`
	}
	if strings.ContainsAny(name, " /\\:*?\"<>|") {
		return "name contains invalid characters"
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "name contains control characters"
		}
	}
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, ".") {
		return "name cannot start with - or ."
	}
	return ""
}

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

// DefaultNamePattern is the worktree directory naming pattern used when
// [naming] pattern is unset or invalid.
const DefaultNamePattern = "{project}-{name}"

// namePatternLiterals restricts literal (non-placeholder) characters to ones
// that are safe in directory names, git branch names, and GitHub URLs.
// Note '.' and ':' are NOT safe in tmux targets — the pattern only governs
// directory names; tmux session names are sanitized separately (see
// TmuxSessionName).
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

// EffectiveNamePattern returns pattern when it is a valid naming pattern and
// DefaultNamePattern otherwise (including empty input).
func EffectiveNamePattern(pattern string) string {
	if ValidateNamePattern(pattern) == nil {
		return pattern
	}
	return DefaultNamePattern
}

// InterpolateNamePattern builds a full worktree directory name from a naming
// pattern. Invalid or empty patterns fall back to DefaultNamePattern, so
// callers (e.g. UI previews) always produce the name grove would create.
func InterpolateNamePattern(pattern, project, name string) string {
	full := strings.ReplaceAll(EffectiveNamePattern(pattern), "{project}", project)
	return strings.ReplaceAll(full, "{name}", name)
}

// detectProjectNameAt determines the project name using priority:
// 1. .grove/config.toml -> project_name (from main worktree)
// 2. Git remote origin URL -> repo name (from main worktree)
// 3. Main worktree directory name as fallback
//
// The main worktree path is taken as a parameter so callers that already
// parsed `git worktree list --porcelain` (e.g. listLight) don't trigger a
// redundant exec via getMainWorktreePath.
func (m *Manager) detectProjectNameAt(mainWorktreePath string) string {
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

// getNamePattern returns the worktree naming pattern, loaded through the
// standard layered config (defaults → global → project → config.local.toml)
// anchored at the main worktree's .grove directory — the same value `grove
// config` displays. Invalid patterns fall back to DefaultNamePattern with a
// stderr warning so the fallback is visible.
func (m *Manager) getNamePattern() string {
	m.mu.Lock()
	cached := m.namePattern
	m.mu.Unlock()
	if cached != "" {
		return cached
	}
	return m.namePatternAt(m.getMainWorktreePath())
}

// namePatternAt is getNamePattern with the main worktree path already in
// hand, so priming a cold cache doesn't re-exec `git worktree list`.
func (m *Manager) namePatternAt(mainWorktreePath string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.namePattern != "" {
		return m.namePattern
	}
	pattern := DefaultNamePattern

	groveDir := filepath.Join(mainWorktreePath, ".grove")
	cfg, err := config.LoadFromGroveDir(groveDir)
	if err == nil && cfg != nil {
		if perr := ValidateNamePattern(cfg.Naming.Pattern); perr != nil {
			invalidPatternWarning.Do(func() {
				fmt.Fprintf(os.Stderr, "grove: warning: ignoring invalid [naming] pattern %q: %v (using %q)\n",
					cfg.Naming.Pattern, perr, DefaultNamePattern)
			})
		} else {
			pattern = cfg.Naming.Pattern
		}
	}
	m.namePattern = pattern
	return m.namePattern
}

// FullName returns the full worktree directory name, built from the project
// naming pattern (default "{project}-{name}", e.g. grove-cli-testing).
// Tmux session names intentionally do NOT follow the pattern — they always
// use the canonical {project}-{name} form (see TmuxSessionName).
func (m *Manager) FullName(name string) string {
	return InterpolateNamePattern(m.getNamePattern(), m.GetProjectName(), name)
}

// ShortName inverts the naming pattern: given a full worktree directory name,
// it extracts the short {name} part. Names that don't match the pattern are
// returned unchanged (e.g. worktrees created before a pattern change or by
// raw `git worktree add`).
func (m *Manager) ShortName(fullName string) string {
	short, _ := shortNameFromFull(fullName, m.GetProjectName(), m.getNamePattern())
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
