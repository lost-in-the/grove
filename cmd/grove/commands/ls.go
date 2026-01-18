package commands

import (
	"fmt"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
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

		// Load config for tmux prefix
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Get current worktree to mark it
		currentTree, _ := mgr.GetCurrent()
		
		// Get tmux sessions for status
		tmuxAvailable := tmux.IsTmuxAvailable()
		var sessions map[string]*tmux.Session
		if tmuxAvailable {
			sessionList, err := tmux.ListSessions()
			if err == nil {
				sessions = make(map[string]*tmux.Session)
				for _, s := range sessionList {
					sessions[s.Name] = s
				}
			}
		}

		// Print header
		fmt.Printf("%-3s %-12s %-15s %-10s %-12s %s\n", "", "NAME", "BRANCH", "STATUS", "TMUX", "PATH")
		fmt.Println("──────────────────────────────────────────────────────────────────────────────────────────")

		for _, tree := range trees {
			// Current indicator
			indicator := "  "
			if currentTree != nil && tree.Path == currentTree.Path {
				indicator = "● "
			}
			
			// Git status
			status := "clean"
			if tree.IsDirty {
				status = "dirty"
			}
			
			// Tmux status - check multiple possible session names
			tmuxStatus := "none"
			if tmuxAvailable && sessions != nil {
				// Try with full name (matches tree.Name)
				if session, ok := sessions[tree.Name]; ok {
					if session.Attached {
						tmuxStatus = "attached"
					} else {
						tmuxStatus = "detached"
					}
				} else {
					// Try with config prefix + short name
					prefixedName := cfg.Tmux.Prefix + tree.ShortName
					if session, ok := sessions[prefixedName]; ok {
						if session.Attached {
							tmuxStatus = "attached"
						} else {
							tmuxStatus = "detached"
						}
					}
				}
			}
			
			fmt.Printf("%s %-12s %-15s %-10s %-12s %s\n", 
				indicator, 
				tree.DisplayName(), 
				tree.Branch, 
				status, 
				tmuxStatus,
				tree.Path)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(lsCmd)
}
