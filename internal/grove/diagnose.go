package grove

import (
	"os"
	"path/filepath"
)

// DiagnoseReason describes why a directory isn't a grove project.
type DiagnoseReason int

const (
	ReasonNoGroveDir               DiagnoseReason = iota // In a git repo, no .grove anywhere
	ReasonNotGitRepo                                     // Not in a git repository
	ReasonMainWorktreeMissingGrove                       // In a worktree, main has no .grove
)

// DiagnoseResult holds the outcome of diagnosing a missing grove project.
type DiagnoseResult struct {
	Reason           DiagnoseReason
	MainWorktreePath string // populated for worktree-related reasons
}

// DiagnoseNoGrove inspects a directory to determine WHY it isn't a grove project.
// Call this after FindRoot returns empty to provide a contextual error message.
func DiagnoseNoGrove(dir string) DiagnoseResult {
	// Check if we're in a git repo at all
	mainPath, err := getMainWorktreePath(dir)
	if err != nil || mainPath == "" {
		return DiagnoseResult{Reason: ReasonNotGitRepo}
	}

	// Resolve symlinks for accurate path comparison (macOS /var → /private/var)
	absDir, _ := filepath.Abs(dir)
	resolvedDir, err := filepath.EvalSymlinks(absDir)
	if err != nil {
		resolvedDir = absDir
	}
	resolvedMain, err := filepath.EvalSymlinks(mainPath)
	if err != nil {
		resolvedMain = mainPath
	}

	// Check if we're in a secondary worktree
	if resolvedDir != resolvedMain {
		// We're in a worktree — check if main has .grove
		mainGrove := filepath.Join(resolvedMain, ".grove")
		if _, err := os.Stat(mainGrove); os.IsNotExist(err) {
			return DiagnoseResult{
				Reason:           ReasonMainWorktreeMissingGrove,
				MainWorktreePath: resolvedMain,
			}
		}
	}

	return DiagnoseResult{Reason: ReasonNoGroveDir}
}
