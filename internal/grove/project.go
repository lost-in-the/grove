// Package grove provides utilities for working with grove projects.
package grove

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lost-in-the/grove/internal/cmdexec"
)

// FindRoot searches for the .grove directory starting from startDir and walking up.
// Returns the path to the .grove directory if found, or empty string if not found.
func FindRoot(startDir string) (string, error) {
	if startDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		startDir = cwd
	}

	// Convert to absolute path
	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	// Resolve symlinks so the walk compares like with like against the git
	// root: `git rev-parse --show-toplevel` returns the symlink-resolved
	// path (e.g. /private/var on macOS) while os.Getwd may return the
	// logical $PWD path (/var). Without this the boundary check below never
	// fires and the walk can escape the repository. Same rule as diagnose.go.
	if resolved, err := filepath.EvalSymlinks(absDir); err == nil {
		absDir = resolved
	}

	// Determine git repo root to limit search scope.
	// Without this boundary, the walk can escape the repo and find
	// unrelated .grove directories (e.g., ~/.grove from debug logging).
	var gitRoot string
	if out, err := cmdexec.Output(context.TODO(), "git", []string{"-C", absDir, "rev-parse", "--show-toplevel"}, "", cmdexec.GitLocal); err == nil {
		gitRoot = strings.TrimSpace(string(out))
	} else if !isNotGitRepoErr(err) {
		// rev-parse can fail for reasons other than "not a git repository"
		// (dubious ownership in containers, unsupported repo format, git
		// missing). Treating those as "no project" produces a false
		// diagnosis — surface git's actual, actionable error instead.
		if stderr := gitStderr(err); stderr != "" {
			return "", fmt.Errorf("failed to locate git repository: %s", stderr)
		}
		return "", fmt.Errorf("failed to locate git repository: %w", err)
	}

	// Resolve the main worktree root — the parent of git's common dir, which
	// points at the main worktree from any worktree. The canonical .grove lives
	// there: `grove init` refuses to run from a linked worktree, so state.json
	// only ever exists in the main worktree.
	mainRoot := ""
	if commonDir, err := GitCommonDir(absDir); err == nil {
		mainRoot = filepath.Dir(commonDir)
	}

	// When cwd is inside the MAIN worktree (its git toplevel IS the main root),
	// walk up from cwd to the main root and honor the nearest .grove — so a
	// genuine nested project (a .grove between cwd and the root) isn't shadowed
	// by the root's. This can't pick up a legacy per-worktree .grove: those only
	// ever existed in LINKED worktrees (older grove's config.toml symlink), and
	// the anchor below handles those. The git-root boundary also keeps the walk
	// from escaping to the global ~/.grove (debug logs, update cache) (#138).
	if gitRoot != "" && gitRoot == mainRoot {
		// The loop condition is a containment check, not just the equality
		// below: if EvalSymlinks left absDir and gitRoot diverged (partial
		// resolution failures, case-mangled paths), `current == gitRoot`
		// would never fire and the walk would escape the repository — the
		// exact bug the boundary exists to prevent. Outside gitRoot the walk
		// simply doesn't run and the main-worktree anchor below decides.
		current := absDir
		for isWithinDir(current, gitRoot) {
			groveDir := filepath.Join(current, ".grove")
			if info, err := os.Stat(groveDir); err == nil && info.IsDir() {
				return groveDir, nil
			}
			if current == gitRoot {
				break
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}

	// Anchor: from a LINKED worktree (git toplevel != main root), or if the
	// main-worktree walk found nothing, resolve the main worktree's .grove
	// directly — skipping any legacy per-worktree .grove a linked worktree might
	// still carry (older grove's config.toml symlink, never state.json), which
	// would otherwise fragment state per-worktree (B1): `grove new`/`rm`/`last`
	// run from inside a worktree would read and write a phantom state.json.
	if mainGrove, ok := mainWorktreeGroveDir(absDir); ok {
		return mainGrove, nil
	}

	// Fallback: find main worktree's .grove via git
	mainPath, err := getMainWorktreePath(absDir)
	if err == nil && mainPath != "" {
		groveDir := filepath.Join(mainPath, ".grove")
		if info, err := os.Stat(groveDir); err == nil && info.IsDir() {
			return groveDir, nil
		}
	}

	return "", nil
}

// isWithinDir reports whether path is dir itself or a descendant of it.
// Separator-aware: "/repo-other" is NOT within "/repo", so a plain prefix
// check can't be fooled by sibling directories sharing a name prefix.
// Both paths must already be absolute and cleaned (FindRoot guarantees this
// via filepath.Abs/EvalSymlinks and git's own output).
func isWithinDir(path, dir string) bool {
	if path == dir {
		return true
	}
	return strings.HasPrefix(path, strings.TrimSuffix(dir, string(filepath.Separator))+string(filepath.Separator))
}

// isNotGitRepoErr reports whether a git command failure means "the directory
// is not inside a git repository" (the one failure FindRoot treats as a clean
// no-project answer), as opposed to git erroring for some other reason.
func isNotGitRepoErr(err error) bool {
	return strings.Contains(gitStderr(err), "not a git repository")
}

// gitStderr extracts the trimmed stderr of a failed git invocation, or ""
// when unavailable (timeout, git not on PATH).
func gitStderr(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return strings.TrimSpace(string(exitErr.Stderr))
	}
	return ""
}

// IsGroveProject checks if the current directory is within a grove project.
// Returns the .grove directory path if found, or empty string if not.
func IsGroveProject() (string, error) {
	return FindRoot("")
}

// ProjectRoot returns the project root directory (parent of .grove).
// Returns empty string if not in a grove project.
func ProjectRoot() (string, error) {
	groveDir, err := FindRoot("")
	if err != nil {
		return "", err
	}
	if groveDir == "" {
		return "", nil
	}
	return filepath.Dir(groveDir), nil
}

// IsInsideWorktree checks if the current directory is inside a git worktree
// (as opposed to the main repository). This is used by grove setup to prevent
// initialization from within a worktree.
func IsInsideWorktree() (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, err
	}

	// Check for .git file (worktrees have a .git file pointing to the main repo)
	gitPath := filepath.Join(cwd, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No .git at all - walk up to find it
			return isWorktreeByWalkingUp(cwd)
		}
		return false, err
	}

	// If .git is a file (not directory), we're in a worktree
	return !info.IsDir(), nil
}

// isWorktreeByWalkingUp walks up the directory tree to find .git
func isWorktreeByWalkingUp(startDir string) (bool, error) {
	current := startDir
	for {
		gitPath := filepath.Join(current, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			// Found .git - check if it's a file (worktree) or directory (main repo)
			return !info.IsDir(), nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root - not in any git repo
			return false, nil
		}
		current = parent
	}
}

// ConfigPath returns the path to the config.toml file for the current project.
// Returns empty string if not in a grove project.
func ConfigPath() (string, error) {
	groveDir, err := FindRoot("")
	if err != nil || groveDir == "" {
		return "", err
	}
	return filepath.Join(groveDir, "config.toml"), nil
}

// MustProjectRoot returns the project root given a .grove directory path.
// This is a convenience function when you already have the grove directory.
func MustProjectRoot(groveDir string) string {
	return filepath.Dir(groveDir)
}

// mainWorktreeGroveDir resolves the main worktree's .grove directory from dir
// via GitCommonDir, which returns the main worktree's .git directory
// regardless of which worktree dir is in. Returns ok=false when dir isn't a
// git repo, the common dir can't be resolved, or the main worktree has no
// .grove (i.e. not a grove project — let the caller's fallbacks decide).
func mainWorktreeGroveDir(dir string) (string, bool) {
	commonDir, err := GitCommonDir(dir)
	if err != nil {
		return "", false
	}
	groveDir := filepath.Join(filepath.Dir(commonDir), ".grove")
	if info, err := os.Stat(groveDir); err != nil || !info.IsDir() {
		return "", false
	}
	return groveDir, true
}

// getMainWorktreePath returns the path of the main worktree by parsing
// the first entry from `git worktree list --porcelain`.
func getMainWorktreePath(fromDir string) (string, error) {
	output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", fromDir, "worktree", "list", "--porcelain"}, "", cmdexec.GitLocal)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(output), "\n") {
		if path, found := strings.CutPrefix(line, "worktree "); found {
			return path, nil
		}
	}
	return "", nil
}
