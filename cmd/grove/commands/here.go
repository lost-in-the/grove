package commands

import (
	"fmt"

	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var hereCmd = &cobra.Command{
	Use:   "here",
	Short: "Show current worktree information",
	Long:  `Display information about the current worktree including name, branch, and status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := worktree.NewManager("")
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		tree, err := mgr.GetCurrent()
		if err != nil {
			return fmt.Errorf("failed to get current worktree: %w", err)
		}

		fmt.Printf("Name:   %s\n", tree.Name)
		fmt.Printf("Branch: %s\n", tree.Branch)
		fmt.Printf("Path:   %s\n", tree.Path)
		fmt.Printf("Commit: %s\n", tree.Commit)

		status := "clean"
		if tree.IsDirty {
			status = "dirty"
		}
		fmt.Printf("Status: %s\n", status)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(hereCmd)
}
