package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/internal/hooks"
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

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
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
		isProtectedByConfig := cfg != nil && cfg.IsProtected(name)
		isEnvironment := false
		ws, _ := ctx.State.GetWorktree(name)
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

		// Check if trying to remove the current worktree
		currentTree, _ := mgr.GetCurrent()
		if currentTree != nil && currentTree.Path == wt.Path {
			cli.Error(stderr, "cannot remove current worktree '%s'", name)
			cli.Info(stderr, "Switch to another worktree first: grove to <name>")
			os.Exit(exitcode.CannotRemove)
		}

		// Cannot remove dirty worktree without --force
		if wt.IsDirty && !rmForce {
			cli.Error(stderr, "worktree '%s' has uncommitted changes", name)
			dirtyFiles, err := mgr.GetDirtyFiles(wt.Path)
			if err == nil && dirtyFiles != "" {
				for _, line := range strings.Split(dirtyFiles, "\n") {
					if line != "" {
						cli.Faint(stderr, "  %s", line)
					}
				}
			}
			cli.Info(stderr, "To remove anyway: grove rm %s --force", name)
			cli.Info(stderr, "To switch and commit: grove to %s", name)
			os.Exit(exitcode.CannotRemove)
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
				sessionName := worktree.TmuxSessionName(mgr.GetProjectName(), name)
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

		// Execute pre-remove hooks (user hooks from hooks.toml)
		hookExecutor, hookErr := hooks.NewExecutor()
		if hookErr == nil && hookExecutor.HasHooksForEvent(hooks.EventPreRemove) {
			hookCtx := &hooks.ExecutionContext{
				Event:        hooks.EventPreRemove,
				Worktree:     name,
				Branch:       branchName,
				Project:      projectName,
				MainPath:     ctx.ProjectRoot,
				NewPath:      wt.Path,
				WorktreeFull: projectName + "-" + name,
			}
			cli.Step(w, "Running pre-remove hooks...")
			if err := hookExecutor.Execute(hooks.EventPreRemove, hookCtx); err != nil {
				cli.Warning(w, "Hook execution had errors: %v", err)
			}
		}

		// Fire plugin pre-remove hook (e.g., stop agent stacks)
		pluginHookCtx := &hooks.Context{
			Worktree:     name,
			Config:       cfg,
			WorktreePath: wt.Path,
			MainPath:     ctx.ProjectRoot,
		}
		if err := hooks.Fire(hooks.EventPreRemove, pluginHookCtx); err != nil {
			cli.Warning(w, "Pre-remove plugin hook failed: %v", err)
		}

		// Remove worktree — the critical step, done before tmux kill
		if err := mgr.Remove(name); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}

		// Remove from state
		if err := ctx.State.RemoveWorktree(name); err != nil {
			cli.Warning(w, "worktree removed but state cleanup failed: %v", err)
		}

		cli.Success(w, "Removed worktree '%s'", name)

		// Kill tmux session after worktree is confirmed gone
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			exists, err := tmux.SessionExists(sessionName)
			if err == nil && exists {
				if err := tmux.KillSession(sessionName); err != nil {
					cli.Warning(w, "Failed to kill tmux session: %v", err)
				} else {
					cli.Success(w, "Killed tmux session '%s'", sessionName)
				}
			}
		}

		// Handle branch deletion
		if branchName != "" && !rmKeepBranch {
			if err := handleBranchDeletion(ctx.ProjectRoot, branchName, rmDeleteBranch, rmForce); err != nil {
				cli.Warning(w, "Branch handling: %v", err)
			}
		}

		// Execute post-remove hooks (user hooks from hooks.toml)
		if hookErr == nil && hookExecutor.HasHooksForEvent(hooks.EventPostRemove) {
			hookCtx := &hooks.ExecutionContext{
				Event:    hooks.EventPostRemove,
				Worktree: name,
				Branch:   branchName,
				Project:  projectName,
				MainPath: ctx.ProjectRoot,
			}
			cli.Step(w, "Running post-remove hooks...")
			if err := hookExecutor.Execute(hooks.EventPostRemove, hookCtx); err != nil {
				cli.Warning(w, "Post-remove hook had errors: %v", err)
			}
		}

		// Fire plugin post-remove hook
		if err := hooks.Fire(hooks.EventPostRemove, pluginHookCtx); err != nil {
			cli.Warning(w, "Post-remove plugin hook failed: %v", err)
		}

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
