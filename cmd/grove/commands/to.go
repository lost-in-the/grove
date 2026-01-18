package commands

import (
	"fmt"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var toCmd = &cobra.Command{
	Use:     "to <name>",
	Aliases: []string{"switch"},
	Short:   "Switch to a worktree",
	Long: `Switch to a worktree by name. If a tmux session exists for the worktree, switch to it.
If no tmux session exists, create one.

When using shell integration, this will also change your current directory.`,
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

		// Get worktree
		mgr, err := worktree.NewManager("")
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		trees, err := mgr.List()
		if err != nil {
			return fmt.Errorf("failed to list worktrees: %w", err)
		}

		var targetTree *worktree.Worktree
		for _, tree := range trees {
			if tree.Name == name {
				targetTree = tree
				break
			}
		}

		if targetTree == nil {
			return fmt.Errorf("worktree '%s' not found", name)
		}

		// Store current session as last if inside tmux
		if tmux.IsInsideTmux() {
			currentSession, err := tmux.GetCurrentSession()
			if err == nil {
				tmux.StoreLastSession(currentSession)
			}
		}

		// Handle tmux session
		if tmux.IsTmuxAvailable() {
			sessionName := cfg.Tmux.Prefix + name
			exists, err := tmux.SessionExists(sessionName)
			if err != nil {
				return fmt.Errorf("failed to check session: %w", err)
			}

			if !exists {
				// Create session if it doesn't exist
				if err := tmux.CreateSession(sessionName, targetTree.Path); err != nil {
					return fmt.Errorf("failed to create session: %w", err)
				}
				fmt.Printf("✓ Created tmux session '%s'\n", sessionName)
			}

			// Switch or attach to session
			if tmux.IsInsideTmux() {
				if err := tmux.SwitchSession(sessionName); err != nil {
					return fmt.Errorf("failed to switch session: %w", err)
				}
			} else {
				fmt.Printf("✓ Tmux session '%s' ready\n", sessionName)
				fmt.Printf("Run: tmux attach -t %s\n", sessionName)
			}
		}

		// Output directory change command for shell integration
		fmt.Printf("cd:%s\n", targetTree.Path)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(toCmd)
}
