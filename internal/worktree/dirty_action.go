package worktree

// DirtyAction represents the resolved action to take when the current worktree has uncommitted changes.
type DirtyAction int

const (
	DirtyAllow  DirtyAction = iota // Proceed with switch
	DirtyStash                     // Auto-stash changes before switching
	DirtyPrompt                    // Ask the user interactively
	DirtyRefuse                    // Block the switch
)

// ResolveDirtyAction determines what to do when switching away from a worktree
// based on the configured dirty handling mode, current state, and environment.
func ResolveDirtyAction(dirtyHandling string, isDirty, isPeek, isInteractive bool) DirtyAction {
	// Peek mode always allows — it's a lightweight switch that skips hooks
	if isPeek {
		return DirtyAllow
	}

	// Clean worktree always allows — nothing to protect
	if !isDirty {
		return DirtyAllow
	}

	// Dirty worktree: resolve based on config
	switch dirtyHandling {
	case "refuse":
		return DirtyRefuse
	case "auto-stash":
		return DirtyStash
	case "prompt":
		if isInteractive {
			return DirtyPrompt
		}
		// Non-interactive fallback: refuse (safe default)
		return DirtyRefuse
	default:
		return DirtyRefuse
	}
}
