package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lost-in-the/grove/internal/tui"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/tracker"
)

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
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		useFzf, _ := cmd.Flags().GetBool("fzf")

		if !useFzf && term.IsTerminal(int(os.Stdin.Fd())) && os.Getenv("GROVE_TUI") != "0" {
			mgr, err := worktree.NewManager(ctx.ProjectRoot)
			if err != nil {
				return fmt.Errorf("failed to initialize worktree manager: %w", err)
			}
			_, _, err = tui.RunIssues(mgr, ctx.State, ctx.ProjectRoot, ctx.PluginManager)
			return err
		}

		return browseIssuesFzf(cmd, ctx)
	}),
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
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		useFzf, _ := cmd.Flags().GetBool("fzf")

		if !useFzf && term.IsTerminal(int(os.Stdin.Fd())) && os.Getenv("GROVE_TUI") != "0" {
			mgr, err := worktree.NewManager(ctx.ProjectRoot)
			if err != nil {
				return fmt.Errorf("failed to initialize worktree manager: %w", err)
			}
			_, _, err = tui.RunPRs(mgr, ctx.State, ctx.ProjectRoot, ctx.PluginManager)
			return err
		}

		return browsePRsFzf(cmd, ctx)
	}),
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

	rootCmd.AddCommand(issuesCmd)
	rootCmd.AddCommand(prsCmd)
}
