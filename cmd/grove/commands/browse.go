package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/plugins"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tui"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/tracker"
)

func browseRunE(
	runTUI func(*worktree.Manager, *state.Manager, string, ...*plugins.Manager) (string, bool, error),
	fzfFallback func(*cobra.Command, *GroveContext) error,
) func(*cobra.Command, []string) error {
	return RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		useFzf, _ := cmd.Flags().GetBool("fzf")

		if !useFzf && term.IsTerminal(int(os.Stdin.Fd())) && os.Getenv("GROVE_TUI") != "0" {
			mgr, err := worktree.NewManager(ctx.ProjectRoot)
			if err != nil {
				return fmt.Errorf("failed to initialize worktree manager: %w", err)
			}
			_, _, err = runTUI(mgr, ctx.State, ctx.ProjectRoot, ctx.PluginManager)
			return err
		}

		return fzfFallback(cmd, ctx)
	})
}

var issuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "Browse and select issues",
	Long: `Browse GitHub issues using an interactive TUI (default) or fzf.

Use arrow keys to navigate, Enter to select an issue.
Selected issue will create a new worktree.

Examples:
  grove issues              # TUI issue browser
  grove issues --fzf        # Use fzf for selection
  grove issues --state all  # Include closed issues
  grove issues --label bug  # Filter by label`,
	RunE: browseRunE(tui.RunIssues, browseIssuesFzf),
}

var prsCmd = &cobra.Command{
	Use:   "prs",
	Short: "Browse and select pull requests",
	Long: `Browse GitHub pull requests using an interactive TUI (default) or fzf.

Use arrow keys to navigate, Enter to select a PR.
Selected PR will create a new worktree.

Examples:
  grove prs                # TUI PR browser
  grove prs --fzf          # Use fzf for selection
  grove prs --state all    # Include closed PRs
  grove prs --label feature  # Filter by label`,
	RunE: browseRunE(tui.RunPRs, browsePRsFzf),
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// fzfSelect runs fzf with the provided lines and returns the selected number parsed
// from the leading "#<number>" field. Returns -1 with nil error if the user cancels.
func fzfSelect(header string, lines []string) (int, error) {
	fzfCmd := exec.Command("fzf",
		"--ansi",
		"--header="+header,
		"--preview=echo {}",
		"--preview-window=up:3:wrap",
		"--height=50%",
		"--border",
	)

	stdin, err := fzfCmd.StdinPipe()
	if err != nil {
		return 0, fmt.Errorf("failed to create fzf stdin pipe: %w", err)
	}

	stdout, err := fzfCmd.StdoutPipe()
	if err != nil {
		return 0, fmt.Errorf("failed to create fzf stdout pipe: %w", err)
	}

	fzfCmd.Stderr = os.Stderr

	if err := fzfCmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start fzf: %w", err)
	}

	writer := bufio.NewWriter(stdin)
	for _, line := range lines {
		_, _ = writer.WriteString(line)
	}
	_ = writer.Flush()
	_ = stdin.Close()

	scanner := bufio.NewScanner(stdout)
	var selection string
	if scanner.Scan() {
		selection = scanner.Text()
	}

	if err := fzfCmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 130 {
				return -1, nil
			}
		}
		return 0, fmt.Errorf("fzf selection failed: %w", err)
	}

	if selection == "" {
		return -1, nil
	}

	parts := strings.Split(selection, "|")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid selection format")
	}

	numberStr := strings.TrimSpace(strings.TrimPrefix(parts[0], "#"))
	number, err := strconv.Atoi(numberStr)
	if err != nil {
		return 0, fmt.Errorf("invalid number in selection: %w", err)
	}
	return number, nil
}

// listOptsFromCmd extracts shared filter flags from a command.
func listOptsFromCmd(cmd *cobra.Command) tracker.ListOptions {
	state, _ := cmd.Flags().GetString("state")
	labels, _ := cmd.Flags().GetStringSlice("label")
	assignee, _ := cmd.Flags().GetString("assignee")
	author, _ := cmd.Flags().GetString("author")
	limit, _ := cmd.Flags().GetInt("limit")
	return tracker.ListOptions{
		State:    state,
		Labels:   labels,
		Assignee: assignee,
		Author:   author,
		Limit:    limit,
	}
}

// checkFzfPrereqs verifies that gh and fzf are available.
func checkFzfPrereqs() error {
	if !tracker.IsGHInstalled() {
		return fmt.Errorf("gh CLI not installed or not authenticated\n\nInstall: https://cli.github.com/\nAuthenticate: gh auth login")
	}
	if _, err := exec.LookPath("fzf"); err != nil {
		return fmt.Errorf("fzf not installed\n\nInstall: https://github.com/junegunn/fzf#installation")
	}
	return nil
}

func browseIssuesFzf(cmd *cobra.Command, ctx *GroveContext) error {
	if err := checkFzfPrereqs(); err != nil {
		return err
	}

	repo, err := tracker.DetectRepo()
	if err != nil {
		return fmt.Errorf("failed to detect repository: %w\n\nMake sure you're in a git repository with a GitHub remote", err)
	}

	gh := tracker.NewGitHubAdapter(repo)

	fmt.Fprintf(os.Stderr, "Fetching issues from %s...\n", repo)
	issues, err := gh.ListIssues(listOptsFromCmd(cmd))
	if err != nil {
		return fmt.Errorf("failed to list issues: %w", err)
	}

	if len(issues) == 0 {
		fmt.Println("No issues found")
		return nil
	}

	lines := make([]string, len(issues))
	for i, issue := range issues {
		lines[i] = fmt.Sprintf("#%-6d | %-60s | %-6s | @%s\n",
			issue.Number,
			truncate(issue.Title, 60),
			issue.State,
			issue.Author,
		)
	}

	number, err := fzfSelect("Select an issue (Ctrl-C to cancel)", lines)
	if err != nil {
		return err
	}
	if number < 0 {
		return nil
	}

	return fetchItem(ctx, "issue", number)
}

func browsePRsFzf(cmd *cobra.Command, ctx *GroveContext) error {
	if err := checkFzfPrereqs(); err != nil {
		return err
	}

	repo, err := tracker.DetectRepo()
	if err != nil {
		return fmt.Errorf("failed to detect repository: %w\n\nMake sure you're in a git repository with a GitHub remote", err)
	}

	gh := tracker.NewGitHubAdapter(repo)

	fmt.Fprintf(os.Stderr, "Fetching PRs from %s...\n", repo)
	prs, err := gh.ListPRs(listOptsFromCmd(cmd))
	if err != nil {
		return fmt.Errorf("failed to list PRs: %w", err)
	}

	if len(prs) == 0 {
		fmt.Println("No PRs found")
		return nil
	}

	lines := make([]string, len(prs))
	for i, pr := range prs {
		lines[i] = fmt.Sprintf("#%-6d | %-50s | %-20s | %-6s | @%s\n",
			pr.Number,
			truncate(pr.Title, 50),
			truncate(pr.Branch, 20),
			pr.State,
			pr.Author,
		)
	}

	number, err := fzfSelect("Select a PR (Ctrl-C to cancel)", lines)
	if err != nil {
		return err
	}
	if number < 0 {
		return nil
	}

	return fetchItem(ctx, "pr", number)
}

// browseURLOutput is the JSON output for grove browse --json.
type browseURLOutput struct {
	URL    string `json:"url"`
	Source string `json:"source"` // "pr", "issue", or "compare"
}

// openBrowserURL opens a URL in the system default browser cross-platform.
func openBrowserURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// parseIssueNumberFromWorktreeName extracts an issue number from worktree names
// following the pattern "issue-<number>-<slug>" (generated by grove fetch issue/<n>).
// Returns 0 if no issue number is found.
func parseIssueNumberFromWorktreeName(shortName string) int {
	if !strings.HasPrefix(shortName, "issue-") {
		return 0
	}
	rest := strings.TrimPrefix(shortName, "issue-")
	dashIdx := strings.Index(rest, "-")
	var numStr string
	if dashIdx < 0 {
		numStr = rest
	} else {
		numStr = rest[:dashIdx]
	}
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0
	}
	return n
}

var browseCmd = &cobra.Command{
	Use:     "browse",
	Aliases: []string{"b"},
	Short:   "Open the current worktree's PR or issue in the browser",
	Long: `Open the associated GitHub PR or issue for the current worktree in your default browser.

Resolution order:
  1. If the current branch has an open PR, open that PR.
  2. If the worktree was created from an issue (name matches "issue-<number>-..."), open that issue.
  3. Otherwise, open the branch compare page on GitHub.

Examples:
  grove browse          # Open PR/issue/compare page in browser
  grove browse --json   # Print the URL without opening the browser`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		jsonMode, _ := cmd.Flags().GetBool("json")

		w := cli.NewStdout()

		if !tracker.IsGHInstalled() {
			return fmt.Errorf("gh CLI not installed or not authenticated\n\nInstall: https://cli.github.com/\nAuthenticate: gh auth login")
		}

		repo, err := tracker.DetectRepo()
		if err != nil {
			return fmt.Errorf("failed to detect repository: %w\n\nMake sure you're in a git repository with a GitHub remote", err)
		}

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		tree, err := mgr.GetCurrent()
		if err != nil {
			return fmt.Errorf("failed to get current worktree: %w", err)
		}
		if tree == nil {
			return fmt.Errorf("not in a grove worktree")
		}

		gh := tracker.NewGitHubAdapter(repo)

		// 1. Check for an open PR on the current branch.
		pr, err := gh.GetPRForBranch(tree.Branch)
		if err != nil {
			return fmt.Errorf("failed to look up PR: %w", err)
		}
		if pr != nil {
			result := browseURLOutput{URL: pr.URL, Source: "pr"}
			if jsonMode {
				return output.PrintJSON(result)
			}
			cli.Info(w, "Opening PR #%d: %s", pr.Number, pr.Title)
			if openErr := openBrowserURL(pr.URL); openErr != nil {
				return fmt.Errorf("failed to open browser: %w", openErr)
			}
			return nil
		}

		// 2. Check if the worktree was created from an issue.
		issueNumber := parseIssueNumberFromWorktreeName(tree.ShortName)
		if issueNumber > 0 {
			issue, fetchErr := gh.FetchIssue(issueNumber)
			if fetchErr == nil && issue != nil {
				result := browseURLOutput{URL: issue.URL, Source: "issue"}
				if jsonMode {
					return output.PrintJSON(result)
				}
				cli.Info(w, "Opening issue #%d: %s", issue.Number, issue.Title)
				if openErr := openBrowserURL(issue.URL); openErr != nil {
					return fmt.Errorf("failed to open browser: %w", openErr)
				}
				return nil
			}
		}

		// 3. Fall back to the branch compare page.
		compareURL := "https://github.com/" + repo + "/compare/" + tree.Branch
		result := browseURLOutput{URL: compareURL, Source: "compare"}
		if jsonMode {
			return output.PrintJSON(result)
		}
		cli.Info(w, "No PR found — opening compare page for branch '%s'", tree.Branch)
		if openErr := openBrowserURL(compareURL); openErr != nil {
			return fmt.Errorf("failed to open browser: %w", openErr)
		}
		return nil
	}),
}

func init() {
	// Issues flags
	issuesCmd.Flags().Bool("fzf", false, "Use fzf for selection instead of TUI")
	issuesCmd.Flags().String("state", "open", "Filter by state (open, closed, all)")
	issuesCmd.Flags().StringSlice("label", nil, "Filter by labels")
	issuesCmd.Flags().String("assignee", "", "Filter by assignee")
	issuesCmd.Flags().String("author", "", "Filter by author")
	issuesCmd.Flags().Int("limit", 30, "Maximum number of issues to fetch")

	// PRs flags
	prsCmd.Flags().Bool("fzf", false, "Use fzf for selection instead of TUI")
	prsCmd.Flags().String("state", "open", "Filter by state (open, closed, all)")
	prsCmd.Flags().StringSlice("label", nil, "Filter by labels")
	prsCmd.Flags().String("assignee", "", "Filter by assignee")
	prsCmd.Flags().String("author", "", "Filter by author")
	prsCmd.Flags().Int("limit", 30, "Maximum number of PRs to fetch")

	// Browse flags
	browseCmd.Flags().Bool("json", false, "Output the URL as JSON without opening the browser")

	rootCmd.AddCommand(issuesCmd)
	rootCmd.AddCommand(prsCmd)
	rootCmd.AddCommand(browseCmd)
}
