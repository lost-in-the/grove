package worktree

import (
	"fmt"
	"time"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/log"
	"github.com/lost-in-the/grove/internal/state"
)

// BootstrapOpts holds the inputs needed to bootstrap a worktree (whether
// freshly created via grove new or adopted post-hoc via grove adopt).
type BootstrapOpts struct {
	Name          string    // short worktree name (e.g., "feature")
	Branch        string    // branch the worktree is on
	WorktreePath  string    // absolute path to the worktree directory
	MainPath      string    // absolute path to the main worktree (parent of .grove)
	ProjectName   string    // project name for hook context
	Now           time.Time // injected for testability
	IsEnvironment bool      // true for environment worktrees
	Mirror        string    // mirror name when IsEnvironment is true
}

// BootstrapWorktree runs the post-git-worktree-add bootstrap sequence:
//  1. Symlink config.toml from main worktree
//  2. Register the worktree in state.json (idempotent — re-registers on second call)
//  3. Fire post-create hooks (per-project hooks.toml, then global plugin hooks)
//
// Returns an error only if state registration or symlinking fails irrecoverably.
// Hook failures are logged via the hooks framework but do not abort the bootstrap.
func BootstrapWorktree(stateMgr *state.Manager, cfg *config.Config, opts BootstrapOpts) error {
	if opts.WorktreePath == "" || opts.MainPath == "" {
		return fmt.Errorf("WorktreePath and MainPath are required")
	}

	if err := grove.EnsureConfigSymlink(opts.MainPath, opts.WorktreePath); err != nil {
		return fmt.Errorf("symlink config: %w", err)
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	wsState := &state.WorktreeState{
		Path:           opts.WorktreePath,
		Branch:         opts.Branch,
		Root:           false,
		CreatedAt:      now,
		LastAccessedAt: now,
		Environment:    opts.IsEnvironment,
	}
	if opts.IsEnvironment {
		wsState.Mirror = opts.Mirror
		wsState.LastSyncedAt = &now
	}
	if err := stateMgr.AddWorktree(opts.Name, wsState); err != nil {
		return fmt.Errorf("register worktree: %w", err)
	}

	// Per-project post-create hooks
	hookExecutor, hookErr := hooks.NewExecutor()
	if hookErr != nil {
		log.Printf("hooks: failed to load config during bootstrap: %v", hookErr)
	} else if hookExecutor.HasHooksForEvent(hooks.EventPostCreate) {
		hookCtx := &hooks.ExecutionContext{
			Event:        hooks.EventPostCreate,
			Worktree:     opts.Name,
			WorktreeFull: opts.ProjectName + "-" + opts.Name,
			Branch:       opts.Branch,
			Project:      opts.ProjectName,
			MainPath:     opts.MainPath,
			NewPath:      opts.WorktreePath,
		}
		_ = hookExecutor.Execute(hooks.EventPostCreate, hookCtx)
	}

	// Global plugin post-create hook (e.g., docker external)
	globalHookCtx := &hooks.Context{
		Worktree:     opts.Name,
		Config:       cfg,
		WorktreePath: opts.WorktreePath,
		MainPath:     opts.MainPath,
	}
	_ = hooks.Fire(hooks.EventPostCreate, globalHookCtx)

	return nil
}
