package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/git"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/prompt"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

var (
	rmForce        bool
	rmUnprotect    bool
	rmDryRun       bool
	rmKeepBranch   bool
	rmDeleteBranch bool
)

var rmCmd = &cobra.Command{
	Use:     "rm <name>",
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
	Args: cobra.ExactArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		name := args[0]
		if name == "" {
			return fmt.Errorf("worktree name cannot be empty")
		}

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		// Load config for protection settings
		cfg, _ := config.Load()

		// Check protection status
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
				fmt.Fprintf(os.Stderr, "Error: worktree '%s' is protected (%s)\n", name, reason)
				fmt.Fprintf(os.Stderr, "To remove, use: grove rm %s --force --unprotect\n", name)
				os.Exit(exitcode.CannotRemove)
			}
			if !rmDryRun {
				fmt.Printf("⚠ Removing protected worktree '%s'\n", name)
			}
		}

		// Check if trying to remove the current worktree
		currentTree, _ := mgr.GetCurrent()
		if currentTree != nil {
			wt, _ := mgr.Find(name)
			if wt != nil && currentTree.Path == wt.Path {
				fmt.Fprintf(os.Stderr, "Error: cannot remove current worktree '%s'\n", name)
				fmt.Fprintf(os.Stderr, "Switch to another worktree first: grove to <name>\n")
				os.Exit(exitcode.CannotRemove)
			}
		}

		// Dry run - just show what would happen
		if rmDryRun {
			fmt.Printf("Would remove worktree '%s'\n", name)
			// Get path from worktree manager
			wt, _ := mgr.Find(name)
			if wt != nil {
				fmt.Printf("  Path: %s\n", wt.Path)
				if wt.Branch != "" {
					fmt.Printf("  Branch: %s\n", wt.Branch)
					if rmDeleteBranch {
						fmt.Printf("  Would delete branch: %s\n", wt.Branch)
					} else if rmKeepBranch {
						fmt.Printf("  Would keep branch: %s\n", wt.Branch)
					} else {
						fmt.Printf("  Would prompt for branch deletion\n")
					}
				}
			}
			if tmux.IsTmuxAvailable() {
				sessionName := worktree.TmuxSessionName(mgr.GetProjectName(), name)
				exists, _ := tmux.SessionExists(sessionName)
				if exists {
					fmt.Printf("  Would kill tmux session: %s\n", sessionName)
				}
			}
			return nil
		}

		projectName := mgr.GetProjectName()

		// Kill tmux session if it exists
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			exists, err := tmux.SessionExists(sessionName)
			if err == nil && exists {
				if err := tmux.KillSession(sessionName); err != nil {
					fmt.Printf("⚠ Failed to kill tmux session: %v\n", err)
				} else {
					fmt.Printf("✓ Killed tmux session '%s'\n", sessionName)
				}
			}
		}

		// Get branch name before removing (need worktree info)
		var branchName string
		wt, _ := mgr.Find(name)
		if wt != nil && wt.Branch != "" {
			branchName = wt.Branch
		}

		// Execute pre-remove hooks (user hooks from hooks.toml)
		hookExecutor, hookErr := hooks.NewExecutor()
		if hookErr == nil && hookExecutor.HasHooksForEvent(hooks.EventPreRemove) {
			hookCtx := &hooks.ExecutionContext{
				Event:    hooks.EventPreRemove,
				Worktree: name,
				Branch:   branchName,
				Project:  projectName,
				MainPath: ctx.ProjectRoot,
			}
			if wt != nil {
				hookCtx.NewPath = wt.Path
				hookCtx.WorktreeFull = projectName + "-" + name
			}
			fmt.Println("\nRunning pre-remove hooks...")
			if err := hookExecutor.Execute(hooks.EventPreRemove, hookCtx); err != nil {
				fmt.Printf("⚠ Hook execution had errors: %v\n", err)
			}
		}

		// Fire plugin pre-remove hook (e.g., stop agent stacks)
		var wtPath string
		if wt != nil {
			wtPath = wt.Path
		}
		pluginHookCtx := &hooks.Context{
			Worktree:     name,
			Config:       cfg,
			WorktreePath: wtPath,
			MainPath:     ctx.ProjectRoot,
		}
		if err := hooks.Fire(hooks.EventPreRemove, pluginHookCtx); err != nil {
			fmt.Printf("⚠ Pre-remove plugin hook failed: %v\n", err)
		}

		// Remove worktree
		if err := mgr.Remove(name); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}

		// Remove from state
		_ = ctx.State.RemoveWorktree(name)

		fmt.Printf("✓ Removed worktree '%s'\n", name)

		// Handle branch deletion
		if branchName != "" && !rmKeepBranch {
			if err := handleBranchDeletion(ctx.ProjectRoot, branchName, rmDeleteBranch, rmForce); err != nil {
				fmt.Printf("⚠ Branch handling: %v\n", err)
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
			fmt.Println("\nRunning post-remove hooks...")
			_ = hookExecutor.Execute(hooks.EventPostRemove, hookCtx)
		}

		// Fire plugin post-remove hook
		if err := hooks.Fire(hooks.EventPostRemove, pluginHookCtx); err != nil {
			fmt.Printf("⚠ Post-remove plugin hook failed: %v\n", err)
		}

		return nil
	}),
}

// handleBranchDeletion manages the branch deletion logic
func handleBranchDeletion(repoPath, branch string, forceDelete, forceUnmerged bool) error {
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
		fmt.Printf("ℹ Branch '%s' is used by worktree at %s (keeping)\n", branch, status.UsedByWorktree)
		return nil
	}

	// If forceDelete flag is set, delete without prompting
	if forceDelete {
		if err := branchMgr.Delete(branch, forceUnmerged); err != nil {
			return fmt.Errorf("failed to delete branch: %w", err)
		}
		fmt.Printf("✓ Deleted branch '%s'\n", branch)
		return nil
	}

	// Interactive mode - build prompt with details
	var details []string

	if !status.IsMerged {
		details = append(details, "⚠ Branch is not merged into default branch")
	}

	if status.UnpushedCount > 0 {
		details = append(details, fmt.Sprintf("⚠ Has %d unpushed commit(s)", status.UnpushedCount))

		// Show commits
		commits, _ := branchMgr.GetUnpushedCommits(branch, 5)
		for _, commit := range commits {
			details = append(details, "  "+commit)
		}
		if status.UnpushedCount > 5 {
			details = append(details, fmt.Sprintf("  ... and %d more", status.UnpushedCount-5))
		}
	}

	if status.IsMerged && status.UnpushedCount == 0 {
		details = append(details, "✓ Branch is merged and safe to delete")
	}

	header := fmt.Sprintf("\nBranch '%s':", branch)

	// Ask for confirmation
	confirmed, err := prompt.ConfirmWithDetails(header, details, "Delete branch?", true)
	if err != nil {
		// Non-interactive - provide guidance
		fmt.Printf("ℹ Branch '%s' not deleted (use --delete-branch or --keep-branch)\n", branch)
		return nil
	}

	if confirmed {
		// Use force delete if branch is not merged
		needsForce := !status.IsMerged || forceUnmerged
		if err := branchMgr.Delete(branch, needsForce); err != nil {
			return fmt.Errorf("failed to delete branch: %w", err)
		}
		fmt.Printf("✓ Deleted branch '%s'\n", branch)
	} else {
		fmt.Printf("ℹ Kept branch '%s'\n", branch)
	}

	return nil
}

func init() {
	rmCmd.Flags().BoolVarP(&rmForce, "force", "f", false, "Force removal (required with --unprotect for protected worktrees, allows deleting unmerged branches)")
	rmCmd.Flags().BoolVar(&rmUnprotect, "unprotect", false, "Allow removing protected worktrees (requires --force)")
	rmCmd.Flags().BoolVar(&rmDryRun, "dry-run", false, "Show what would be removed without making changes")
	rmCmd.Flags().BoolVar(&rmKeepBranch, "keep-branch", false, "Do not delete the associated branch")
	rmCmd.Flags().BoolVar(&rmDeleteBranch, "delete-branch", false, "Delete the associated branch without prompting")
	rmCmd.MarkFlagsMutuallyExclusive("keep-branch", "delete-branch")
	rootCmd.AddCommand(rmCmd)
}
