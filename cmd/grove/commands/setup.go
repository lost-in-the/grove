package commands

import (
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:    "setup",
	Short:  "Initialize a new grove project (alias for 'grove init')",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func init() {
	setupCmd.Flags().BoolVar(&initWithTesting, "with-testing", false, "Also create a testing worktree")
	setupCmd.Flags().BoolVar(&initWithScratch, "with-scratch", false, "Also create a scratch worktree")
	setupCmd.Flags().BoolVar(&initFull, "full", false, "Create testing, scratch, and hotfix worktrees")
	setupCmd.Flags().BoolVar(&initNoHooks, "no-hooks", false, "Skip hooks.toml generation")
	rootCmd.AddCommand(setupCmd)
}
