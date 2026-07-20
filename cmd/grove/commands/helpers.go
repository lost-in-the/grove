package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/log"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/docker"
)

func isGitRepo(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func detectProjectName(dir string) string {
	output, err := cmdexec.Output(context.TODO(), "git", []string{"remote", "get-url", "origin"}, dir, cmdexec.GitLocal)
	if err != nil {
		return filepath.Base(dir)
	}

	url := strings.TrimSpace(string(output))
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return filepath.Base(dir)
	}

	name := parts[len(parts)-1]
	if idx := strings.LastIndex(name, ":"); idx != -1 {
		name = name[idx+1:]
	}
	if name != "" {
		return name
	}

	return filepath.Base(dir)
}

// detectMainBranch resolves the branch `grove init` records as
// default_base_branch. It delegates to git.DetectDefaultBranch — the same
// detector `grove rm`/`grove trim` use — so init and the branch-cleanup checks
// agree (origin/HEAD → init.defaultBranch → main/master → HEAD), instead of
// init's older main/master-only heuristic recording a different branch.
func detectMainBranch(dir string) string {
	if branch, err := git.DetectDefaultBranch(dir); err == nil && branch != "" {
		return branch
	}
	return "main"
}

// loadTmuxSessions returns the running tmux sessions keyed by name, or nil
// when tmux is unavailable or listing fails — tmuxStatusFor treats a nil map
// as "no session". Shared by `grove ls` and `grove here`.
func loadTmuxSessions() map[string]*tmux.Session {
	if !tmux.IsTmuxAvailable() {
		return nil
	}
	sessionList, err := tmux.ListSessions()
	if err != nil {
		return nil
	}
	sessions := make(map[string]*tmux.Session, len(sessionList))
	for _, s := range sessionList {
		sessions[s.Name] = s
	}
	return sessions
}

// emitCdOrExplain emits the cd: directive for the shell wrapper when shell
// integration is active; otherwise it explains how to set up shell
// integration and change directory manually. Shared by every command that
// lands the user in a different worktree without a tmux client switch.
func emitCdOrExplain(stderr *cli.Writer, path string) {
	// Prefers GROVE_CD_FILE when the wrapper set one (un-captured commands),
	// else a cd: line on stdout for capture-based commands.
	if cli.CdDirective(path) {
		return
	}
	cli.Faint(stderr, "Note: Directory switching requires shell integration.")
	cli.Faint(stderr, "Add this to your shell config (~/.zshrc or ~/.bashrc):")
	_, _ = fmt.Fprintf(stderr, "\n")
	cli.Faint(stderr, "  eval \"$(grove install zsh)\"   # for zsh")
	cli.Faint(stderr, "  eval \"$(grove install bash)\"  # for bash")
	_, _ = fmt.Fprintf(stderr, "\n")
	cli.Faint(stderr, "To change directory manually:")
	cli.Faint(stderr, "  cd %s", path)
}

// switchToWorktree runs the shared switch epilogue used by new/fork
// (`grove to` and `grove last` route through performSwitch instead):
// batches SetLastWorktree + TouchWorktree into a single state save, stores
// the current tmux session as "last", and switches the tmux client to the
// target session (creating it if missing). Returns whether the client was
// actually switched; callers emit the cd-directive fallback when it wasn't.
//
// suppressTmux must be true when the client must not be relocated (agent
// mode, --no-tmux, tmux mode "off") — compute it via effectiveTmuxMode.
func switchToWorktree(ctx *GroveContext, stderr *cli.Writer, prevName, targetName, sessionName, targetPath string, suppressTmux bool) bool {
	var tmuxSwitched bool
	batchErr := ctx.State.Batch(func() error {
		if prevName != "" {
			if err := ctx.State.SetLastWorktree(prevName); err != nil {
				log.Printf("failed to set last worktree %q: %v", prevName, err)
			}
		}

		// Store current session as last if inside tmux
		if tmux.IsInsideTmux() {
			if currentSession, err := tmux.GetCurrentSession(); err == nil {
				if err := tmux.StoreLastSession(currentSession); err != nil {
					log.Printf("failed to store last session %q: %v", currentSession, err)
				}
			}
		}

		// Switch tmux session, creating it first when missing (e.g. killed
		// by hand, or the worktree was entered with --no-tmux). Failures
		// degrade to the cd-directive fallback instead of aborting.
		if !suppressTmux && tmux.IsTmuxAvailable() && tmux.IsInsideTmux() {
			if exists, err := tmux.SessionExists(sessionName); err == nil && !exists {
				if createErr := tmux.CreateSession(sessionName, targetPath); createErr != nil {
					cli.Warning(stderr, "Failed to create tmux session: %v", createErr)
				}
			}
			if err := tmux.SwitchSession(sessionName); err != nil {
				cli.Warning(stderr, "Failed to switch session: %v", err)
			} else {
				tmuxSwitched = true
			}
		}

		// Update last_accessed_at for target worktree
		if err := ctx.State.TouchWorktree(targetName); err != nil {
			log.Printf("failed to touch worktree %q: %v", targetName, err)
		}
		return nil
	})
	if batchErr != nil {
		log.Printf("state save failed: %v", batchErr)
	}
	return tmuxSwitched
}

// currentWorktreeRoot resolves the root directory of the worktree containing
// the current working directory. Docker slot detection keys on the worktree
// directory basename (docker.FindWorktreeSlot), so container commands must
// pass the worktree root — a raw os.Getwd() from a subdirectory would miss
// the isolated stack and silently operate on the shared one instead.
func currentWorktreeRoot(ctx *GroveContext) (string, error) {
	mgr, err := ctx.WorktreeManager()
	if err != nil {
		return "", err
	}
	root, err := mgr.CurrentPath()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current worktree root: %w", err)
	}
	return root, nil
}

// removeWorktreeWithHooks runs the shared removal sequence used by `grove rm`
// and `grove trim`: user pre-remove hooks (hooks.toml), the plugin pre-remove
// hook (e.g. stop agent stacks), git worktree removal, state cleanup, and
// tmux session kill. Returns an error only when the git removal itself fails;
// ancillary failures are warned and skipped.
func removeWorktreeWithHooks(ctx *GroveContext, mgr *worktree.Manager, w *cli.Writer, projectName, name, wtPath, branchName string, force bool) error {
	// Execute pre-remove hooks (user hooks from hooks.toml). Pin resolution to
	// the main worktree's .grove — a no-arg NewExecutor() resolves from cwd and
	// would honor a linked worktree's own hooks.toml when `grove rm B` runs from
	// inside worktree A, diverging from the project's hooks (and from the TUI
	// delete path, which pins to main).
	hookExecutor, hookErr := hooks.NewExecutor(filepath.Join(ctx.ProjectRoot, ".grove"))
	if hookErr == nil && hookExecutor.HasHooksForEvent(hooks.EventPreRemove) {
		hookCtx := &hooks.ExecutionContext{
			Event:        hooks.EventPreRemove,
			Worktree:     name,
			Branch:       branchName,
			Project:      projectName,
			MainPath:     ctx.ProjectRoot,
			NewPath:      wtPath,
			WorktreeFull: filepath.Base(wtPath),
		}
		cli.Step(w, "Running pre-remove hooks...")
		// A required (on_failure="fail") pre-remove hook failing aborts the
		// removal before the worktree is touched (B7). Non-required failures
		// warn inside Execute and return nil.
		if err := hookExecutor.Execute(hooks.EventPreRemove, hookCtx); err != nil {
			return fmt.Errorf("required pre-remove hook failed: %w", err)
		}
	}

	// Fire plugin pre-remove hook (e.g., stop agent stacks)
	pluginHookCtx := &hooks.Context{
		Worktree:     name,
		Config:       ctx.Config,
		WorktreePath: wtPath,
		MainPath:     ctx.ProjectRoot,
	}
	if err := hooks.Fire(hooks.EventPreRemove, pluginHookCtx); err != nil {
		cli.Warning(w, "Pre-remove plugin hook failed: %v", err)
	}

	// Remove worktree — the critical step, done before tmux kill
	if err := mgr.Remove(name, force); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	// Remove from state
	if err := ctx.State.RemoveWorktree(name); err != nil {
		cli.Warning(w, "worktree removed but state cleanup failed: %v", err)
	}

	cli.Success(w, "Removed worktree '%s'", name)

	// Kill tmux session after worktree is confirmed gone
	if tmux.IsTmuxAvailable() {
		sessionName := worktree.TmuxSessionName(projectName, name)
		if exists, err := tmux.SessionExists(sessionName); err == nil && exists {
			if err := tmux.KillSession(sessionName); err != nil {
				cli.Warning(w, "Failed to kill tmux session: %v", err)
			} else {
				cli.Success(w, "Killed tmux session '%s'", sessionName)
			}
		}
	}

	return nil
}

// firePostRemoveHooks fires user post-remove hooks (hooks.toml) and the
// plugin post-remove hook for a worktree that was just removed. Kept separate
// from removeWorktreeWithHooks so `grove rm` can interleave branch deletion
// before the post-remove hooks, preserving its established ordering.
func firePostRemoveHooks(ctx *GroveContext, w *cli.Writer, projectName, name, wtPath, branchName string) {
	// Pin to the main worktree's .grove (see removeWorktreeWithHooks) so post-
	// remove cleanup resolves the project's hooks regardless of the cwd.
	hookExecutor, hookErr := hooks.NewExecutor(filepath.Join(ctx.ProjectRoot, ".grove"))
	if hookErr == nil && hookExecutor.HasHooksForEvent(hooks.EventPostRemove) {
		hookCtx := &hooks.ExecutionContext{
			Event:    hooks.EventPostRemove,
			Worktree: name,
			Branch:   branchName,
			Project:  projectName,
			MainPath: ctx.ProjectRoot,
			// Populate the removed worktree's path/name like pre-remove does.
			// Without WorktreeFull, `{{.worktree_full}}` interpolated to "" and
			// a cleanup like `rm -rf cache/{{.worktree_full}}` became
			// `rm -rf cache/` (B28). The directory is already gone, so
			// post_remove actions should use working_dir = "main".
			NewPath:      wtPath,
			WorktreeFull: filepath.Base(wtPath),
		}
		cli.Step(w, "Running post-remove hooks...")
		if err := hookExecutor.Execute(hooks.EventPostRemove, hookCtx); err != nil {
			cli.Warning(w, "Post-remove hook had errors: %v", err)
		}
	}

	pluginHookCtx := &hooks.Context{
		Worktree:     name,
		Config:       ctx.Config,
		WorktreePath: wtPath,
		MainPath:     ctx.ProjectRoot,
	}
	if err := hooks.Fire(hooks.EventPostRemove, pluginHookCtx); err != nil {
		cli.Warning(w, "Post-remove plugin hook failed: %v", err)
	}
}

// loadConfigHookExecutor loads the hooks.toml executor rooted at mainPath's
// .grove directory, sending hook progress to out. Returns nil (logged, not
// fatal) when the config can't be loaded — callers skip hooks in that case.
// Load once per command and reuse across events: every construction re-reads
// global + project hooks.toml from disk.
func loadConfigHookExecutor(out *cli.Writer, mainPath string) *hooks.Executor {
	executor, err := hooks.NewExecutor(filepath.Join(mainPath, ".grove"))
	if err != nil {
		log.Printf("hooks: failed to load config: %v", err)
		return nil
	}
	if out != nil {
		executor.Output = out
	}
	return executor
}

// runConfigHooksWith fires one lifecycle event's hooks.toml actions on a
// previously loaded executor (nil-safe — a nil executor means the config
// failed to load and hooks are skipped).
//
// Returns a non-nil error only when a required (on_failure="fail") action
// failed, so callers can abort the operation (B7); a non-required hook
// failure warns inside Execute and returns nil.
// The caller passes a *hooks.ExecutionContext built with named fields rather
// than a run of same-typed positional strings — a transposed pair there
// compiled silently and mis-filled hook variables. Event is stamped here, and
// WorktreeFull is derived from NewPath when the caller left it unset.
func runConfigHooksWith(executor *hooks.Executor, event string, ec *hooks.ExecutionContext) error {
	if executor == nil || !executor.HasHooksForEvent(event) {
		return nil
	}
	ec.Event = event
	if ec.WorktreeFull == "" && ec.NewPath != "" {
		ec.WorktreeFull = filepath.Base(ec.NewPath)
	}
	return executor.Execute(event, ec)
}

// runConfigHooks executes the user's hooks.toml actions for a lifecycle event —
// the config-file counterpart to the plugin hooks.Fire registry. Wiring this in
// is what makes documented pre_switch / post_switch / pre_create recipes
// actually run; before it, those events silently did nothing (B6).
//
// out receives the hooks' own progress lines — route it to stderr on the switch
// path so hook output never lands on the cd: stdout channel the shell wrapper
// parses. mainPath is the project root, whose .grove holds hooks.toml.
//
// Loads the hooks config on every call; commands firing multiple events
// should load once with loadConfigHookExecutor and use runConfigHooksWith.
func runConfigHooks(out *cli.Writer, event string, ec *hooks.ExecutionContext) error {
	return runConfigHooksWith(loadConfigHookExecutor(out, ec.MainPath), event, ec)
}

// worktreeSetupOpts configures the post-create setup sequence.
type worktreeSetupOpts struct {
	IsEnvironment bool
	Mirror        string
	NoDocker      bool
	JSONOutput    bool
}

// setupCreatedWorktree runs the shared post-create sequence: find the worktree,
// record git excludes, register state, execute hooks, and auto-start Docker.
func setupCreatedWorktree(ctx *GroveContext, mgr *worktree.Manager, name, branchName string, opts worktreeSetupOpts, w *cli.Writer) (*worktree.Worktree, error) {
	// Compute the canonical path directly. The worktree was just created by
	// the caller at the standard location, so re-running List() to find it
	// would only burn another N parallel git status calls.
	wtPath := mgr.PathForName(name)
	if _, err := os.Stat(wtPath); err != nil {
		return nil, fmt.Errorf("created worktree not found at %s: %w", wtPath, err)
	}
	wt := &worktree.Worktree{
		Name:      filepath.Base(wtPath),
		Path:      wtPath,
		Branch:    branchName,
		ShortName: name,
	}

	if !opts.JSONOutput {
		cli.Step(w, "Bootstrapping worktree...")
	}

	bootstrapOpts := worktree.BootstrapOpts{
		Name:          name,
		Branch:        branchName,
		WorktreePath:  wt.Path,
		MainPath:      ctx.ProjectRoot,
		ProjectName:   mgr.GetProjectName(),
		IsEnvironment: opts.IsEnvironment,
		Mirror:        opts.Mirror,
	}
	var bootstrapWriter *cli.Writer
	if !opts.JSONOutput {
		bootstrapWriter = w
	}
	if err := worktree.BootstrapWorktree(ctx.State, ctx.Config, bootstrapOpts, bootstrapWriter); err != nil {
		// A required post-create hook failing fails the command (B7). Other
		// bootstrap failures (symlink, state) are recoverable — warn and point
		// at `grove repair` rather than aborting, preserving prior behavior.
		if errors.Is(err, worktree.ErrRequiredHookFailed) {
			return wt, err
		}
		if !opts.JSONOutput {
			cli.Warning(w, "Bootstrap failed: %v", err)
			cli.Faint(w, "run 'grove repair' to fix")
		}
	}

	autoStartDocker(w, ctx.Config, wt.Path, opts.NoDocker, opts.JSONOutput)
	return wt, nil
}

// runFileSetup runs worktree.SetupFiles when an external docker config is
// present. Kept as a separate helper so callers like `grove fork` (which
// don't go through BootstrapWorktree) can reuse it.
func runFileSetup(cfg *config.Config, newPath, mainPath string, w *cli.Writer, jsonOutput bool) {
	if cfg == nil || cfg.Plugins.Docker.External == nil {
		return
	}
	if err := worktree.SetupFiles(cfg.Plugins.Docker.External, newPath, mainPath); err != nil {
		if !jsonOutput {
			cli.Warning(w, "File setup had issues: %v", err)
		}
	}
}

// autoStartDocker starts the Docker stack for a new worktree if configured.
func autoStartDocker(w *cli.Writer, cfg *config.Config, wtPath string, noDocker, jsonOutput bool) {
	if noDocker || !shouldAutoDocker(cfg) {
		return
	}
	if !jsonOutput {
		cli.Step(w, "Starting Docker stack...")
	}
	dockerPlugin := docker.New()
	if cfg.AgentMode {
		dockerPlugin.SetIsolated(true)
	}
	if err := dockerPlugin.Init(cfg); err != nil {
		if !jsonOutput {
			cli.Warning(w, "Docker init failed: %v", err)
		}
		return
	}
	if !dockerPlugin.Enabled() {
		return
	}
	if err := dockerPlugin.Up(wtPath, true); err != nil {
		if !jsonOutput {
			cli.Warning(w, "Docker auto-start failed: %v", err)
		}
	} else if !jsonOutput {
		cli.Success(w, "Docker stack started")
	}
}

// shouldAutoDocker returns true when Docker should be auto-started on grove new.
// Enabled by default when agent stacks are configured, or explicitly via auto_up.
func shouldAutoDocker(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}

	// Explicit auto_up setting takes precedence
	if cfg.Plugins.Docker.AutoUp != nil {
		return *cfg.Plugins.Docker.AutoUp
	}

	// Default: auto-start when agent stacks are configured and enabled
	if cfg.IsExternalDockerMode() {
		ext := cfg.Plugins.Docker.External
		if ext != nil && ext.Agent != nil && ext.Agent.Enabled != nil && *ext.Agent.Enabled {
			return true
		}
	}

	return false
}
