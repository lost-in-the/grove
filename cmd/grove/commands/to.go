package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/log"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/worktree"
)

var (
	toJSON   bool
	toPeek   bool
	toNoTmux bool
)

var toCmd = &cobra.Command{
	Use:     "to [name]",
	Aliases: []string{"switch", "t"},
	Short:   "Switch to a worktree",
	Long: `Switch to a worktree by name. If a tmux session exists for the worktree, switch to it.
If no tmux session exists, create one.

Use --peek for a lightweight switch that skips hooks and tmux entirely
(no Docker side effects, no tmux client relocation). Useful for code review
or quick file checks.

Use --no-tmux to switch without creating, switching, or attaching tmux
sessions — hooks still fire. Useful for automation running inside an
existing tmux session.

When using shell integration, this will also change your current directory.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeWorktreeNames,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		var name string
		if len(args) == 0 {
			selected, err := selectWorktree(ctx, "Switch to which worktree?")
			if err != nil {
				return err
			}
			name = selected
		} else {
			name = args[0]
		}

		if name == "" {
			return fmt.Errorf("worktree name cannot be empty")
		}

		return performSwitch(ctx, name, toJSON, toPeek, toNoTmux)
	}),
}

// performSwitch executes the full switch-to-worktree flow shared by `grove to`
// and `grove last`: resolve the target, honor dirty handling, fire pre/post
// switch hooks, manage the tmux session, and emit the cd/attach directives.
// The JSON result is emitted before the tmux client is relocated, so a machine
// caller (--json) never has its terminal moved (B20); sharing this flow also
// gives `grove last` the hooks and dirty handling it previously skipped (B19).
// jsonOut/peek/noTmux are the per-invocation flags.
func performSwitch(ctx *GroveContext, name string, jsonOut, peek, noTmux bool) error {
	stderr := cli.NewStderr()

	mgr, err := ctx.WorktreeManager()
	if err != nil {
		return err
	}

	// Find worktree by short name or full name
	targetTree, err := mgr.Find(name)
	if err != nil {
		return fmt.Errorf("failed to find worktree: %w", err)
	}
	if targetTree == nil {
		return fmt.Errorf("worktree '%s' not found", name)
	}

	// Check if worktree is stale (directory missing)
	if targetTree.IsPrunable {
		return fmt.Errorf("worktree '%s' is stale (directory missing). Run 'grove rm %s' to clean up", name, name)
	}

	// Resolve the current worktree once (used for the already-here check,
	// hook context, and state update).
	currentPath, _ := mgr.CurrentPath()

	// Already in the target worktree — no-op (spec: "Already in X", exit 0).
	// Without this, a self-switch runs the dirty gate against the worktree
	// you're standing in (refusing a no-op move when dirty_handling="refuse")
	// and mis-records last_worktree as the current one, breaking the A↔B
	// toggle (B18).
	if currentPath != "" && currentPath == targetTree.Path {
		if jsonOut {
			return output.PrintJSON(output.SwitchResult{
				SwitchTo: targetTree.Path,
				Name:     targetTree.DisplayName(),
				Branch:   targetTree.Branch,
				Path:     targetTree.Path,
			})
		}
		if !peek {
			cli.Info(stderr, "Already in '%s'", targetTree.DisplayName())
		}
		return nil
	}

	// Get current worktree for hook context and state update. Path +
	// display name are all that's needed — the WIP handler does its own
	// per-path dirty check below.
	var prevWorktree string
	var prevWorktreePath string
	if currentPath != "" {
		prevWorktree = mgr.DisplayNameForPath(currentPath)
		prevWorktreePath = currentPath

		// Check for dirty worktree before allowing switch
		wip := worktree.NewWIPHandler(currentPath)
		hasDirty, wipErr := wip.HasWIP()
		if wipErr != nil {
			log.Printf("failed to check dirty state: %v", wipErr)
			// Treat check failure as clean to avoid blocking the user
		}
		action := worktree.ResolveDirtyAction(ctx.Config.Switch.DirtyHandling, hasDirty, peek, cli.IsInteractive())
		switch action {
		case worktree.DirtyRefuse:
			files, _ := wip.ListWIPFiles()
			msg := fmt.Sprintf("worktree '%s' has uncommitted changes", prevWorktree)
			if len(files) > 0 {
				msg += ":\n  " + strings.Join(files, "\n  ")
			}
			msg += "\n\nCommit or stash your changes, or set dirty_handling = \"auto-stash\" in .grove/config.toml"
			return fmt.Errorf("%s", msg)
		case worktree.DirtyStash:
			stashMsg := fmt.Sprintf("grove: auto-stash before switch to %s", name)
			if stashErr := wip.Stash(stashMsg); stashErr != nil {
				return fmt.Errorf("failed to auto-stash changes: %w", stashErr)
			}
			if !jsonOut {
				cli.Success(stderr, "Stashed changes in '%s'", prevWorktree)
			}
		case worktree.DirtyPrompt:
			files, _ := wip.ListWIPFiles()
			details := files
			if len(details) == 0 {
				details = []string{"(uncommitted changes detected)"}
			}
			confirmed, promptErr := cli.ConfirmWithDetails(
				stderr,
				fmt.Sprintf("Worktree '%s' has uncommitted changes:", prevWorktree),
				details,
				"Switch anyway?",
				false,
			)
			if promptErr != nil || !confirmed {
				return fmt.Errorf("switch aborted: worktree has uncommitted changes")
			}
		}

		// Update last_worktree in state before switching
		if err := ctx.State.SetLastWorktree(prevWorktree); err != nil {
			log.Printf("failed to set last worktree %q: %v", prevWorktree, err)
		}
	}

	// Build hook context (used by pre/post-switch hooks unless --peek)
	hookCtx := &hooks.Context{
		Worktree:         name,
		PrevWorktree:     prevWorktree,
		Config:           ctx.Config,
		WorktreePath:     targetTree.Path,
		PrevWorktreePath: prevWorktreePath,
		MainPath:         ctx.ProjectRoot,
	}

	// Fire pre-switch hooks (skip when --peek)
	if !peek {
		if !jsonOut {
			cli.Step(stderr, "Switching to '%s'...", name)
		}
		if err := hooks.Fire(hooks.EventPreSwitch, hookCtx); err != nil {
			cli.Warning(stderr, "pre-switch hooks failed: %v", err)
		}
		// Config-file (hooks.toml) pre-switch actions. Output to stderr so it
		// never collides with the cd: directive on stdout. A required
		// action failing aborts the switch before anything changes (B7).
		if err := runConfigHooks(stderr, hooks.EventPreSwitch, mgr.GetProjectName(), name, targetTree.Branch, targetTree.Path, prevWorktreePath, ctx.ProjectRoot); err != nil {
			return err
		}
	}

	// Store current session as last if inside tmux
	if tmux.IsInsideTmux() {
		currentSession, err := tmux.GetCurrentSession()
		if err == nil {
			if err := tmux.StoreLastSession(currentSession); err != nil {
				log.Printf("failed to store last session %q: %v", currentSession, err)
			}
		}
	}

	projectName := mgr.GetProjectName()
	cfg := ctx.Config
	tmuxMode := cfg.Tmux.Mode
	if tmuxMode == "" {
		tmuxMode = tmuxModeAuto
	}
	useCC := tmux.ShouldUseControlMode(cfg.Tmux.ControlMode)
	tmuxMode = effectiveTmuxMode(tmuxMode, cfg.AgentMode, noTmux, peek)

	// Handle tmux session (unless mode is "off")
	var sessionName string
	var tmuxSwitched bool
	if tmuxMode != tmuxModeOff && tmux.IsTmuxAvailable() {
		sessionName = worktree.TmuxSessionName(projectName, targetTree.DisplayName())
		exists, err := tmux.SessionExists(sessionName)
		if err != nil {
			return fmt.Errorf("failed to check session: %w", err)
		}

		if !exists {
			if err := tmux.CreateSession(sessionName, targetTree.Path); err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}
			if !jsonOut {
				cli.Success(stderr, "Created tmux session '%s'", sessionName)
			}
		}

		if tmux.IsInsideTmux() {
			// Detect and correct directory drift before switching
			if exists {
				handleDirectoryDrift(sessionName, targetTree.Path, cfg.Tmux.OnSwitch, stderr)
			}
		} else if tmuxMode == "manual" && !jsonOut {
			cli.Success(stderr, "Tmux session '%s' ready", sessionName)
			cli.Faint(stderr, "Run: tmux attach -t %s", sessionName)
		}
		// auto mode outside tmux: handled below via shell directive or direct attach
	}

	// Update last_accessed_at for target worktree
	if err := ctx.State.TouchWorktree(targetTree.DisplayName()); err != nil {
		log.Printf("failed to touch worktree %q: %v", targetTree.DisplayName(), err)
	}

	// Fire post-switch hooks (Docker start, etc.) BEFORE the tmux switch
	// so the user sees Docker progress in the current session. After the
	// tmux switch the old session's stderr is no longer visible.
	// Also fire before the JSON return so machine consumers get hooks too.
	if !peek {
		if hooks.HasHooks(hooks.EventPostSwitch) {
			cli.Step(stderr, "Starting services...")
		}
		if err := hooks.Fire(hooks.EventPostSwitch, hookCtx); err != nil {
			cli.Warning(stderr, "post-switch hooks failed: %v", err)
		}
		// Config-file (hooks.toml) post-switch actions run after plugin
		// hooks so a docker:compose action sees a started stack (e.g. the
		// documented `bin/rails db:migrate` recipe). Output to stderr. A
		// required action failing fails the command (B7).
		if err := runConfigHooks(stderr, hooks.EventPostSwitch, mgr.GetProjectName(), name, targetTree.Branch, targetTree.Path, prevWorktreePath, ctx.ProjectRoot); err != nil {
			return err
		}
	}

	// JSON output mode
	if jsonOut {
		result := output.SwitchResult{
			SwitchTo: targetTree.Path,
			Name:     targetTree.DisplayName(),
			Branch:   targetTree.Branch,
			Path:     targetTree.Path,
		}
		return output.PrintJSON(result)
	}

	// Output directory change command for shell integration
	hasShellIntegration := os.Getenv("GROVE_SHELL") == "1"

	// Now perform the tmux session switch (if inside tmux)
	if tmuxMode != tmuxModeOff && sessionName != "" && tmux.IsInsideTmux() {
		if err := tmux.SwitchSession(sessionName); err != nil {
			return fmt.Errorf("failed to switch session: %w", err)
		}
		tmuxSwitched = true
	}

	// Skip cd directive when tmux switch already moved the user to the
	// target session — emitting cd: here would change the OLD session's
	// directory, not the one the user is now viewing.
	if !tmuxSwitched {
		emitCdOrExplain(stderr, targetTree.Path)
		// In auto mode outside tmux: emit the tmux-attach directive for
		// the shell wrapper, or attach directly without it.
		if tmuxMode == tmuxModeAuto && sessionName != "" {
			if hasShellIntegration {
				cli.TmuxAttachDirective(sessionName, useCC)
			} else {
				var attachErr error
				if useCC {
					attachErr = tmux.AttachSessionControlMode(sessionName)
				} else {
					attachErr = tmux.AttachSession(sessionName)
				}
				if attachErr != nil {
					return fmt.Errorf("failed to attach session: %w", attachErr)
				}
			}
		}
	}

	return nil
}

// effectiveTmuxMode returns the tmux mode after per-invocation overrides.
// Agent mode suppresses tmux to prevent terminal takeover; --no-tmux does the
// same for a single invocation without implying agent Docker isolation; --peek
// is observational and must never relocate the caller's tmux client (#105).
func effectiveTmuxMode(mode string, agentMode, noTmux, peek bool) string {
	if agentMode || noTmux || peek {
		return tmuxModeOff
	}
	return mode
}

// handleDirectoryDrift detects if a tmux session's active pane has drifted
// from the worktree root and corrects it based on the configured on_switch mode.
func handleDirectoryDrift(sessionName, worktreePath, onSwitch string, stderr *cli.Writer) {
	pane, err := tmux.GetPaneInfo(sessionName)
	if err != nil {
		log.Printf("failed to get pane info for drift check on %q: %v", sessionName, err)
		return
	}

	if pane.CurrentPath == worktreePath {
		return
	}

	if !pane.IsShell() {
		return
	}

	switch onSwitch {
	case "warn":
		cli.Warning(stderr, "session directory drifted from %s", worktreePath)
	case "ignore":
		// Do nothing
	default: // "reset" or ""
		// Single-quote for shell safety. The path derives from the repo's
		// parent directory plus the naming pattern (and externally created
		// worktrees can live anywhere), so it may legally contain single
		// quotes — escape them with the standard '\'' idiom.
		if err := tmux.SendKeys(sessionName, "cd "+shellSingleQuote(worktreePath)); err != nil {
			log.Printf("failed to reset directory in session %q: %v", sessionName, err)
		}
	}
}

// shellSingleQuote wraps s in single quotes for safe interpolation into a
// POSIX shell command line, escaping embedded single quotes with '\”.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func init() {
	toCmd.Flags().BoolVarP(&toJSON, "json", "j", false, "Output as JSON with switch_to field")
	toCmd.Flags().BoolVar(&toPeek, "peek", false, "Lightweight switch: skip hooks and tmux (no Docker or session side effects)")
	toCmd.Flags().BoolVar(&toNoTmux, "no-tmux", false, "Skip tmux session creation/switch/attach for this invocation")
	rootCmd.AddCommand(toCmd)
}
