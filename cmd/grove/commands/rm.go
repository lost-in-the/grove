package commands

import (
	"fmt"
	"os"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	rmForce     bool
	rmUnprotect bool
	rmDryRun    bool
)

var rmCmd = &cobra.Command{
	Use:     "rm <name>",
	Aliases: []string{"remove", "delete"},
	Short:   "Remove a worktree and its tmux session",
	Long: `Remove a git worktree by name and kill its associated tmux session.

This will delete the worktree directory and remove the git worktree reference.

Protected worktrees (configured in config.toml) require both --force and --unprotect flags.
Environment worktrees are implicitly protected.

Examples:
  grove rm feature-auth           # Remove regular worktree
  grove rm staging --force --unprotect  # Remove protected worktree`,
	Args: cobra.ExactArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		name := args[0]
		if name == "" {
			return fmt.Errorf("worktree name cannot be empty")
		}

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		// Load config for protection settings
		cfg, _ := config.Load()

		// Check protection status
		isProtectedByConfig := cfg != nil && cfg.IsProtected(name)
		isEnvironment := false
		ws, _ := ctx.State.GetWorktree(name)
		if ws != nil {
			isEnvironment = ws.Environment
		}

		isProtected := isProtectedByConfig || isEnvironment

		if isProtected {
			if !rmForce || !rmUnprotect {
				reason := "protected in config.toml"
				if isEnvironment {
					reason = "environment worktree"
				}
				fmt.Fprintf(os.Stderr, "Error: worktree '%s' is protected (%s)\n", name, reason)
				fmt.Fprintf(os.Stderr, "To remove, use: grove rm %s --force --unprotect\n", name)
				os.Exit(exitcode.CannotRemove)
			}
			if !rmDryRun {
				fmt.Printf("⚠ Removing protected worktree '%s'\n", name)
			}
		}

		// Dry run - just show what would happen
		if rmDryRun {
			fmt.Printf("Would remove worktree '%s'\n", name)
			// Get path from worktree manager
			wt, _ := mgr.Find(name)
			if wt != nil {
				fmt.Printf("  Path: %s\n", wt.Path)
			}
			if tmux.IsTmuxAvailable() {
				sessionName := worktree.TmuxSessionName(mgr.GetProjectName(), name)
				exists, _ := tmux.SessionExists(sessionName)
				if exists {
					fmt.Printf("  Would kill tmux session: %s\n", sessionName)
				}
			}
			return nil
		}

		projectName := mgr.GetProjectName()

		// Kill tmux session if it exists
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			exists, err := tmux.SessionExists(sessionName)
			if err == nil && exists {
				if err := tmux.KillSession(sessionName); err != nil {
					fmt.Printf("⚠ Failed to kill tmux session: %v\n", err)
				} else {
					fmt.Printf("✓ Killed tmux session '%s'\n", sessionName)
				}
			}
		}

		// Remove worktree
		if err := mgr.Remove(name); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}

		// Remove from state
		_ = ctx.State.RemoveWorktree(name)

		fmt.Printf("✓ Removed worktree '%s'\n", name)

		return nil
	}),
}

func init() {
	rmCmd.Flags().BoolVarP(&rmForce, "force", "f", false, "Force removal (required with --unprotect for protected worktrees)")
	rmCmd.Flags().BoolVar(&rmUnprotect, "unprotect", false, "Allow removing protected worktrees (requires --force)")
	rmCmd.Flags().BoolVar(&rmDryRun, "dry-run", false, "Show what would be removed without making changes")
	rootCmd.AddCommand(rmCmd)
}
