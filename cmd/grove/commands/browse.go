package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/LeahArmstrong/grove-cli/plugins/tracker"
	"github.com/spf13/cobra"
)

var issuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "Browse and select issues with fzf",
	Long: `Browse GitHub issues using fzf for interactive selection.

Use arrow keys to navigate, Enter to select an issue.
Selected issue will create a new worktree.

Examples:
  grove issues          # Browse all open issues
  grove issues --state all  # Include closed issues
  grove issues --label bug  # Filter by label`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		// Get flags
		state, _ := cmd.Flags().GetString("state")
		labels, _ := cmd.Flags().GetStringSlice("label")
		assignee, _ := cmd.Flags().GetString("assignee")
		author, _ := cmd.Flags().GetString("author")
		limit, _ := cmd.Flags().GetInt("limit")

		// Check if gh CLI is available
		if !tracker.IsGHInstalled() {
			return fmt.Errorf("gh CLI not installed or not authenticated\n\nInstall: https://cli.github.com/\nAuthenticate: gh auth login")
		}

		// Check if fzf is available
		if _, err := exec.LookPath("fzf"); err != nil {
			return fmt.Errorf("fzf not installed\n\nInstall: https://github.com/junegunn/fzf#installation")
		}

		// Detect repository
		repo, err := tracker.DetectRepo()
		if err != nil {
			return fmt.Errorf("failed to detect repository: %w\n\nMake sure you're in a git repository with a GitHub remote", err)
		}

		// Create GitHub adapter
		gh := tracker.NewGitHubAdapter(repo)

		// List issues
		opts := tracker.ListOptions{
			State:    state,
			Labels:   labels,
			Assignee: assignee,
			Author:   author,
			Limit:    limit,
		}

		fmt.Fprintf(os.Stderr, "Fetching issues from %s...\n", repo)
		issues, err := gh.ListIssues(opts)
		if err != nil {
			return fmt.Errorf("failed to list issues: %w", err)
		}

		if len(issues) == 0 {
			fmt.Println("No issues found")
			return nil
		}

		// Prepare fzf input with pipe for capturing selection
		fzfCmd := exec.Command("fzf",
			"--ansi",
			"--header=Select an issue (Ctrl-C to cancel)",
			"--preview=echo {}",
			"--preview-window=up:3:wrap",
			"--height=50%",
			"--border",
		)

		// Create pipe for input
		stdin, err := fzfCmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("failed to create fzf stdin pipe: %w", err)
		}

		// Capture fzf output
		stdout, err := fzfCmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create fzf stdout pipe: %w", err)
		}

		fzfCmd.Stderr = os.Stderr

		// Start fzf
		if err := fzfCmd.Start(); err != nil {
			return fmt.Errorf("failed to start fzf: %w", err)
		}

		// Write issues to fzf
		writer := bufio.NewWriter(stdin)
		for _, issue := range issues {
			// Format: #123 | Title | state | @author
			line := fmt.Sprintf("#%-6d | %-60s | %-6s | @%s\n",
				issue.Number,
				truncate(issue.Title, 60),
				issue.State,
				issue.Author,
			)
			writer.WriteString(line)
		}
		writer.Flush()
		stdin.Close()

		// Read selected issue
		scanner := bufio.NewScanner(stdout)
		var selection string
		if scanner.Scan() {
			selection = scanner.Text()
		}

		// Wait for fzf to complete
		if err := fzfCmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				// Exit code 130 means user cancelled with Ctrl-C
				if exitErr.ExitCode() == 130 {
					return nil
				}
			}
			return fmt.Errorf("fzf selection failed: %w", err)
		}

		if selection == "" {
			return nil
		}

		// Parse selection to extract issue number
		// Format: #123 | ...
		parts := strings.Split(selection, "|")
		if len(parts) == 0 {
			return fmt.Errorf("invalid selection format")
		}

		numberStr := strings.TrimSpace(strings.TrimPrefix(parts[0], "#"))
		number, err := strconv.Atoi(numberStr)
		if err != nil {
			return fmt.Errorf("invalid issue number: %w", err)
		}

		// Run fetch command with the selected issue
		fetchCmd.SetArgs([]string{fmt.Sprintf("issue/%d", number)})
		return fetchCmd.Execute()
	}),
}

var prsCmd = &cobra.Command{
	Use:   "prs",
	Short: "Browse and select pull requests with fzf",
	Long: `Browse GitHub pull requests using fzf for interactive selection.

Use arrow keys to navigate, Enter to select a PR.
Selected PR will create a new worktree.

Examples:
  grove prs             # Browse all open PRs
  grove prs --state all # Include closed PRs
  grove prs --label feature  # Filter by label`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		// Get flags
		state, _ := cmd.Flags().GetString("state")
		labels, _ := cmd.Flags().GetStringSlice("label")
		assignee, _ := cmd.Flags().GetString("assignee")
		author, _ := cmd.Flags().GetString("author")
		limit, _ := cmd.Flags().GetInt("limit")

		// Check if gh CLI is available
		if !tracker.IsGHInstalled() {
			return fmt.Errorf("gh CLI not installed or not authenticated\n\nInstall: https://cli.github.com/\nAuthenticate: gh auth login")
		}

		// Check if fzf is available
		if _, err := exec.LookPath("fzf"); err != nil {
			return fmt.Errorf("fzf not installed\n\nInstall: https://github.com/junegunn/fzf#installation")
		}

		// Detect repository
		repo, err := tracker.DetectRepo()
		if err != nil {
			return fmt.Errorf("failed to detect repository: %w\n\nMake sure you're in a git repository with a GitHub remote", err)
		}

		// Create GitHub adapter
		gh := tracker.NewGitHubAdapter(repo)

		// List PRs
		opts := tracker.ListOptions{
			State:    state,
			Labels:   labels,
			Assignee: assignee,
			Author:   author,
			Limit:    limit,
		}

		fmt.Fprintf(os.Stderr, "Fetching PRs from %s...\n", repo)
		prs, err := gh.ListPRs(opts)
		if err != nil {
			return fmt.Errorf("failed to list PRs: %w", err)
		}

		if len(prs) == 0 {
			fmt.Println("No PRs found")
			return nil
		}

		// Prepare fzf input with pipe for capturing selection
		fzfCmd := exec.Command("fzf",
			"--ansi",
			"--header=Select a PR (Ctrl-C to cancel)",
			"--preview=echo {}",
			"--preview-window=up:3:wrap",
			"--height=50%",
			"--border",
		)

		// Create pipe for input
		stdin, err := fzfCmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("failed to create fzf stdin pipe: %w", err)
		}

		// Capture fzf output
		stdout, err := fzfCmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create fzf stdout pipe: %w", err)
		}

		fzfCmd.Stderr = os.Stderr

		// Start fzf
		if err := fzfCmd.Start(); err != nil {
			return fmt.Errorf("failed to start fzf: %w", err)
		}

		// Write PRs to fzf
		writer := bufio.NewWriter(stdin)
		for _, pr := range prs {
			// Format: #123 | Title | branch | state | @author
			line := fmt.Sprintf("#%-6d | %-50s | %-20s | %-6s | @%s\n",
				pr.Number,
				truncate(pr.Title, 50),
				truncate(pr.Branch, 20),
				pr.State,
				pr.Author,
			)
			writer.WriteString(line)
		}
		writer.Flush()
		stdin.Close()

		// Read selected PR
		scanner := bufio.NewScanner(stdout)
		var selection string
		if scanner.Scan() {
			selection = scanner.Text()
		}

		// Wait for fzf to complete
		if err := fzfCmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				// Exit code 130 means user cancelled with Ctrl-C
				if exitErr.ExitCode() == 130 {
					return nil
				}
			}
			return fmt.Errorf("fzf selection failed: %w", err)
		}

		if selection == "" {
			return nil
		}

		// Parse selection to extract PR number
		// Format: #123 | ...
		parts := strings.Split(selection, "|")
		if len(parts) == 0 {
			return fmt.Errorf("invalid selection format")
		}

		numberStr := strings.TrimSpace(strings.TrimPrefix(parts[0], "#"))
		number, err := strconv.Atoi(numberStr)
		if err != nil {
			return fmt.Errorf("invalid PR number: %w", err)
		}

		// Run fetch command with the selected PR
		fetchCmd.SetArgs([]string{fmt.Sprintf("pr/%d", number)})
		return fetchCmd.Execute()
	}),
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func init() {
	// Add flags to issues command
	issuesCmd.Flags().String("state", "open", "Filter by state (open, closed, all)")
	issuesCmd.Flags().StringSlice("label", nil, "Filter by labels")
	issuesCmd.Flags().String("assignee", "", "Filter by assignee")
	issuesCmd.Flags().String("author", "", "Filter by author")
	issuesCmd.Flags().Int("limit", 30, "Maximum number of issues to fetch")

	// Add flags to prs command
	prsCmd.Flags().String("state", "open", "Filter by state (open, closed, all)")
	prsCmd.Flags().StringSlice("label", nil, "Filter by labels")
	prsCmd.Flags().String("assignee", "", "Filter by assignee")
	prsCmd.Flags().String("author", "", "Filter by author")
	prsCmd.Flags().Int("limit", 30, "Maximum number of PRs to fetch")

	rootCmd.AddCommand(issuesCmd)
	rootCmd.AddCommand(prsCmd)
}
