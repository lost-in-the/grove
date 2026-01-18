package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
)

const (
	// maxDirtyFilesShown is the maximum number of dirty files to display
	maxDirtyFilesShown = 5
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

		// Get project name from worktree manager
		projectName := mgr.GetProjectName()

		// The tmux session name should match the directory name
		// For main worktree: project name
		// For other worktrees: project-name (full directory name)
		tmuxSessionName := filepath.Base(tree.Path)
		tmuxStatus := tmux.GetSessionStatus(tmuxSessionName)

		// Format status
		statusIcon := "✓ Clean"
		if tree.IsDirty {
			statusIcon = "● Dirty"
		}

		// Print formatted output
		displayName := tree.DisplayName()
		if tree.IsMain {
			displayName = projectName
		}
		fmt.Printf("%s (%s)\n", displayName, tree.Branch)
		fmt.Println(strings.Repeat("━", 40))
		fmt.Printf("Path:    %s\n", tree.Path)
		fmt.Printf("Branch:  %s\n", tree.Branch)

		// Show commit info
		if tree.ShortCommit != "" && tree.CommitMessage != "" {
			fmt.Printf("Commit:  %s - %s (%s)\n", tree.ShortCommit, tree.CommitMessage, tree.CommitAge)
		} else {
			fmt.Printf("Commit:  %s\n", tree.Commit)
		}

		fmt.Printf("Status:  %s\n", statusIcon)

		// Show dirty files if present
		if tree.IsDirty && tree.DirtyFiles != "" {
			lines := strings.Split(tree.DirtyFiles, "\n")
			// Show first few files
			if len(lines) > maxDirtyFilesShown {
				for i := 0; i < maxDirtyFilesShown; i++ {
					fmt.Printf("         %s\n", lines[i])
				}
				fmt.Printf("         ... and %d more\n", len(lines)-maxDirtyFilesShown)
			} else {
				for _, line := range lines {
					if line != "" {
						fmt.Printf("         %s\n", line)
					}
				}
			}
		}

		// Show tmux status
		fmt.Printf("tmux:    %s", tmuxSessionName)
		if tmuxStatus != "none" {
			fmt.Printf(" (%s)", tmuxStatus)
		}
		fmt.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(hereCmd)
}
