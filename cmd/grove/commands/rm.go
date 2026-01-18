package commands

import (
	"fmt"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:     "rm <name>",
	Aliases: []string{"remove", "delete"},
	Short:   "Remove a worktree and its tmux session",
	Long: `Remove a git worktree by name and kill its associated tmux session.
	
This will delete the worktree directory and remove the git worktree reference.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if name == "" {
			return fmt.Errorf("worktree name cannot be empty")
		}

		// Load config
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Kill tmux session if it exists
		if tmux.IsTmuxAvailable() {
			sessionName := cfg.Tmux.Prefix + name
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
		mgr, err := worktree.NewManager("")
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		if err := mgr.Remove(name); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}

		fmt.Printf("✓ Removed worktree '%s'\n", name)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(rmCmd)
}
