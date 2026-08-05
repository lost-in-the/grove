package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/log"
	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/worktree"
)

var attachJSON bool

var attachCmd = &cobra.Command{
	Use:     "join [name]",
	Aliases: []string{"attach", "a", "j"},
	Short:   "Join a tmux session for a worktree",
	Long: `Join (attach to) or create a tmux session for a worktree without changing the
current shell directory. If no name is given, uses the current worktree.

This is a tmux-only command — it does not emit cd: directives.`,
	Args: cobra.MaximumNArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		stderr := cli.NewStderr()

		mgr, err := ctx.WorktreeManager()
		if err != nil {
			return err
		}

		var targetTree *worktree.Worktree
		if len(args) == 1 {
			name := args[0]
			if name == "" {
				return fmt.Errorf("worktree name cannot be empty")
			}
			targetTree, err = mgr.Find(name)
			if err != nil {
				return fmt.Errorf("failed to find worktree: %w", err)
			}
			if targetTree == nil {
				return fmt.Errorf("worktree '%s' not found", name)
			}
		} else {
			targetTree, err = mgr.GetCurrent()
			if err != nil {
				return fmt.Errorf("failed to get current worktree: %w", err)
			}
			if targetTree == nil {
				return fmt.Errorf("not in a grove worktree")
			}
		}

		if targetTree.IsPrunable {
			displayName := targetTree.DisplayName()
			return fmt.Errorf("worktree '%s' is stale (directory missing). Run 'grove rm %s' to clean up", displayName, displayName)
		}

		cfg := ctx.Config
		tmuxMode := cfg.Tmux.Mode
		if tmuxMode == "" {
			tmuxMode = tmuxModeAuto
		}
		m := ctx.Mux()

		if tmuxMode == tmuxModeOff || m.Backend() == mux.BackendOff {
			return fmt.Errorf("session management is disabled in grove configuration")
		}

		if !m.Available() {
			return fmt.Errorf("%s is not available", m.Backend())
		}

		// Store current session as last if inside a multiplexer
		if m.Inside() {
			currentSession, err := m.Current()
			if err == nil {
				if err := mux.StoreLastSession(currentSession); err != nil {
					log.Printf("failed to store last session %q: %v", currentSession, err)
				}
			}
		}

		projectName := mgr.GetProjectName()
		target := muxTarget(projectName, targetTree.DisplayName(), targetTree.Path)
		sessionName := target.Name

		created := false
		exists, err := m.Exists(target)
		if err != nil {
			return fmt.Errorf("failed to check session: %w", err)
		}

		if !exists {
			if err := m.Ensure(target); err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}
			created = true
			if !attachJSON {
				cli.Success(stderr, "Created %s session '%s'", m.Backend(), sessionName)
			}
		}

		// Update last_accessed_at for target worktree
		if err := ctx.State.TouchWorktree(targetTree.DisplayName()); err != nil {
			log.Printf("failed to touch worktree %q: %v", targetTree.DisplayName(), err)
		}

		// JSON output mode
		if attachJSON {
			result := output.AttachResult{
				Name:    targetTree.DisplayName(),
				Session: sessionName,
				Path:    targetTree.Path,
				Created: created,
			}
			return output.PrintJSON(result)
		}

		if m.Inside() {
			// Already inside: relocate the existing client
			if err := m.Switch(target); err != nil {
				return fmt.Errorf("failed to switch session: %w", err)
			}
		} else {
			// With shell integration this hands off to the wrapper; otherwise
			// it attaches directly and takes over the terminal.
			hasShellIntegration := os.Getenv("GROVE_SHELL") == "1"
			if err := attachToSession(m, target, cfg.Tmux.ControlMode, hasShellIntegration); err != nil {
				return fmt.Errorf("failed to attach session: %w", err)
			}
		}

		return nil
	}),
}

func init() {
	attachCmd.Flags().BoolVarP(&attachJSON, "json", "j", false, "Output as JSON")
	rootCmd.AddCommand(attachCmd)
}
