package commands

import (
	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/worktree"
)

// completeWorktreeNames returns worktree display names for shell completion.
// Uses ListNames() which skips dirty checks for fast tab completion.
func completeWorktreeNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return listWorktreeNames(), cobra.ShellCompDirectiveNoFileComp
}

// completeWorktreeNamesFirstArg completes worktree names only for the first argument.
// Use this for commands like rename where only the first arg is a worktree name.
func completeWorktreeNamesFirstArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return listWorktreeNames(), cobra.ShellCompDirectiveNoFileComp
}

func listWorktreeNames() []string {
	groveDir, err := grove.IsGroveProject()
	if err != nil || groveDir == "" {
		return nil
	}
	projectRoot := grove.MustProjectRoot(groveDir)
	mgr, err := worktree.NewManager(projectRoot)
	if err != nil {
		return nil
	}
	names, err := mgr.ListNames()
	if err != nil {
		return nil
	}
	return names
}
