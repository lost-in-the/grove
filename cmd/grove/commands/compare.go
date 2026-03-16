package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/worktree"
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
	Use:     "diff <name>",
	Aliases: []string{"compare", "d"},
	Short:   "Diff current worktree against another",
	Long: `Diff the current worktree against another worktree.

Shows differences in commits and optionally uncommitted changes (WIP).
By default shows both committed and uncommitted differences.

Examples:
  grove diff main           # Diff against main worktree
  grove diff feature --stat # Show only diffstat
  grove diff main --committed  # Show only commit differences
  grove diff main --wip     # Show only uncommitted differences
  grove diff main --json    # Output as JSON`,
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
			commits := getCommitDifference(currentTree.Path, targetTree.Branch)
			result.Commits = commits
			if len(commits) > 0 {
				result.HasDiff = true
			}
		}

		// Compare WIP (uncommitted changes)
		if showWIP {
			wipDiff := getWIPDifference(currentTree.Path)
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
			return output.PrintJSON(result)
		}

		// Human-readable output
		w := cli.NewStdout()
		cli.Header(w, "Comparing %s → %s", result.Current, result.Target)

		if !result.HasDiff {
			cli.Info(w, "No differences found")
			return nil
		}

		// Show commits
		if len(result.Commits) > 0 {
			_, _ = fmt.Fprintln(w)
			cli.Bold(w, "Commits ahead of %s (%d):", result.Target, len(result.Commits))
			for _, commit := range result.Commits {
				sha := cli.StatusText(w, cli.StatusInfo, commit.SHA[:7])
				age := cli.StatusText(w, cli.StatusInfo, "("+commit.Age+")")
				_, _ = fmt.Fprintf(w, "  %s %s %s\n", sha, commit.Message, age)
			}
		}

		// Show WIP
		if result.WIP != nil {
			_, _ = fmt.Fprintln(w)
			cli.Bold(w, "Uncommitted changes:")
			if len(result.WIP.Staged) > 0 {
				_, _ = fmt.Fprintf(w, "  Staged (%d):\n", len(result.WIP.Staged))
				for _, f := range result.WIP.Staged {
					_, _ = fmt.Fprintf(w, "    %s\n", cli.StatusText(w, cli.StatusOK, "+ "+f))
				}
			}
			if len(result.WIP.Unstaged) > 0 {
				_, _ = fmt.Fprintf(w, "  Unstaged (%d):\n", len(result.WIP.Unstaged))
				for _, f := range result.WIP.Unstaged {
					_, _ = fmt.Fprintf(w, "    %s\n", cli.StatusText(w, cli.StatusDirty, "M "+f))
				}
			}
			if len(result.WIP.Untracked) > 0 {
				_, _ = fmt.Fprintf(w, "  Untracked (%d):\n", len(result.WIP.Untracked))
				for _, f := range result.WIP.Untracked {
					_, _ = fmt.Fprintf(w, "    %s\n", cli.StatusText(w, cli.StatusInfo, "? "+f))
				}
			}
		}

		// Show stats
		if result.Stats != nil {
			_, _ = fmt.Fprintln(w)
			cli.Faint(w, "Diffstat: %d files changed, %d insertions(+), %d deletions(-)",
				result.Stats.FilesChanged, result.Stats.Insertions, result.Stats.Deletions)
		}

		return nil
	}),
}

// getCommitDifference returns commits in current that are not in target branch.
func getCommitDifference(repoPath, targetBranch string) []CommitDiff {
	// Get commits that are in HEAD but not in target branch
	output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", repoPath, "log", "--oneline", "--format=%H|%s|%an|%ar",
		fmt.Sprintf("%s..HEAD", targetBranch)}, "", cmdexec.GitLocal)
	if err != nil {
		// This might fail if branches have diverged significantly
		return nil
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
	return commits
}

// getWIPDifference returns uncommitted changes in the current worktree.
func getWIPDifference(currentPath string) *WIPDiff {
	wip := &WIPDiff{}

	// Get staged files
	if output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", currentPath, "diff", "--cached", "--name-only"}, "", cmdexec.GitLocal); err == nil {
		for _, f := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if f != "" {
				wip.Staged = append(wip.Staged, f)
			}
		}
	}

	// Get unstaged files (modified tracked files)
	if output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", currentPath, "diff", "--name-only"}, "", cmdexec.GitLocal); err == nil {
		for _, f := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if f != "" {
				wip.Unstaged = append(wip.Unstaged, f)
			}
		}
	}

	// Get untracked files
	if output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", currentPath, "ls-files", "--others", "--exclude-standard"}, "", cmdexec.GitLocal); err == nil {
		for _, f := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if f != "" {
				wip.Untracked = append(wip.Untracked, f)
			}
		}
	}

	return wip
}

// getDiffStats returns the diffstat between current HEAD and target branch.
func getDiffStats(repoPath, targetBranch string) (*DiffStats, error) {
	output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", repoPath, "diff", "--stat", targetBranch}, "", cmdexec.GitLocal)
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
			parseDiffStatLine(line, stats)
			break
		}
	}

	return stats, nil
}

// parseDiffStatLine extracts file, insertion, and deletion counts from a git diff --stat summary line.
func parseDiffStatLine(line string, stats *DiffStats) {
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
}

func init() {
	compareCmd.Flags().BoolVar(&compareStat, "stat", false, "Show diffstat")
	compareCmd.Flags().BoolVar(&compareCommitted, "committed", false, "Show only committed differences")
	compareCmd.Flags().BoolVar(&compareWIP, "wip", false, "Show only uncommitted differences")
	compareCmd.Flags().BoolVarP(&compareJSON, "json", "j", false, "Output as JSON")

	compareCmd.MarkFlagsMutuallyExclusive("committed", "wip")

	rootCmd.AddCommand(compareCmd)
}
