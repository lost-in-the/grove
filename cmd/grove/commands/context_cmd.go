package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/worktree"
)

var contextJSON bool

// contextRecentCommit holds a short SHA and commit message for JSON output.
type contextRecentCommit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

// contextOutput is the JSON schema for grove context.
//
// Fields:
//
//	name              - short display name (e.g. "my-feature")
//	path              - absolute path to the worktree
//	branch            - current branch name; "(detached HEAD at <sha>)" if detached
//	commit            - { sha, message } of HEAD
//	tracking_branch   - remote tracking branch (omitted when not set)
//	has_remote        - true when a remote tracking branch is configured
//	status            - "clean" or "dirty"
//	changes           - list of changed files (omitted when clean)
//	ahead             - commits ahead of remote (meaningful only when has_remote: true)
//	behind            - commits behind remote (meaningful only when has_remote: true)
//	stash_count       - number of stashes (0 when none)
//	recent_commits    - last 3–5 commits [{sha, message}, ...]
type contextOutput struct {
	Name           string                `json:"name"`
	Path           string                `json:"path"`
	Branch         string                `json:"branch"`
	Commit         contextCommitInfo     `json:"commit"`
	TrackingBranch string                `json:"tracking_branch,omitempty"`
	HasRemote      bool                  `json:"has_remote"`
	Status         string                `json:"status"`
	Changes        []string              `json:"changes,omitempty"`
	Ahead          int                   `json:"ahead"`
	Behind         int                   `json:"behind"`
	StashCount     int                   `json:"stash_count"`
	RecentCommits  []contextRecentCommit `json:"recent_commits,omitempty"`
}

type contextCommitInfo struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

// ctxUpstreamInfo returns ahead/behind counts, whether a remote is configured,
// and the tracking branch name. Mirrors the signature of getUpstreamInfo in
// internal/tui/data.go: hasRemote is false whenever no upstream is configured
// or the rev-list output is malformed, so callers can distinguish "no remote"
// from "0 ahead, 0 behind with a remote".
func ctxUpstreamInfo(worktreePath string) (ahead, behind int, hasRemote bool, trackingBranch string) {
	ctx := context.TODO()
	out, err := cmdexec.Output(ctx, "git",
		[]string{"rev-list", "--count", "--left-right", "@{upstream}...HEAD"},
		worktreePath, cmdexec.GitLocal)
	if err != nil {
		return 0, 0, false, ""
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) != 2 {
		return 0, 0, false, ""
	}
	b, _ := strconv.Atoi(parts[0])
	a, _ := strconv.Atoi(parts[1])

	tbOut, tbErr := cmdexec.Output(ctx, "git",
		[]string{"rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"},
		worktreePath, cmdexec.GitLocal)
	if tbErr == nil {
		trackingBranch = strings.TrimSpace(string(tbOut))
	}
	return a, b, true, trackingBranch
}

// ctxRecentCommits returns the last n commits as (short-sha, subject) pairs.
func ctxRecentCommits(worktreePath string, n int) []contextRecentCommit {
	out, err := cmdexec.Output(context.TODO(), "git",
		[]string{"log", fmt.Sprintf("-%d", n), "--format=%h %s"},
		worktreePath, cmdexec.GitLocal)
	if err != nil {
		return nil
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	commits := make([]contextRecentCommit, 0, len(lines))
	for _, line := range lines {
		if sha, msg, ok := strings.Cut(line, " "); ok {
			commits = append(commits, contextRecentCommit{SHA: sha, Message: msg})
		}
	}
	return commits
}

// ctxStashCount returns the number of stashes in the worktree.
func ctxStashCount(worktreePath string) int {
	out, err := cmdexec.Output(context.TODO(), "git",
		[]string{"stash", "list"},
		worktreePath, cmdexec.GitLocal)
	if err != nil {
		return 0
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return 0
	}
	return len(strings.Split(raw, "\n"))
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Show full worktree context details",
	Long: `Display comprehensive context for the current worktree.

Includes branch, commit, remote tracking status, working-tree state,
stash count, and recent commits. Use --json for machine-readable output.

JSON schema:
  name              short display name
  path              absolute path to worktree
  branch            current branch (or detached HEAD description)
  commit            { sha, message } of HEAD
  tracking_branch   remote tracking branch (omitted when not set)
  has_remote        true when a remote tracking branch is configured
  status            "clean" or "dirty"
  changes           changed files (omitted when clean)
  ahead             commits ahead of remote (meaningful only when has_remote: true)
  behind            commits behind remote (meaningful only when has_remote: true)
  stash_count       number of stashes
  recent_commits    last 5 commits [{sha, message}]`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
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

		displayName := tree.DisplayName()
		ahead, behind, hasRemote, trackingBranch := ctxUpstreamInfo(tree.Path)
		stashCount := ctxStashCount(tree.Path)
		recentCommits := ctxRecentCommits(tree.Path, 5)

		var changes []string
		// Mirror here.go convention: only check DirtyFiles, not IsDirty.
		// IsDirty=true with DirtyFiles="" would be a bug elsewhere; we let it surface rather than hiding it.
		if tree.DirtyFiles != "" {
			for _, f := range strings.Split(tree.DirtyFiles, "\n") {
				if f != "" {
					changes = append(changes, f)
				}
			}
		}

		wtStatus := statusClean
		if tree.IsDirty {
			wtStatus = statusDirty
		}

		if contextJSON {
			result := contextOutput{
				Name:          displayName,
				Path:          tree.Path,
				Branch:        tree.Branch,
				Commit:        contextCommitInfo{SHA: tree.ShortCommit, Message: tree.CommitMessage},
				HasRemote:     hasRemote,
				Status:        wtStatus,
				Changes:       changes,
				Ahead:         ahead,
				Behind:        behind,
				StashCount:    stashCount,
				RecentCommits: recentCommits,
			}
			if hasRemote {
				result.TrackingBranch = trackingBranch
			}
			return output.PrintJSON(result)
		}

		// Human-readable output
		w := cli.NewStdout()

		cli.Header(w, "%s (%s)", displayName, tree.Branch)
		cli.Label(w, "Path:      ", tree.Path)
		cli.Label(w, "Branch:    ", tree.Branch)

		if trackingBranch != "" {
			syncLabel := trackingBranch
			if ahead > 0 || behind > 0 {
				syncLabel = fmt.Sprintf("%s  ↑%d ↓%d", trackingBranch, ahead, behind)
			}
			cli.Label(w, "Tracking:  ", syncLabel)
		}

		if tree.ShortCommit != "" && tree.CommitMessage != "" {
			cli.Label(w, "Commit:    ", fmt.Sprintf("%s %s", tree.ShortCommit, tree.CommitMessage))
		} else if tree.Commit != "" {
			cli.Label(w, "Commit:    ", tree.Commit)
		}

		if tree.IsDirty {
			cli.Label(w, "Status:    ", cli.StatusText(w, cli.StatusDirty, "● dirty"))
			if len(changes) > 0 {
				shown := changes
				const maxShown = 5
				if len(shown) > maxShown {
					shown = shown[:maxShown]
				}
				for _, f := range shown {
					cli.Faint(w, "           %s", f)
				}
				if len(changes) > maxShown {
					cli.Faint(w, "           ... and %d more", len(changes)-maxShown)
				}
			}
		} else {
			cli.Label(w, "Status:    ", cli.StatusText(w, cli.StatusClean, "✓ clean"))
		}

		if stashCount > 0 {
			cli.Label(w, "Stash:     ", fmt.Sprintf("%d", stashCount))
		}

		if len(recentCommits) > 0 {
			cli.Label(w, "Recent:    ", "")
			for _, rc := range recentCommits {
				cli.Faint(w, "  %s %s", rc.SHA, rc.Message)
			}
		}

		return nil
	}),
}

func init() {
	contextCmd.Flags().BoolVarP(&contextJSON, "json", "j", false, "Output as JSON")
	rootCmd.AddCommand(contextCmd)
}
