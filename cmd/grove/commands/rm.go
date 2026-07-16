package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/worktree"
)

var (
	rmForce        bool
	rmUnprotect    bool
	rmDryRun       bool
	rmKeepBranch   bool
	rmDeleteBranch bool
)

var rmCmd = &cobra.Command{
	Use:     "rm [name]",
	Aliases: []string{"remove", "delete"},
	Short:   "Remove a worktree and its tmux session",
	Long: `Remove a git worktree by name and kill its associated tmux session.

This will delete the worktree directory and remove the git worktree reference.
By default, you'll be prompted to delete the associated branch.

Protected worktrees (configured in config.toml) require both --force and --unprotect flags.
Environment worktrees are implicitly protected.

Examples:
  grove rm feature-auth                  # Remove worktree, prompt for branch
  grove rm feature-auth --delete-branch  # Remove worktree and branch
  grove rm feature-auth --keep-branch    # Remove worktree, keep branch
  grove rm staging --force --unprotect   # Remove protected worktree`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeWorktreeNames,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		w := cli.NewStdout()
		stderr := cli.NewStderr()

		var name string
		if len(args) == 0 {
			selected, err := selectWorktree(ctx, "Remove which worktree?")
			if err != nil {
				return err
			}
			name = selected
		} else {
			name = args[0]
		}

		if name == "" {
			return fmt.Errorf("worktree name cannot be empty")
		}

		mgr, err := ctx.WorktreeManager()
		if err != nil {
			return err
		}

		// Find worktree early — all checks reuse this
		wt, err := mgr.Find(name)
		if err != nil {
			return fmt.Errorf("failed to find worktree: %w", err)
		}
		if wt == nil {
			cli.Error(stderr, "worktree '%s' not found", name)
			os.Exit(exitcode.ResourceNotFound)
		}

		// Find matches by short name, display name, branch, or path basename,
		// so the argument the user typed (e.g. a branch name that differs
		// from the worktree's directory name) isn't necessarily the name
		// state and tmux sessions are keyed under. Use the resolved short
		// name for state/tmux operations from here on; user-facing messages
		// keep echoing the argument as typed.
		resolvedName := wt.ShortName

		// When selected interactively, confirm before proceeding
		if len(args) == 0 {
			details := []string{
				fmt.Sprintf("Branch: %s", wt.Branch),
				fmt.Sprintf("Path:   %s", wt.Path),
			}
			confirmed, confirmErr := cli.ConfirmWithDetails(
				stderr,
				fmt.Sprintf("Remove worktree '%s'?", name),
				details,
				"Proceed?",
				false,
			)
			if confirmErr != nil || !confirmed {
				return fmt.Errorf("removal canceled")
			}
		}

		// Cannot remove the main worktree (unconditional — git won't allow it)
		if wt.IsMain {
			cli.Error(stderr, "cannot remove the main worktree")
			cli.Faint(stderr, "The main worktree is your primary project directory.")
			cli.Faint(stderr, "To remove the entire project, delete it manually.")
			os.Exit(exitcode.CannotRemove)
		}

		// Check protection status
		cfg := ctx.Config
		isProtectedByConfig := cfg != nil && (cfg.IsProtected(resolvedName) || cfg.IsProtected(name))
		isEnvironment := false
		ws, _ := ctx.State.GetWorktree(resolvedName)
		if ws != nil {
			isEnvironment = ws.Environment
		}

		isProtected := isProtectedByConfig || isEnvironment

		if isProtected {
			if !rmForce || !rmUnprotect {
				reason := "protected in config.toml"
				if isEnvironment {
					reason = "environment worktree"
				}
				cli.Error(stderr, "worktree '%s' is protected (%s)", name, reason)
				cli.Info(stderr, "To remove, use: grove rm %s --force --unprotect", name)
				os.Exit(exitcode.CannotRemove)
			}
			if !rmDryRun {
				cli.Warning(w, "Removing protected worktree '%s'", name)
			}
		}

		// Check if trying to remove the current worktree. CurrentPath avoids
		// the per-worktree dirty checks GetCurrent would perform on the rest
		// of the repo.
		if currentPath, err := mgr.CurrentPath(); err == nil && currentPath == wt.Path {
			cli.Error(stderr, "cannot remove current worktree '%s'", name)
			cli.Info(stderr, "Switch to another worktree first: grove to <name>")
			os.Exit(exitcode.CannotRemove)
		}

		// Cannot remove dirty worktree without --force. Dirty status is no
		// longer populated by Find — check it here for just this one path.
		if !rmForce {
			dirtyFiles, _ := mgr.GetDirtyFiles(wt.Path)
			if dirtyFiles != "" {
				cli.Error(stderr, "worktree '%s' has uncommitted changes", name)
				for _, line := range strings.Split(dirtyFiles, "\n") {
					if line != "" {
						cli.Faint(stderr, "  %s", line)
					}
				}
				cli.Info(stderr, "To remove anyway: grove rm %s --force", name)
				cli.Info(stderr, "To switch and commit: grove to %s", name)
				os.Exit(exitcode.CannotRemove)
			}
		}

		// Dry run - just show what would happen
		if rmDryRun {
			cli.Info(w, "Would remove worktree '%s'", name)
			cli.Faint(w, "  Path: %s", wt.Path)
			if wt.Branch != "" {
				cli.Faint(w, "  Branch: %s", wt.Branch)
				if rmDeleteBranch {
					cli.Faint(w, "  Would delete branch: %s", wt.Branch)
				} else if rmKeepBranch {
					cli.Faint(w, "  Would keep branch: %s", wt.Branch)
				} else {
					cli.Faint(w, "  Would prompt for branch deletion")
				}
			}
			if tmux.IsTmuxAvailable() {
				sessionName := worktree.TmuxSessionName(mgr.GetProjectName(), resolvedName)
				exists, _ := tmux.SessionExists(sessionName)
				if exists {
					cli.Faint(w, "  Would kill tmux session: %s", sessionName)
				}
			}
			return nil
		}

		projectName := mgr.GetProjectName()

		// Get branch name before removing (need worktree info)
		var branchName string
		if wt.Branch != "" {
			branchName = wt.Branch
		}

		// Shared removal sequence: pre-remove hooks, git removal, state
		// cleanup, tmux session kill (also used by `grove trim`).
		if err := removeWorktreeWithHooks(ctx, mgr, w, projectName, resolvedName, wt.Path, branchName, rmForce); err != nil {
			return err
		}

		// Handle branch deletion
		if branchName != "" && !rmKeepBranch {
			if err := handleBranchDeletion(ctx.ProjectRoot, branchName, rmDeleteBranch, rmForce); err != nil {
				cli.Warning(w, "Branch handling: %v", err)
			}
		}

		// User + plugin post-remove hooks
		firePostRemoveHooks(ctx, w, projectName, resolvedName, wt.Path, branchName)

		return nil
	}),
}

// handleBranchDeletion manages the branch deletion logic
func handleBranchDeletion(repoPath, branch string, forceDelete, forceUnmerged bool) error {
	w := cli.NewStdout()

	branchMgr, err := git.NewBranchManager(repoPath)
	if err != nil {
		return fmt.Errorf("failed to initialize branch manager: %w", err)
	}

	// Get branch status
	status, err := branchMgr.GetStatus(branch, "")
	if err != nil {
		return fmt.Errorf("failed to get branch status: %w", err)
	}

	// If branch is used by another worktree, keep it
	if status.UsedByWorktree != "" {
		cli.Info(w, "Branch '%s' is used by worktree at %s (keeping)", branch, status.UsedByWorktree)
		return nil
	}

	// If forceDelete flag is set, delete without prompting
	if forceDelete {
		if err := branchMgr.Delete(branch, forceUnmerged); err != nil {
			return fmt.Errorf("failed to delete branch: %w", err)
		}
		cli.Success(w, "Deleted branch '%s'", branch)
		return nil
	}

	// Interactive mode - confirm with user
	return confirmAndDeleteBranch(w, branchMgr, branch, status, forceUnmerged)
}

// confirmAndDeleteBranch handles interactive branch deletion with user confirmation.
func confirmAndDeleteBranch(w *cli.Writer, branchMgr *git.BranchManager, branch string, status *git.BranchStatus, forceUnmerged bool) error {
	details := branchDeletionDetails(branchMgr, branch, status)
	header := fmt.Sprintf("Branch '%s':", branch)

	confirmed, err := cli.ConfirmWithDetails(w, header, details, "Delete branch?", false)
	if err != nil {
		cli.Info(w, "Branch '%s' not deleted (use --delete-branch or --keep-branch)", branch)
		return nil
	}

	if confirmed {
		needsForce := !status.IsMerged || forceUnmerged
		if err := branchMgr.Delete(branch, needsForce); err != nil {
			return fmt.Errorf("failed to delete branch: %w", err)
		}
		cli.Success(w, "Deleted branch '%s'", branch)
	} else {
		cli.Info(w, "Kept branch '%s'", branch)
	}

	return nil
}

// branchDeletionDetails builds the detail lines for the branch deletion prompt.
func branchDeletionDetails(branchMgr *git.BranchManager, branch string, status *git.BranchStatus) []string {
	var details []string

	if !status.IsMerged {
		details = append(details, "Branch is not merged into default branch")
	}

	if status.UnpushedCount > 0 {
		details = append(details, fmt.Sprintf("Has %d unpushed commit(s)", status.UnpushedCount))

		commits, _ := branchMgr.GetUnpushedCommits(branch, 5)
		for _, commit := range commits {
			details = append(details, "  "+commit)
		}
		if status.UnpushedCount > 5 {
			details = append(details, fmt.Sprintf("  ... and %d more", status.UnpushedCount-5))
		}
	}

	if status.IsMerged && status.UnpushedCount == 0 {
		details = append(details, "Branch is merged and safe to delete")
	}

	return details
}

func init() {
	rmCmd.Flags().BoolVarP(&rmForce, "force", "f", false, "Force removal of dirty or protected worktrees (also allows deleting unmerged branches)")
	rmCmd.Flags().BoolVar(&rmUnprotect, "unprotect", false, "Allow removing protected worktrees (requires --force)")
	rmCmd.Flags().BoolVar(&rmDryRun, "dry-run", false, "Show what would be removed without making changes")
	rmCmd.Flags().BoolVar(&rmKeepBranch, "keep-branch", false, "Do not delete the associated branch")
	rmCmd.Flags().BoolVar(&rmDeleteBranch, "delete-branch", false, "Delete the associated branch without prompting")
	rmCmd.MarkFlagsMutuallyExclusive("keep-branch", "delete-branch")
	rootCmd.AddCommand(rmCmd)
}
