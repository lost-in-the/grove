package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/cli"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/log"
	"github.com/LeahArmstrong/grove-cli/internal/output"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

var (
	toJSON bool
	toPeek bool
)

var toCmd = &cobra.Command{
	Use:     "to <name>",
	Aliases: []string{"switch"},
	Short:   "Switch to a worktree",
	Long: `Switch to a worktree by name. If a tmux session exists for the worktree, switch to it.
If no tmux session exists, create one.

Use --peek for a lightweight switch that skips hooks (no Docker side effects).
Useful for code review or quick file checks.

When using shell integration, this will also change your current directory.`,
	Args: cobra.ExactArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		name := args[0]
		stderr := cli.NewStderr()

		if name == "" {
			return fmt.Errorf("worktree name cannot be empty")
		}

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
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

		// Get current worktree for hook context and state update
		currentTree, _ := mgr.GetCurrent()
		var prevWorktree string
		var prevWorktreePath string
		if currentTree != nil {
			prevWorktree = currentTree.DisplayName()
			prevWorktreePath = currentTree.Path
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
		if !toPeek {
			if !toJSON {
				cli.Step(stderr, "Switching to '%s'...", name)
			}
			if err := hooks.Fire(hooks.EventPreSwitch, hookCtx); err != nil {
				cli.Warning(stderr, "pre-switch hooks failed: %v", err)
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
			tmuxMode = "auto"
		}
		// Agent mode: suppress tmux to prevent terminal takeover
		if cfg.AgentMode {
			tmuxMode = "off"
		}

		// Handle tmux session (unless mode is "off")
		var sessionName string
		var tmuxSwitched bool
		if tmuxMode != "off" && tmux.IsTmuxAvailable() {
			sessionName = worktree.TmuxSessionName(projectName, targetTree.DisplayName())
			exists, err := tmux.SessionExists(sessionName)
			if err != nil {
				return fmt.Errorf("failed to check session: %w", err)
			}

			if !exists {
				if err := tmux.CreateSession(sessionName, targetTree.Path); err != nil {
					return fmt.Errorf("failed to create session: %w", err)
				}
				if !toJSON {
					cli.Success(stderr, "Created tmux session '%s'", sessionName)
				}
			}

			if tmux.IsInsideTmux() {
				// Detect and correct directory drift before switching
				if exists {
					handleDirectoryDrift(sessionName, targetTree.Path, cfg.Tmux.OnSwitch, stderr)
				}
			} else if tmuxMode == "manual" && !toJSON {
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
		if !toPeek {
			if hooks.HasHooks(hooks.EventPostSwitch) {
				cli.Step(stderr, "Starting services...")
			}
			if err := hooks.Fire(hooks.EventPostSwitch, hookCtx); err != nil {
				cli.Warning(stderr, "post-switch hooks failed: %v", err)
			}
		}

		// JSON output mode
		if toJSON {
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
		if tmuxMode != "off" && sessionName != "" && tmux.IsInsideTmux() {
			if err := tmux.SwitchSession(sessionName); err != nil {
				return fmt.Errorf("failed to switch session: %w", err)
			}
			tmuxSwitched = true
		}

		// Skip cd directive when tmux switch already moved the user to the
		// target session — emitting cd: here would change the OLD session's
		// directory, not the one the user is now viewing.
		if !tmuxSwitched {
			if hasShellIntegration {
				// Shell wrapper will parse this and execute cd
				cli.Directive("cd", targetTree.Path)
				// In auto mode outside tmux, emit tmux-attach directive for shell wrapper
				if tmuxMode == "auto" && sessionName != "" {
					cli.Directive("tmux-attach", sessionName)
				}
			} else {
				cli.Faint(stderr, "Note: Directory switching requires shell integration.")
				cli.Faint(stderr, "Add this to your shell config (~/.zshrc or ~/.bashrc):")
				_, _ = fmt.Fprintf(stderr, "\n")
				cli.Faint(stderr, "  eval \"$(grove install zsh)\"   # for zsh")
				cli.Faint(stderr, "  eval \"$(grove install bash)\"  # for bash")
				_, _ = fmt.Fprintf(stderr, "\n")
				cli.Faint(stderr, "To change directory manually:")
				cli.Faint(stderr, "  cd %s", targetTree.Path)
				// In auto mode outside tmux without shell wrapper, attach directly
				if tmuxMode == "auto" && sessionName != "" {
					if err := tmux.AttachSession(sessionName); err != nil {
						return fmt.Errorf("failed to attach session: %w", err)
					}
				}
			}
		}

		return nil
	}),
}

// handleDirectoryDrift detects if a tmux session's active pane has drifted
// from the worktree root and corrects it based on the configured on_switch mode.
func handleDirectoryDrift(sessionName, worktreePath, onSwitch string, stderr *cli.Writer) {
	pane, err := tmux.GetPaneInfo(sessionName)
	if err != nil {
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
		_ = tmux.SendKeys(sessionName, fmt.Sprintf("cd %q", worktreePath))
	}
}

func init() {
	toCmd.Flags().BoolVarP(&toJSON, "json", "j", false, "Output as JSON with switch_to field")
	toCmd.Flags().BoolVar(&toPeek, "peek", false, "Lightweight switch: skip hooks (no Docker side effects)")
	rootCmd.AddCommand(toCmd)
}
