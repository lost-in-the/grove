package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/cli"
	"github.com/LeahArmstrong/grove-cli/internal/log"
	"github.com/LeahArmstrong/grove-cli/internal/output"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

var attachJSON bool

var attachCmd = &cobra.Command{
	Use:     "attach [name]",
	Aliases: []string{"a"},
	Short:   "Attach to a tmux session for a worktree",
	Long: `Attach to or create a tmux session for a worktree without changing the
current shell directory. If no name is given, uses the current worktree.

This is a tmux-only command — it does not emit cd: directives.`,
	Args: cobra.MaximumNArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		stderr := cli.NewStderr()

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
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
			tmuxMode = "auto"
		}
		useCC := tmux.ShouldUseControlMode(cfg.Tmux.ControlMode)

		if tmuxMode == "off" {
			return fmt.Errorf("tmux is disabled in grove configuration (mode: off)")
		}

		if !tmux.IsTmuxAvailable() {
			return fmt.Errorf("tmux is not available")
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
		sessionName := worktree.TmuxSessionName(projectName, targetTree.DisplayName())

		created := false
		exists, err := tmux.SessionExists(sessionName)
		if err != nil {
			return fmt.Errorf("failed to check session: %w", err)
		}

		if !exists {
			if err := tmux.CreateSession(sessionName, targetTree.Path); err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}
			created = true
			if !attachJSON {
				cli.Success(stderr, "Created tmux session '%s'", sessionName)
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

		if tmux.IsInsideTmux() {
			// Inside tmux: switch-client
			if err := tmux.SwitchSession(sessionName); err != nil {
				return fmt.Errorf("failed to switch session: %w", err)
			}
		} else {
			hasShellIntegration := os.Getenv("GROVE_SHELL") == "1"
			if hasShellIntegration {
				// Emit tmux-attach directive for shell wrapper
				cli.TmuxAttachDirective(sessionName, useCC)
			} else {
				// No shell integration: attach directly (blocks, takes over terminal)
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

		return nil
	}),
}

func init() {
	attachCmd.Flags().BoolVarP(&attachJSON, "json", "j", false, "Output as JSON")
	rootCmd.AddCommand(attachCmd)
}
