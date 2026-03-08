package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/cli"
	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/git"
	"github.com/LeahArmstrong/grove-cli/internal/log"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

var (
	cleanOlderThan      int
	cleanIncludeDirty   bool
	cleanDryRun         bool
	cleanKeepBranches   bool
	cleanDeleteBranches bool
)

// CleanCandidate represents a worktree that may be cleaned
type CleanCandidate struct {
	Name          string
	Path          string
	Branch        string
	LastAccess    time.Time
	DaysSince     int
	IsDirty       bool
	ExcludeReason string // If set, worktree cannot be cleaned
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove old unused worktrees",
	Long: `Remove worktrees that haven't been accessed recently.

By default, targets worktrees not accessed in 30 days.
Dirty worktrees are excluded unless --include-dirty is specified.
After removing worktrees, you'll be prompted to delete associated branches.

Always excludes:
  - Root/main worktree
  - Current worktree
  - Protected worktrees (configured in config.toml)
  - Environment worktrees (--mirror)

This command ALWAYS prompts for confirmation, even in non-interactive mode.

Examples:
  grove clean                      # Clean worktrees older than 30 days
  grove clean --older-than 7       # Clean worktrees older than 7 days
  grove clean --include-dirty      # Include dirty worktrees
  grove clean --delete-branches    # Delete branches without prompting
  grove clean --keep-branches      # Keep all branches
  grove clean --dry-run            # Show what would be cleaned`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		w := cli.NewStdout()

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		cfg := ctx.Config

		// Get current worktree
		currentTree, _ := mgr.GetCurrent()
		var currentPath string
		if currentTree != nil {
			currentPath = currentTree.Path
		}

		// List all worktrees
		trees, err := mgr.List()
		if err != nil {
			return fmt.Errorf("failed to list worktrees: %w", err)
		}

		projectName := mgr.GetProjectName()
		now := time.Now()

		candidates := []CleanCandidate{}

		for _, tree := range trees {
			candidate := CleanCandidate{
				Name:    tree.ShortName,
				Path:    tree.Path,
				Branch:  tree.Branch,
				IsDirty: tree.IsDirty,
			}

			// Get last access time from state
			ws, _ := ctx.State.GetWorktree(tree.ShortName)
			if ws != nil {
				candidate.LastAccess = ws.LastAccessedAt
				candidate.DaysSince = int(now.Sub(ws.LastAccessedAt).Hours() / 24)
			} else {
				// If not in state, assume very old
				candidate.DaysSince = 9999
			}

			// Check exclusions
			if tree.IsMain {
				candidate.ExcludeReason = "root worktree"
			} else if tree.Path == currentPath {
				candidate.ExcludeReason = "current worktree"
			} else if cfg != nil && cfg.IsProtected(tree.ShortName) {
				candidate.ExcludeReason = "protected"
			} else if ws != nil && ws.Environment {
				candidate.ExcludeReason = "environment worktree"
			} else if tree.IsDirty && !cleanIncludeDirty {
				candidate.ExcludeReason = "dirty (use --include-dirty)"
			} else if candidate.DaysSince < cleanOlderThan {
				candidate.ExcludeReason = fmt.Sprintf("accessed %d days ago (threshold: %d)", candidate.DaysSince, cleanOlderThan)
			}

			candidates = append(candidates, candidate)
		}

		// Filter to cleanable candidates
		cleanable := []CleanCandidate{}
		for _, c := range candidates {
			if c.ExcludeReason == "" {
				cleanable = append(cleanable, c)
			}
		}

		if len(cleanable) == 0 {
			cli.Info(w, "No worktrees eligible for cleanup.")
			if len(candidates) > 0 {
				_, _ = fmt.Fprintln(w)
				cli.Header(w, "Excluded worktrees")
				for _, c := range candidates {
					if c.ExcludeReason != "" {
						cli.Faint(w, "  %s - %s", c.Name, c.ExcludeReason)
					}
				}
			}
			return nil
		}

		// Display candidates
		cli.Header(w, "Found %d worktree(s) eligible for cleanup", len(cleanable))
		for _, c := range cleanable {
			dirtyMark := ""
			if c.IsDirty {
				dirtyMark = " [dirty]"
			}
			_, _ = fmt.Fprintf(w, "  %s ", c.Name)
			cli.Faint(w, "  (%s) - %d days since last access%s", c.Branch, c.DaysSince, dirtyMark)
		}

		if cleanDryRun {
			_, _ = fmt.Fprintln(w)
			cli.Info(w, "Dry run - no changes made.")
			return nil
		}

		// ALWAYS prompt - mandatory confirmation
		_, _ = fmt.Fprintln(w)
		cli.Warning(w, "This will permanently remove %d worktree(s) and their associated tmux sessions.", len(cleanable))
		fmt.Print("Type 'yes' to confirm: ")

		var response string
		_, _ = fmt.Scanln(&response)
		if response != "yes" {
			fmt.Println("Canceled")
			os.Exit(exitcode.UserCancelled)
		}

		// Perform cleanup
		removed := 0
		failed := 0
		removedBranches := []string{}

		for _, c := range cleanable {
			// Remove worktree
			if err := mgr.Remove(c.Name); err != nil {
				cli.Warning(w, "Failed to remove '%s': %v", c.Name, err)
				failed++
				continue
			}

			// Remove from state
			if err := ctx.State.RemoveWorktree(c.Name); err != nil {
				log.Printf("failed to remove worktree %q from state: %v", c.Name, err)
			}

			// Kill tmux session if exists
			sessionName := worktree.TmuxSessionName(projectName, c.Name)
			if tmux.IsTmuxAvailable() {
				if exists, _ := tmux.SessionExists(sessionName); exists {
					if err := tmux.KillSession(sessionName); err != nil {
						cli.Warning(w, "Worktree removed but failed to kill tmux session '%s': %v", sessionName, err)
					}
				}
			}

			cli.Success(w, "Removed '%s'", c.Name)
			removed++

			// Track branch for later deletion
			if c.Branch != "" {
				removedBranches = append(removedBranches, c.Branch)
			}
		}

		_, _ = fmt.Fprintln(w)
		cli.Info(w, "Cleanup complete: %d removed, %d failed", removed, failed)

		// Handle branch deletion
		if len(removedBranches) > 0 && !cleanKeepBranches {
			if err := handleBatchBranchDeletion(ctx.ProjectRoot, removedBranches, cleanDeleteBranches); err != nil {
				cli.Warning(w, "Branch cleanup: %v", err)
			}
		}

		return nil
	}),
}

// BranchInfo holds branch status for batch processing
type BranchInfo struct {
	Name          string
	Status        *git.BranchStatus
	SafeToDelete  bool
	StatusSummary string
}

// handleBatchBranchDeletion handles batch deletion of branches after grove clean
func handleBatchBranchDeletion(repoPath string, branches []string, forceDelete bool) error {
	w := cli.NewStdout()

	branchMgr, err := git.NewBranchManager(repoPath)
	if err != nil {
		return fmt.Errorf("failed to initialize branch manager: %w", err)
	}

	// Collect branch info
	branchInfos := []BranchInfo{}
	for _, branch := range branches {
		status, err := branchMgr.GetStatus(branch, "")
		if err != nil {
			log.Printf("skipping branch %q during cleanup: %v", branch, err)
			continue
		}

		// Skip branches used by other worktrees
		if status.UsedByWorktree != "" {
			continue
		}

		info := BranchInfo{
			Name:   branch,
			Status: status,
		}

		// Determine if safe to delete and build summary
		if status.IsMerged && status.UnpushedCount == 0 {
			info.SafeToDelete = true
			info.StatusSummary = "merged, safe to delete"
		} else if status.UnpushedCount > 0 {
			info.SafeToDelete = false
			info.StatusSummary = fmt.Sprintf("%d unpushed commit(s)", status.UnpushedCount)
		} else if !status.IsMerged {
			info.SafeToDelete = false
			info.StatusSummary = "not merged"
		}

		branchInfos = append(branchInfos, info)
	}

	if len(branchInfos) == 0 {
		return nil
	}

	// Force delete - no prompting
	if forceDelete {
		deleted := 0
		for _, info := range branchInfos {
			if err := branchMgr.Delete(info.Name, true); err != nil {
				cli.Warning(w, "Failed to delete branch '%s': %v", info.Name, err)
			} else {
				cli.Success(w, "Deleted branch '%s'", info.Name)
				deleted++
			}
		}
		if deleted > 0 {
			cli.Info(w, "Deleted %d branch(es)", deleted)
		}
		return nil
	}

	// Interactive mode - show summary and prompt
	_, _ = fmt.Fprintln(w)
	cli.Header(w, "Associated branches")
	hasUnsafe := false
	for _, info := range branchInfos {
		if !info.SafeToDelete {
			hasUnsafe = true
			cli.Warning(w, "%s (%s)", info.Name, info.StatusSummary)
		} else {
			_, _ = fmt.Fprintf(w, "  • %s ", info.Name)
			cli.Faint(w, "  (%s)", info.StatusSummary)
		}
	}

	if hasUnsafe {
		_, _ = fmt.Fprintln(w)
		cli.Warning(w, "Some branches have unpushed commits or are not merged")
	}

	// Ask for confirmation
	question := fmt.Sprintf("Delete %d associated branch(es)?", len(branchInfos))
	confirmed, err := cli.Confirm(question, false)
	if err != nil {
		// Non-interactive
		cli.Info(w, "Branches not deleted (use --delete-branches or --keep-branches)")
		return nil
	}

	if confirmed {
		deleted := 0
		for _, info := range branchInfos {
			// Force delete if not safe (user confirmed)
			if err := branchMgr.Delete(info.Name, !info.SafeToDelete); err != nil {
				cli.Warning(w, "Failed to delete branch '%s': %v", info.Name, err)
			} else {
				cli.Success(w, "Deleted branch '%s'", info.Name)
				deleted++
			}
		}
		if deleted > 0 {
			cli.Info(w, "Deleted %d branch(es)", deleted)
		}
	} else {
		cli.Info(w, "Kept all branches")
	}

	return nil
}

func init() {
	cleanCmd.Flags().IntVar(&cleanOlderThan, "older-than", 30, "Remove worktrees not accessed in this many days")
	cleanCmd.Flags().BoolVar(&cleanIncludeDirty, "include-dirty", false, "Include worktrees with uncommitted changes")
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "Show what would be cleaned without making changes")
	cleanCmd.Flags().BoolVar(&cleanKeepBranches, "keep-branches", false, "Do not delete associated branches")
	cleanCmd.Flags().BoolVar(&cleanDeleteBranches, "delete-branches", false, "Delete associated branches without prompting")
	cleanCmd.MarkFlagsMutuallyExclusive("keep-branches", "delete-branches")
	rootCmd.AddCommand(cleanCmd)
}
