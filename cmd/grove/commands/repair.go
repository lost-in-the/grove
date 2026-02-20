package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

var (
	repairDryRun bool
)

// RepairIssue represents a single repair issue found
type RepairIssue struct {
	Type        string // "orphan_state", "missing_state", "orphan_tmux", "corrupt_state"
	Description string
	Action      string
	Resolved    bool
}

var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Repair state and worktree inconsistencies",
	Long: `Detect and repair inconsistencies between grove state and actual worktrees.

Repairs performed:
  1. Remove state entries for missing worktree directories
  2. Add state entries for worktrees not tracked in state
  3. Offer to kill orphaned tmux sessions (grove sessions without worktrees)
  4. Reinitialize corrupted state file

Examples:
  grove repair           # Detect and fix issues
  grove repair --dry-run # Show what would be fixed without making changes`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		projectName := mgr.GetProjectName()
		issues := []RepairIssue{}

		// 1. Get actual git worktrees
		gitWorktrees, err := mgr.List()
		if err != nil {
			return fmt.Errorf("failed to list git worktrees: %w", err)
		}

		// Build map of git worktree paths
		gitWorktreeByPath := make(map[string]*worktree.Worktree)
		gitWorktreeByName := make(map[string]*worktree.Worktree)
		for _, wt := range gitWorktrees {
			gitWorktreeByPath[wt.Path] = wt
			gitWorktreeByName[wt.ShortName] = wt
		}

		// 2. Check state entries against actual worktrees
		stateWorktrees := ctx.State.ListWorktrees()
		for _, name := range stateWorktrees {
			ws, _ := ctx.State.GetWorktree(name)
			if ws == nil {
				continue
			}

			// Check if the path still exists
			if _, err := os.Stat(ws.Path); os.IsNotExist(err) {
				issues = append(issues, RepairIssue{
					Type:        "orphan_state",
					Description: fmt.Sprintf("State entry '%s' points to missing directory: %s", name, ws.Path),
					Action:      fmt.Sprintf("Remove '%s' from state", name),
				})
			}
		}

		// 3. Check git worktrees not in state
		for _, wt := range gitWorktrees {
			if wt.IsMain {
				continue // Main worktree doesn't need state entry
			}

			// Check if worktree is in state
			ws, _ := ctx.State.GetWorktree(wt.ShortName)
			if ws == nil {
				issues = append(issues, RepairIssue{
					Type:        "missing_state",
					Description: fmt.Sprintf("Worktree '%s' at %s not tracked in state", wt.ShortName, wt.Path),
					Action:      fmt.Sprintf("Add '%s' to state", wt.ShortName),
				})
			}
		}

		// 4. Check for orphaned tmux sessions
		if tmux.IsTmuxAvailable() {
			sessions, err := tmux.ListSessions()
			if err == nil {
				prefix := projectName + "-"
				for _, session := range sessions {
					if strings.HasPrefix(session.Name, prefix) {
						// Extract worktree name from session name
						wtName := strings.TrimPrefix(session.Name, prefix)

						// Check if corresponding worktree exists
						if _, exists := gitWorktreeByName[wtName]; !exists {
							issues = append(issues, RepairIssue{
								Type:        "orphan_tmux",
								Description: fmt.Sprintf("Tmux session '%s' has no corresponding worktree", session.Name),
								Action:      fmt.Sprintf("Kill tmux session '%s'", session.Name),
							})
						}
					}
				}
			}
		}

		// Display findings
		if len(issues) == 0 {
			fmt.Println("No issues found. State is consistent with worktrees.")
			return nil
		}

		fmt.Printf("Found %d issue(s):\n\n", len(issues))
		for i, issue := range issues {
			fmt.Printf("  %d. [%s] %s\n", i+1, issue.Type, issue.Description)
			fmt.Printf("     Action: %s\n\n", issue.Action)
		}

		if repairDryRun {
			fmt.Println("Dry run - no changes made.")
			return nil
		}

		// Prompt for confirmation
		if !isInteractive() {
			fmt.Println("Non-interactive mode: use --dry-run to preview or run interactively to repair")
			return nil
		}

		fmt.Print("Proceed with repairs? [y/N]: ")
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			os.Exit(exitcode.UserCancelled)
		}

		// Perform repairs
		repaired := 0
		failed := 0

		for i := range issues {
			issue := &issues[i]
			switch issue.Type {
			case "orphan_state":
				// Extract name from description (hacky but works)
				name := extractWorktreeName(issue.Description)
				if err := ctx.State.RemoveWorktree(name); err != nil {
					fmt.Printf("  Failed to remove '%s' from state: %v\n", name, err)
					failed++
				} else {
					fmt.Printf("  Removed '%s' from state\n", name)
					issue.Resolved = true
					repaired++
				}

			case "missing_state":
				// Find the worktree and add it to state
				name := extractWorktreeName(issue.Description)
				if wt, exists := gitWorktreeByName[name]; exists {
					now := time.Now()
					ws := &state.WorktreeState{
						Path:           wt.Path,
						Branch:         wt.Branch,
						CreatedAt:      now, // Best guess
						LastAccessedAt: now,
					}
					if err := ctx.State.AddWorktree(name, ws); err != nil {
						fmt.Printf("  Failed to add '%s' to state: %v\n", name, err)
						failed++
					} else {
						fmt.Printf("  Added '%s' to state\n", name)
						issue.Resolved = true
						repaired++
					}
				}

			case "orphan_tmux":
				// Extract session name
				sessionName := extractSessionName(issue.Description)
				if sessionName != "" {
					if err := tmux.KillSession(sessionName); err != nil {
						fmt.Printf("  Failed to kill session '%s': %v\n", sessionName, err)
						failed++
					} else {
						fmt.Printf("  Killed tmux session '%s'\n", sessionName)
						issue.Resolved = true
						repaired++
					}
				}
			}
		}

		fmt.Printf("\nRepairs complete: %d fixed, %d failed\n", repaired, failed)

		if failed > 0 {
			os.Exit(exitcode.WorktreeMissing)
		}

		return nil
	}),
}

// extractWorktreeName extracts the worktree name from issue description
func extractWorktreeName(desc string) string {
	// Pattern: "State entry 'name' ..." or "Worktree 'name' ..."
	start := strings.Index(desc, "'")
	if start < 0 {
		return ""
	}
	end := strings.Index(desc[start+1:], "'")
	if end < 0 {
		return ""
	}
	return desc[start+1 : start+1+end]
}

// extractSessionName extracts the tmux session name from issue description
func extractSessionName(desc string) string {
	// Pattern: "Tmux session 'name' ..."
	start := strings.Index(desc, "'")
	if start < 0 {
		return ""
	}
	end := strings.Index(desc[start+1:], "'")
	if end < 0 {
		return ""
	}
	return desc[start+1 : start+1+end]
}

func init() {
	repairCmd.Flags().BoolVar(&repairDryRun, "dry-run", false, "Show what would be repaired without making changes")
	rootCmd.AddCommand(repairCmd)
}
