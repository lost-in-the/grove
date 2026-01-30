package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/LeahArmstrong/grove-cli/plugins/tracker"
	"github.com/spf13/cobra"
)

// fetchItem creates a worktree from a GitHub PR or issue.
// itemType must be "pr" or "issue". Called by both `grove fetch` and `grove prs`/`grove issues`.
func fetchItem(ctx *GroveContext, itemType string, number int) error {
	// Check if gh CLI is available
	if !tracker.IsGHInstalled() {
		return fmt.Errorf("gh CLI not installed or not authenticated\n\nInstall: https://cli.github.com/\nAuthenticate: gh auth login")
	}

	// Detect repository
	repo, err := tracker.DetectRepo()
	if err != nil {
		return fmt.Errorf("failed to detect repository: %w\n\nMake sure you're in a git repository with a GitHub remote", err)
	}

	// Create GitHub adapter
	gh := tracker.NewGitHubAdapter(repo)

	// Initialize worktree manager
	mgr, err := worktree.NewManager(ctx.ProjectRoot)
	if err != nil {
		return fmt.Errorf("failed to initialize worktree manager: %w", err)
	}

	// Fetch metadata and create worktree
	var worktreeName string
	var branchName string

	if itemType == "pr" {
		pr, err := gh.FetchPR(number)
		if err != nil {
			return fmt.Errorf("failed to fetch PR #%d: %w", number, err)
		}

		fmt.Printf("Fetching PR #%d: %s\n", pr.Number, pr.Title)

		worktreeName = tracker.GenerateWorktreeName("pr", pr.Number, pr.Title)
		branchName = pr.Branch

		if existingWt, _ := mgr.Find(worktreeName); existingWt != nil {
			return fmt.Errorf("worktree '%s' already exists\n\nOptions:\n  • Switch to it: grove to %s\n  • Remove it first: grove rm %s",
				worktreeName, worktreeName, worktreeName)
		}

		if err := mgr.CreateFromBranch(worktreeName, branchName); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}

		fmt.Printf("✓ Created worktree '%s' from branch '%s'\n", worktreeName, branchName)

	} else {
		issue, err := gh.FetchIssue(number)
		if err != nil {
			return fmt.Errorf("failed to fetch issue #%d: %w", number, err)
		}

		fmt.Printf("Fetching issue #%d: %s\n", issue.Number, issue.Title)

		worktreeName = tracker.GenerateWorktreeName("issue", issue.Number, issue.Title)
		branchName = worktreeName

		if existingWt, _ := mgr.Find(worktreeName); existingWt != nil {
			return fmt.Errorf("worktree '%s' already exists\n\nOptions:\n  • Switch to it: grove to %s\n  • Remove it first: grove rm %s",
				worktreeName, worktreeName, worktreeName)
		}

		if err := mgr.Create(worktreeName, branchName); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}

		fmt.Printf("✓ Created worktree '%s' with new branch '%s'\n", worktreeName, branchName)
	}

	// Get the created worktree
	wt, err := mgr.Find(worktreeName)
	if err != nil {
		return fmt.Errorf("failed to find created worktree: %w", err)
	}

	// Register worktree in state
	if wt != nil {
		now := time.Now()
		wsState := &state.WorktreeState{
			Path:           wt.Path,
			Branch:         branchName,
			CreatedAt:      now,
			LastAccessedAt: now,
		}
		_ = ctx.State.AddWorktree(worktreeName, wsState)
	}

	// Create tmux session if available
	if tmux.IsTmuxAvailable() && wt != nil {
		projectName := mgr.GetProjectName()
		sessionName := worktree.TmuxSessionName(projectName, worktreeName)
		if err := tmux.CreateSession(sessionName, wt.Path); err != nil {
			fmt.Printf("⚠ Failed to create tmux session: %v\n", err)
		} else {
			fmt.Printf("✓ Created tmux session '%s'\n", sessionName)
		}
	}

	// Get current worktree to update last_worktree
	currentTree, _ := mgr.GetCurrent()
	if currentTree != nil {
		_ = ctx.State.SetLastWorktree(currentTree.DisplayName())
	}

	// Switch to the worktree if shell integration is active
	hasShellIntegration := os.Getenv("GROVE_SHELL") == "1"
	if hasShellIntegration && wt != nil {
		fmt.Printf("cd:%s\n", wt.Path)
	} else {
		fmt.Printf("\nTo switch to this worktree:\n  grove to %s\n", worktreeName)
	}

	return nil
}

var fetchCmd = &cobra.Command{
	Use:   "fetch <pr|issue>/<number>",
	Short: "Create worktree from issue or PR",
	Long: `Create a new worktree from a GitHub issue or pull request.

Examples:
  grove fetch pr/123     # Create worktree from PR #123
  grove fetch issue/456  # Create worktree from issue #456
  grove fetch is/456     # Shorthand for issue

The worktree name will be automatically generated from the issue/PR metadata.
For PRs, the remote branch will be checked out.`,
	Args: cobra.ExactArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		// Parse argument: pr/123 or issue/456
		parts := strings.SplitN(args[0], "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid format: use 'pr/<number>' or 'issue/<number>'")
		}

		itemType := strings.ToLower(parts[0])
		numberStr := parts[1]

		// Validate type
		if itemType != "pr" && itemType != "issue" && itemType != "is" {
			return fmt.Errorf("invalid type %q: use 'pr', 'issue', or 'is'", itemType)
		}

		// Normalize "is" to "issue"
		if itemType == "is" {
			itemType = "issue"
		}

		// Parse number
		number, err := strconv.Atoi(numberStr)
		if err != nil {
			return fmt.Errorf("invalid number %q: %w", numberStr, err)
		}

		return fetchItem(ctx, itemType, number)
	}),
}

func init() {
	rootCmd.AddCommand(fetchCmd)
}
