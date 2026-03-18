package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/log"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/tracker"
)

func fetchItemMetadata(gh *tracker.GitHubAdapter, w *cli.Writer, itemType string, number int) (worktreeName, branchName string, err error) {
	if itemType == "pr" {
		pr, ferr := gh.FetchPR(number)
		if ferr != nil {
			return "", "", fmt.Errorf("failed to fetch PR #%d: %w", number, ferr)
		}
		cli.Step(w, "Fetching PR #%d: %s", pr.Number, pr.Title)
		return tracker.GenerateWorktreeName("pr", pr.Number, pr.Title), pr.Branch, nil
	}

	issue, err := gh.FetchIssue(number)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch issue #%d: %w", number, err)
	}
	cli.Step(w, "Fetching issue #%d: %s", issue.Number, issue.Title)
	name := tracker.GenerateWorktreeName("issue", issue.Number, issue.Title)
	return name, name, nil
}

func createFetchedWorktree(mgr *worktree.Manager, itemType, worktreeName, branchName string) error {
	if existingWt, _ := mgr.Find(worktreeName); existingWt != nil {
		return fmt.Errorf("worktree '%s' already exists\n\nOptions:\n  • Switch to it: grove to %s\n  • Remove it first: grove rm %s",
			worktreeName, worktreeName, worktreeName)
	}

	if itemType == "pr" {
		return mgr.CreateFromBranch(worktreeName, branchName)
	}
	return mgr.Create(worktreeName, branchName)
}

func setupFetchedWorktree(ctx *GroveContext, mgr *worktree.Manager, w *cli.Writer, wt *worktree.Worktree, worktreeName, branchName string) {
	now := time.Now()
	wsState := &state.WorktreeState{
		Path:           wt.Path,
		Branch:         branchName,
		CreatedAt:      now,
		LastAccessedAt: now,
	}
	if err := ctx.State.AddWorktree(worktreeName, wsState); err != nil {
		log.Printf("failed to add worktree %q to state: %v", worktreeName, err)
	}

	if tmux.IsTmuxAvailable() {
		projectName := mgr.GetProjectName()
		sessionName := worktree.TmuxSessionName(projectName, worktreeName)
		if err := tmux.CreateSession(sessionName, wt.Path); err != nil {
			cli.Warning(w, "Failed to create tmux session: %v", err)
		} else {
			cli.Success(w, "Created tmux session '%s'", sessionName)
		}
	}

	currentTree, _ := mgr.GetCurrent()
	if currentTree != nil {
		if err := ctx.State.SetLastWorktree(currentTree.DisplayName()); err != nil {
			log.Printf("failed to set last worktree %q: %v", currentTree.DisplayName(), err)
		}
	}

	if os.Getenv("GROVE_SHELL") == "1" {
		cli.Directive("cd", wt.Path)
	} else {
		fmt.Printf("\nTo switch to this worktree:\n  grove to %s\n", worktreeName)
	}
}

// fetchItem creates a worktree from a GitHub PR or issue.
// itemType must be "pr" or "issue". Called by both `grove fetch` and `grove prs`/`grove issues`.
func fetchItem(ctx *GroveContext, itemType string, number int) error {
	w := cli.NewStdout()

	if !tracker.IsGHInstalled() {
		return fmt.Errorf("gh CLI not installed or not authenticated\n\nInstall: https://cli.github.com/\nAuthenticate: gh auth login")
	}

	repo, err := tracker.DetectRepo()
	if err != nil {
		return fmt.Errorf("failed to detect repository: %w\n\nMake sure you're in a git repository with a GitHub remote", err)
	}

	gh := tracker.NewGitHubAdapter(repo)

	mgr, err := worktree.NewManager(ctx.ProjectRoot)
	if err != nil {
		return fmt.Errorf("failed to initialize worktree manager: %w", err)
	}

	worktreeName, branchName, err := fetchItemMetadata(gh, w, itemType, number)
	if err != nil {
		return err
	}

	if err := createFetchedWorktree(mgr, itemType, worktreeName, branchName); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	if itemType == "pr" {
		cli.Success(w, "Created worktree '%s' from branch '%s'", worktreeName, branchName)
	} else {
		cli.Success(w, "Created worktree '%s' with new branch '%s'", worktreeName, branchName)
	}

	wt, err := mgr.Find(worktreeName)
	if err != nil {
		return fmt.Errorf("failed to find created worktree: %w", err)
	}
	if wt != nil {
		setupFetchedWorktree(ctx, mgr, w, wt, worktreeName, branchName)
	}

	return nil
}

var fetchCmd = &cobra.Command{
	Use:     "fetch <pr|issue>/<number>",
	Aliases: []string{"f"},
	Short:   "Create worktree from issue or PR",
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
