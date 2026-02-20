package commands

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

var (
	compareStat      bool
	compareCommitted bool
	compareWIP       bool
	compareJSON      bool
)

// CompareResult represents the JSON output for grove compare.
type CompareResult struct {
	Current string       `json:"current"`
	Target  string       `json:"target"`
	Commits []CommitDiff `json:"commits,omitempty"`
	WIP     *WIPDiff     `json:"wip,omitempty"`
	Stats   *DiffStats   `json:"stats,omitempty"`
	HasDiff bool         `json:"has_diff"`
}

type CommitDiff struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Age     string `json:"age"`
}

type WIPDiff struct {
	Staged    []string `json:"staged,omitempty"`
	Unstaged  []string `json:"unstaged,omitempty"`
	Untracked []string `json:"untracked,omitempty"`
}

type DiffStats struct {
	FilesChanged int `json:"files_changed"`
	Insertions   int `json:"insertions"`
	Deletions    int `json:"deletions"`
}

var compareCmd = &cobra.Command{
	Use:   "compare <name>",
	Short: "Compare current worktree with another",
	Long: `Compare the current worktree with another worktree.

Shows differences in commits and optionally uncommitted changes (WIP).
By default shows both committed and uncommitted differences.

Examples:
  grove compare main           # Compare with main worktree
  grove compare feature --stat # Show only diffstat
  grove compare main --committed  # Show only commit differences
  grove compare main --wip     # Show only uncommitted differences
  grove compare main --json    # Output as JSON`,
	Args: cobra.ExactArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		targetName := args[0]

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		// Get current worktree
		currentTree, err := mgr.GetCurrent()
		if err != nil {
			return fmt.Errorf("failed to get current worktree: %w", err)
		}
		if currentTree == nil {
			return fmt.Errorf("could not determine current worktree")
		}

		// Find target worktree
		targetTree, err := mgr.Find(targetName)
		if err != nil {
			return fmt.Errorf("failed to find worktree: %w", err)
		}
		if targetTree == nil {
			return fmt.Errorf("worktree '%s' not found", targetName)
		}

		result := CompareResult{
			Current: currentTree.DisplayName(),
			Target:  targetTree.DisplayName(),
		}

		// Determine what to compare
		showCommits := !compareWIP || compareCommitted
		showWIP := !compareCommitted || compareWIP

		// If neither flag specified, show both
		if !compareCommitted && !compareWIP {
			showCommits = true
			showWIP = true
		}

		// Compare commits
		if showCommits {
			commits, err := getCommitDifference(currentTree.Path, targetTree.Branch)
			if err != nil {
				return fmt.Errorf("failed to get commit difference: %w", err)
			}
			result.Commits = commits
			if len(commits) > 0 {
				result.HasDiff = true
			}
		}

		// Compare WIP (uncommitted changes)
		if showWIP {
			wipDiff, err := getWIPDifference(currentTree.Path, targetTree.Path)
			if err != nil {
				return fmt.Errorf("failed to get WIP difference: %w", err)
			}
			if wipDiff != nil && (len(wipDiff.Staged) > 0 || len(wipDiff.Unstaged) > 0 || len(wipDiff.Untracked) > 0) {
				result.WIP = wipDiff
				result.HasDiff = true
			}
		}

		// Get stats if --stat flag
		if compareStat {
			stats, err := getDiffStats(currentTree.Path, targetTree.Branch)
			if err != nil {
				return fmt.Errorf("failed to get diff stats: %w", err)
			}
			result.Stats = stats
		}

		// JSON output
		if compareJSON {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		// Human-readable output
		fmt.Printf("Comparing %s → %s\n", result.Current, result.Target)
		fmt.Println(strings.Repeat("━", 50))

		if !result.HasDiff {
			fmt.Println("No differences found")
			return nil
		}

		// Show commits
		if len(result.Commits) > 0 {
			fmt.Printf("\nCommits ahead of %s (%d):\n", result.Target, len(result.Commits))
			for _, commit := range result.Commits {
				fmt.Printf("  %s %s (%s)\n", commit.SHA[:7], commit.Message, commit.Age)
			}
		}

		// Show WIP
		if result.WIP != nil {
			fmt.Println("\nUncommitted changes:")
			if len(result.WIP.Staged) > 0 {
				fmt.Printf("  Staged (%d):\n", len(result.WIP.Staged))
				for _, f := range result.WIP.Staged {
					fmt.Printf("    + %s\n", f)
				}
			}
			if len(result.WIP.Unstaged) > 0 {
				fmt.Printf("  Unstaged (%d):\n", len(result.WIP.Unstaged))
				for _, f := range result.WIP.Unstaged {
					fmt.Printf("    M %s\n", f)
				}
			}
			if len(result.WIP.Untracked) > 0 {
				fmt.Printf("  Untracked (%d):\n", len(result.WIP.Untracked))
				for _, f := range result.WIP.Untracked {
					fmt.Printf("    ? %s\n", f)
				}
			}
		}

		// Show stats
		if result.Stats != nil {
			fmt.Printf("\nDiffstat: %d files changed, %d insertions(+), %d deletions(-)\n",
				result.Stats.FilesChanged, result.Stats.Insertions, result.Stats.Deletions)
		}

		return nil
	}),
}

// getCommitDifference returns commits in current that are not in target branch.
func getCommitDifference(repoPath, targetBranch string) ([]CommitDiff, error) {
	// Get commits that are in HEAD but not in target branch
	cmd := exec.Command("git", "-C", repoPath, "log", "--oneline", "--format=%H|%s|%an|%ar",
		fmt.Sprintf("%s..HEAD", targetBranch))
	output, err := cmd.Output()
	if err != nil {
		// This might fail if branches have diverged significantly
		return nil, nil
	}

	var commits []CommitDiff
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) >= 4 {
			commits = append(commits, CommitDiff{
				SHA:     parts[0],
				Message: parts[1],
				Author:  parts[2],
				Age:     parts[3],
			})
		}
	}
	return commits, nil
}

// getWIPDifference returns uncommitted changes in the current worktree.
func getWIPDifference(currentPath, _ string) (*WIPDiff, error) {
	wip := &WIPDiff{}

	// Get staged files
	stagedCmd := exec.Command("git", "-C", currentPath, "diff", "--cached", "--name-only")
	if output, err := stagedCmd.Output(); err == nil {
		for _, f := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if f != "" {
				wip.Staged = append(wip.Staged, f)
			}
		}
	}

	// Get unstaged files (modified tracked files)
	unstagedCmd := exec.Command("git", "-C", currentPath, "diff", "--name-only")
	if output, err := unstagedCmd.Output(); err == nil {
		for _, f := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if f != "" {
				wip.Unstaged = append(wip.Unstaged, f)
			}
		}
	}

	// Get untracked files
	untrackedCmd := exec.Command("git", "-C", currentPath, "ls-files", "--others", "--exclude-standard")
	if output, err := untrackedCmd.Output(); err == nil {
		for _, f := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if f != "" {
				wip.Untracked = append(wip.Untracked, f)
			}
		}
	}

	return wip, nil
}

// getDiffStats returns the diffstat between current HEAD and target branch.
func getDiffStats(repoPath, targetBranch string) (*DiffStats, error) {
	cmd := exec.Command("git", "-C", repoPath, "diff", "--stat", targetBranch)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	stats := &DiffStats{}
	lines := strings.Split(string(output), "\n")

	// Parse the summary line (last non-empty line)
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// Format: "N files changed, M insertions(+), K deletions(-)"
		if strings.Contains(line, "file") && strings.Contains(line, "changed") {
			_, _ = fmt.Sscanf(line, "%d file", &stats.FilesChanged)
			if idx := strings.Index(line, "insertion"); idx > 0 {
				_, _ = fmt.Sscanf(line[strings.LastIndex(line[:idx], " "):idx], "%d", &stats.Insertions)
			}
			if idx := strings.Index(line, "deletion"); idx > 0 {
				start := strings.LastIndex(line[:idx], ",")
				if start < 0 {
					start = strings.LastIndex(line[:idx], " ")
				}
				_, _ = fmt.Sscanf(line[start:idx], "%d", &stats.Deletions)
			}
			break
		}
	}

	return stats, nil
}

func init() {
	compareCmd.Flags().BoolVar(&compareStat, "stat", false, "Show diffstat")
	compareCmd.Flags().BoolVar(&compareCommitted, "committed", false, "Show only committed differences")
	compareCmd.Flags().BoolVar(&compareWIP, "wip", false, "Show only uncommitted differences")
	compareCmd.Flags().BoolVarP(&compareJSON, "json", "j", false, "Output as JSON")

	compareCmd.MarkFlagsMutuallyExclusive("committed", "wip")

	rootCmd.AddCommand(compareCmd)
}
