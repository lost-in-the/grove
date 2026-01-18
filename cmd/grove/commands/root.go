package commands

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "grove",
	Short: "Zero-friction worktree management",
	Long: `Grove is a zero-friction worktree + tmux manager for developers.
	
It provides fast context switching between git worktrees with automatic 
tmux session management. Every command completes in less than 500ms.

Use 'grove <command> --help' for more information about a command.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags can be added here if needed
}
