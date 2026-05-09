package grove

import (
	"encoding/json"
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

// DriftReason describes whether the cwd's worktree is registered in grove state.
type DriftReason int

const (
	ReasonRegistered      DriftReason = iota // cwd is the main worktree, or a registered grove worktree
	ReasonDriftedWorktree                    // cwd is a git worktree but not in state.json
)

// stateSnapshot is a minimal representation of state.json used only for path
// lookup. It mirrors the persisted shape without importing internal/state.
type stateSnapshot struct {
	Worktrees map[string]struct {
		Path string `json:"path"`
	} `json:"worktrees"`
}

// IsWorktreeInState checks whether a worktree path appears as a registered
// path value in a state.json byte slice. It unmarshals only the worktrees map
// and compares path values directly, avoiding substring-match false positives
// and JSON-escape mismatches.
//
// Returns false when stateData is empty, nil, or not valid JSON.
func IsWorktreeInState(stateData []byte, worktreePath string) bool {
	if len(stateData) == 0 {
		return false
	}

	var snap stateSnapshot
	if err := json.Unmarshal(stateData, &snap); err != nil {
		return false
	}

	// Resolve symlinks on the caller's path once so we can compare against
	// either stored form (handles macOS /var → /private/var and similar).
	resolvedCaller, resolveCallerErr := filepath.EvalSymlinks(worktreePath)

	for _, wt := range snap.Worktrees {
		if wt.Path == worktreePath {
			return true
		}
		if resolveCallerErr == nil && resolvedCaller != worktreePath && wt.Path == resolvedCaller {
			return true
		}
		// Also resolve the stored path so a state.json written before
		// path normalization (unresolved /var/...) still matches a caller
		// that already passes a resolved /private/var/... path.
		resolvedStored, err := filepath.EvalSymlinks(wt.Path)
		if err != nil {
			continue
		}
		if resolvedStored == worktreePath {
			return true
		}
		if resolveCallerErr == nil && resolvedStored == resolvedCaller {
			return true
		}
	}
	return false
}

// DiagnoseDrift checks whether the worktree at worktreePath is registered in state.json
// at mainPath/.grove/state.json. Returns ReasonRegistered when it's the main worktree
// or appears in state, and ReasonDriftedWorktree otherwise.
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

	// Pass the original (unresolved) path so IsWorktreeInState can match both
	// the stored form and the resolved form (handles macOS /var → /private/var).
	if IsWorktreeInState(data, worktreePath) {
		return ReasonRegistered
	}
	return ReasonDriftedWorktree
}
