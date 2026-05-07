package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/internal/worktreeinfo"
)

var contextJSON bool

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
	Name           string                      `json:"name"`
	Path           string                      `json:"path"`
	Branch         string                      `json:"branch"`
	Commit         contextCommitInfo           `json:"commit"`
	TrackingBranch string                      `json:"tracking_branch,omitempty"`
	HasRemote      bool                        `json:"has_remote"`
	Status         string                      `json:"status"`
	Changes        []string                    `json:"changes,omitempty"`
	Ahead          int                         `json:"ahead"`
	Behind         int                         `json:"behind"`
	StashCount     int                         `json:"stash_count"`
	RecentCommits  []worktreeinfo.RecentCommit `json:"recent_commits,omitempty"`
}

type contextCommitInfo struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
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
		ahead, behind, hasRemote, trackingBranch := worktreeinfo.UpstreamInfo(tree.Path)
		stashCount := worktreeinfo.StashCount(tree.Path)
		recentCommits := worktreeinfo.RecentCommits(tree.Path, 5)

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
