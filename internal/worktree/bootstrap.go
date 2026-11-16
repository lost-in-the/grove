package worktree

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/log"
	"github.com/lost-in-the/grove/internal/state"
)

// ErrRequiredHookFailed is returned by BootstrapWorktree when a required
// (on_failure="fail") post-create hook fails. Callers treat it as fatal (fail
// the command). File setup and state write failures are reported separately
// (ErrFileSetupIncomplete, ErrStateRegistration).
var ErrRequiredHookFailed = errors.New("required post-create hook failed")

// BootstrapOpts holds the inputs needed to bootstrap a worktree (whether
// freshly created via grove new or adopted post-hoc via grove adopt).
type BootstrapOpts struct {
	Name           string // short worktree name (e.g., "feature")
	Branch         string // branch the worktree is on
	WorktreePath   string // absolute path to the worktree directory
	MainPath       string // absolute path to the main worktree (parent of .grove)
	ProjectName    string // project name for hook context
	IsEnvironment  bool   // true for environment worktrees
	Mirror         string // mirror name when IsEnvironment is true
	ParentWorktree string // source worktree short name when this is a fork
}

// BootstrapWorktree runs the post-git-worktree-add bootstrap sequence:
//  1. Record grove's machine-local artifacts in the shared git exclude, so
//     fresh clones that never ran `grove init` on this machine still get
//     born-clean worktrees (config resolution needs no per-worktree files —
//     everything anchors at the main worktree via git's common dir)
//  2. Register the worktree in state.json (idempotent — re-registers on second call)
//  3. Copy/symlink external compose artifacts (SetupFiles) so plugin Up() sees them
//  4. Fire global plugin post-create hooks (docker container Up, etc.)
//  5. Run config-driven post-create hooks (hooks.toml — bundle install, etc.)
//
// Hook ordering matters: plugin Go hooks fire before config-driven hooks so
// containers are up by the time user setup commands (which may target them
// via docker:compose handlers) run.
//
// The returned error joins every problem, so callers tell them apart with
// errors.Is / errors.As: ErrStateRegistration (state write failed; file setup
// still ran), ErrFileSetupIncomplete (a *FileSetupError lists the entries not
// copied or linked), and ErrRequiredHookFailed. None of them roll the
// worktree back. File setup problems are not printed here: the caller reports
// them. Hook warnings go to w; pass nil for JSON-mode callers where stderr
// would corrupt machine-readable output, or for tests that don't assert on
// warning output. With nil, hook failures are still written to grove.log when
// GROVE_LOG=1 is set, but are otherwise invisible.
func BootstrapWorktree(stateMgr *state.Manager, cfg *config.Config, opts BootstrapOpts, w *cli.Writer) error {
	_, err := Bootstrap(stateMgr, cfg, opts, w)
	return err
}

// BootstrapResult reports what bootstrap did that callers may need to show.
type BootstrapResult struct {
	// Files has one entry per configured copy_files, symlink_files, or
	// symlink_dirs entry; empty when there is no external config.
	Files []FileResult
	// StateRegistered is false when state registration failed.
	StateRegistered bool
}

// ErrStateRegistration is matched (errors.Is) by the error Bootstrap returns
// when the worktree could not be registered in state.json.
var ErrStateRegistration = errors.New("register worktree")

// StateRegistrationError wraps the state write failure.
type StateRegistrationError struct{ Err error }

func (e *StateRegistrationError) Error() string { return "register worktree: " + e.Err.Error() }
func (e *StateRegistrationError) Unwrap() error { return e.Err }

// Is lets errors.Is(err, ErrStateRegistration) match.
func (e *StateRegistrationError) Is(target error) bool { return target == ErrStateRegistration }

// Bootstrap is BootstrapWorktree plus a report of the file setup results.
func Bootstrap(stateMgr *state.Manager, cfg *config.Config, opts BootstrapOpts, w *cli.Writer) (BootstrapResult, error) {
	var res BootstrapResult
	if opts.WorktreePath == "" || opts.MainPath == "" {
		return res, fmt.Errorf("WorktreePath and MainPath are required")
	}

	// Best-effort: worktrees function without the excludes, they just show
	// grove's machine-local files as untracked until `grove init` runs here.
	// (The one-time legacy-migration notice is handled by the command context,
	// which runs this same migration on every invocation — no need to surface
	// it from inside bootstrap.)
	if _, err := grove.EnsureGroveExcludes(opts.MainPath); err != nil {
		log.Printf("record git excludes: %v", err)
		if w != nil {
			cli.Warning(w, "record git excludes: %v", err)
		}
	}

	res.StateRegistered = true
	var stateErr error
	if stateMgr != nil {
		now := time.Now()
		wsState := &state.WorktreeState{
			Path:           opts.WorktreePath,
			Branch:         opts.Branch,
			Root:           false,
			CreatedAt:      now,
			LastAccessedAt: now,
			Environment:    opts.IsEnvironment,
			ParentWorktree: opts.ParentWorktree,
		}
		if opts.IsEnvironment {
			wsState.Mirror = opts.Mirror
			wsState.LastSyncedAt = &now
		}
		if err := stateMgr.AddWorktree(opts.Name, wsState); err != nil {
			res.StateRegistered = false
			stateErr = &StateRegistrationError{Err: err}
		}
	}

	// File setup (copy/symlink external compose artifacts) — must precede
	// plugin hooks so symlinked dirs exist before any container mount. It runs
	// even when state registration failed: the files are the part a seller
	// cannot work without, and `grove adopt` does not recopy them.
	var fileErr error
	if cfg != nil && cfg.Plugins.Docker.External != nil {
		res.Files = SetupFiles(cfg.Plugins.Docker.External, opts.WorktreePath, opts.MainPath)
		fileErr = NewFileSetupError(res.Files)
		if fileErr != nil {
			log.Printf("file setup: %v", fileErr)
		}
	}

	// Hooks assume a registered worktree, so a failed registration stops here.
	if stateErr != nil {
		return res, errors.Join(stateErr, fileErr)
	}

	// Global plugin post-create hook (e.g., docker external) — runs before
	// config-driven hooks so containers are up by the time user setup commands run.
	globalHookCtx := &hooks.Context{
		Worktree:     opts.Name,
		Config:       cfg,
		WorktreePath: opts.WorktreePath,
		MainPath:     opts.MainPath,
	}
	if err := hooks.Fire(hooks.EventPostCreate, globalHookCtx); err != nil {
		log.Printf("hooks: global post-create plugin hook failed: %v", err)
		if w != nil {
			cli.Warning(w, "post-create plugin hook failed: %v", err)
		}
	}

	// Per-project (config-driven) post-create hooks last — these may target
	// containers via docker:compose handlers and need them already running.
	// The main worktree's .grove dir is passed explicitly so hooks.toml is
	// found regardless of the caller's cwd (subdirectory, secondary worktree).
	hookExecutor, hookErr := hooks.NewExecutor(filepath.Join(opts.MainPath, ".grove"))
	if hookErr != nil {
		log.Printf("hooks: failed to load config during bootstrap: %v", hookErr)
		if w != nil {
			cli.Warning(w, "post-create hooks: failed to load config: %v", hookErr)
		}
	} else if hookExecutor.HasHooksForEvent(hooks.EventPostCreate) {
		// Hook output follows w (stdout for interactive callers, a capture
		// buffer for the TUI). With no writer it is DISCARDED rather than
		// falling through to the executor's os.Stdout default, which used to
		// corrupt `grove new --json`'s machine-readable stdout whenever a
		// post-create hook printed progress.
		var hookOut = io.Discard
		if w != nil {
			hookOut = w
		}
		hookCtx := &hooks.ExecutionContext{
			Event:    hooks.EventPostCreate,
			Worktree: opts.Name,
			// WorktreeFull is contractually the real directory name
			// ({{.worktree_full}} in hooks.toml). Derive it from the path —
			// the canonical {project}-{name} concatenation is wrong under a
			// custom [naming] pattern or for adopted directories, and every
			// other producer (rm, TUI) uses filepath.Base(wt.Path).
			WorktreeFull: filepath.Base(opts.WorktreePath),
			Branch:       opts.Branch,
			Project:      opts.ProjectName,
			MainPath:     opts.MainPath,
			NewPath:      opts.WorktreePath,
			Output:       hookOut,
		}
		if err := hookExecutor.Execute(hooks.EventPostCreate, hookCtx); err != nil {
			// A required (on_failure="fail") post-create hook failing fails the
			// operation (B7). The worktree already exists, so it is left in
			// place for inspection rather than rolled back; the non-zero exit
			// signals that provisioning did not complete. Non-required failures
			// are handled inside Execute (warn) and don't reach here.
			log.Printf("hooks: required post-create project hook failed: %v", err)
			if w != nil {
				cli.Warning(w, "required post-create hook failed (worktree kept): %v", err)
			}
			return res, errors.Join(fileErr, fmt.Errorf("%w: %v", ErrRequiredHookFailed, err))
		}
	}

	return res, fileErr
}
