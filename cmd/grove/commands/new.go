package commands

import (
	"fmt"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new worktree and tmux session",
	Long: `Create a new git worktree with the specified name and create a tmux session for it.
	
The worktree will be created in the parent directory of the current repository.
A new branch with the same name will be created automatically.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if name == "" {
			return fmt.Errorf("worktree name cannot be empty")
		}

		// Load config for tmux prefix
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Create worktree
		mgr, err := worktree.NewManager("")
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		// Use name as branch name
		branchName := name
		if err := mgr.Create(name, branchName); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}

		fmt.Printf("✓ Created worktree '%s'\n", name)

		// Create tmux session if tmux is available
		if tmux.IsTmuxAvailable() {
			trees, err := mgr.List()
			if err != nil {
				return fmt.Errorf("failed to get worktree path: %w", err)
			}

			var wtPath string
			for _, tree := range trees {
				if tree.Name == name {
					wtPath = tree.Path
					break
				}
			}

			if wtPath != "" {
				sessionName := cfg.Tmux.Prefix + name
				if err := tmux.CreateSession(sessionName, wtPath); err != nil {
					fmt.Printf("⚠ Failed to create tmux session: %v\n", err)
				} else {
					fmt.Printf("✓ Created tmux session '%s'\n", sessionName)
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}
