package grove

import (
	"os"
	"path/filepath"
	"strings"
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

// DriftReason describes whether the cwd's worktree is registered in grove state.
type DriftReason int

const (
	ReasonRegistered      DriftReason = iota // cwd is the main worktree, or a registered grove worktree
	ReasonDriftedWorktree                    // cwd is a git worktree but not in state.json
)

// DiagnoseDrift checks whether the worktree at worktreePath is registered in state.json
// at mainPath/.grove/state.json. Returns ReasonRegistered when it's the main worktree
// or appears in state, and ReasonDriftedWorktree otherwise.
//
// This is intentionally lightweight (no JSON parsing of complex shapes): it just
// checks whether the worktree path appears as a value in the state's worktrees map.
func DiagnoseDrift(worktreePath, mainPath string) DriftReason {
	resolvedWT, _ := filepath.EvalSymlinks(worktreePath)
	if resolvedWT == "" {
		resolvedWT = worktreePath
	}
	resolvedMain, _ := filepath.EvalSymlinks(mainPath)
	if resolvedMain == "" {
		resolvedMain = mainPath
	}
	if resolvedWT == resolvedMain {
		return ReasonRegistered
	}

	statePath := filepath.Join(resolvedMain, ".grove", "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		// No state file = brand new project, treat as registered (don't nag).
		return ReasonRegistered
	}

	// Look for the worktree path as a substring in the state file.
	// Full JSON parse would be more robust, but state.go owns that and a
	// lightweight check here keeps this package's surface area small.
	// Check both the resolved and unresolved paths to handle symlinked temp
	// directories (e.g. macOS /var → /private/var).
	stateStr := string(data)
	if strings.Contains(stateStr, `"`+resolvedWT+`"`) {
		return ReasonRegistered
	}
	if worktreePath != resolvedWT && strings.Contains(stateStr, `"`+worktreePath+`"`) {
		return ReasonRegistered
	}
	return ReasonDriftedWorktree
}
