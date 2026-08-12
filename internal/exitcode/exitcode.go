// Package exitcode defines standard exit codes for Grove CLI commands.
// These codes provide consistent error handling across all Grove commands.
package exitcode

const (
	// ResourceNotFound indicates a requested resource (worktree, branch, etc.) was not found.
	ResourceNotFound = 1

	// ResourceExists indicates an attempt to create something that already exists.
	ResourceExists = 2

	// GitOperationFailed indicates a git command failed.
	GitOperationFailed = 3

	// InvalidInput indicates invalid arguments or flags were provided.
	InvalidInput = 4

	// UserCancelled indicates the user canceled an interactive operation.
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

	// MountDrift indicates `grove here --check-mount` detected a mismatch
	// between the env-configured worktree and a running container's
	// bind-mount source. Used to gate scripts ("did I forget to grove up
	// after switching?").
	MountDrift = 12

	// MountCheckMismatch indicates `grove here --check-mount --require-current`
	// found that the current worktree (cwd) is not the one the stack's env
	// file is configured for. Distinct from MountDrift on purpose: this is a
	// cwd-vs-configured comparison, not the env-vs-container comparison
	// MountDrift reports, so a script can tell "wrong worktree" apart from
	// "containers weren't restarted."
	MountCheckMismatch = 13
)
