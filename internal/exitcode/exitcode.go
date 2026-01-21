// Package exitcode defines standard exit codes for Grove CLI commands.
// These codes follow the V2-SPEC.md specification for consistent error handling.
package exitcode

const (
	// Success indicates the command completed successfully.
	Success = 0

	// ResourceNotFound indicates a requested resource (worktree, branch, etc.) was not found.
	ResourceNotFound = 1

	// ResourceExists indicates an attempt to create something that already exists.
	ResourceExists = 2

	// GitOperationFailed indicates a git command failed.
	GitOperationFailed = 3

	// InvalidInput indicates invalid arguments or flags were provided.
	InvalidInput = 4

	// UserCancelled indicates the user cancelled an interactive operation.
	UserCancelled = 5

	// ExternalCommandFailed indicates an external command (docker, tmux, etc.) failed.
	ExternalCommandFailed = 6

	// CannotRemove indicates a worktree cannot be removed (dirty, protected, etc.).
	CannotRemove = 7

	// ConstraintViolated indicates a constraint was violated (e.g., syncing non-environment worktree).
	ConstraintViolated = 8

	// NotGroveProject indicates the command was run outside a grove project.
	// Commands requiring grove context should exit with this code.
	NotGroveProject = 10

	// WorktreeMissing indicates the worktree directory is missing from disk.
	// Used by grove repair when worktrees are orphaned.
	WorktreeMissing = 11
)

// Message returns a human-readable message for an exit code.
func Message(code int) string {
	switch code {
	case Success:
		return "success"
	case ResourceNotFound:
		return "resource not found"
	case ResourceExists:
		return "resource already exists"
	case GitOperationFailed:
		return "git operation failed"
	case InvalidInput:
		return "invalid input"
	case UserCancelled:
		return "operation cancelled"
	case ExternalCommandFailed:
		return "external command failed"
	case CannotRemove:
		return "cannot remove worktree"
	case ConstraintViolated:
		return "constraint violated"
	case NotGroveProject:
		return "not a grove project"
	case WorktreeMissing:
		return "worktree missing"
	default:
		return "unknown error"
	}
}
