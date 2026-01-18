package commands

import (
	"fmt"

	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List all worktrees",
	Long:    `List all git worktrees with their status (clean/dirty) and branch information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := worktree.NewManager("")
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		trees, err := mgr.List()
		if err != nil {
			return fmt.Errorf("failed to list worktrees: %w", err)
		}

		if len(trees) == 0 {
			fmt.Println("No worktrees found")
			return nil
		}

		// Print header
		fmt.Printf("%-30s %-20s %-10s %s\n", "NAME", "BRANCH", "STATUS", "PATH")
		fmt.Println("─────────────────────────────────────────────────────────────────────────────")

		for _, tree := range trees {
			status := "clean"
			if tree.IsDirty {
				status = "dirty"
			}
			fmt.Printf("%-30s %-20s %-10s %s\n", tree.Name, tree.Branch, status, tree.Path)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(lsCmd)
}
