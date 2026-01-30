package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LeahArmstrong/grove-cli/internal/grove"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tui"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

var rootCmd = &cobra.Command{
	Use:   "grove",
	Short: "Zero-friction worktree management",
	Long: `Grove is a zero-friction worktree + tmux manager for developers.

It provides fast context switching between git worktrees with automatic
tmux session management. Every command completes in less than 500ms.

GETTING STARTED: Add shell integration to your ~/.zshrc or ~/.bashrc:
  eval "$(grove install zsh)"   # or bash

This enables directory switching, tab completion, and the 'w' alias.
Use 'grove install --help' for details.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Only launch TUI when:
		// 1. No subcommand was invoked (this RunE only fires for bare "grove")
		// 2. TTY is attached (interactive terminal)
		// 3. TUI is not disabled via env var
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return cmd.Help()
		}
		if os.Getenv("GROVE_TUI") == "0" {
			return cmd.Help()
		}

		// Check if we're in a grove project
		groveDir, err := grove.IsGroveProject()
		if err != nil {
			return fmt.Errorf("failed to detect grove project: %w", err)
		}
		if groveDir == "" {
			// Not in a grove project — fall through to help
			return cmd.Help()
		}

		projectRoot := grove.MustProjectRoot(groveDir)

		mgr, err := worktree.NewManager(projectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		stateMgr, err := state.NewManager(groveDir)
		if err != nil {
			return fmt.Errorf("failed to initialize state: %w", err)
		}

		_, err = tui.Run(mgr, stateMgr, projectRoot)
		return err
	},
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags can be added here if needed
}
