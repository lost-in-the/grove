package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/docker"
)

const (
	// recentCommitsLimit is the number of recent commits to show
	recentCommitsLimit = 5
)

var contextJSON bool

// contextOutput is the JSON output structure for grove context
type contextOutput struct {
	Name          string         `json:"name"`
	FullName      string         `json:"fullName"`
	Project       string         `json:"project"`
	Branch        string         `json:"branch"`
	Path          string         `json:"path"`
	Commit        commitInfo     `json:"commit"`
	Remote        remoteInfo     `json:"remote"`
	Status        string         `json:"status"`
	Changes       []string       `json:"changes,omitempty"`
	StashCount    int            `json:"stashCount"`
	RecentCommits []recentCommit `json:"recentCommits"`
	Tmux          tmuxInfo       `json:"tmux"`
}

type remoteInfo struct {
	Tracking string `json:"tracking"`
	Ahead    int    `json:"ahead"`
	Behind   int    `json:"behind"`
}

type recentCommit struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Age     string `json:"age"`
}

var contextCmd = &cobra.Command{
	Use:     "context",
	Aliases: []string{"ctx"},
	Short:   "Show comprehensive worktree details",
	Long:    `Display detailed information about the current worktree including sync status, stash count, and recent commits.`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, groveCtx *GroveContext) error {
		mgr, err := worktree.NewManager(groveCtx.ProjectRoot)
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
		projectName := mgr.GetProjectName()
		tmuxSessionName := worktree.TmuxSessionName(projectName, tree.ShortName)
		tmuxStatus := tmux.GetSessionStatus(tmuxSessionName)

		// Fallback: check with directory basename
		if tmuxStatus == tmuxStatusNone {
			tmuxSessionName = filepath.Base(tree.Path)
			tmuxStatus = tmux.GetSessionStatus(tmuxSessionName)
		}

		// Gather extended git details
		remote := getRemoteInfo(tree.Path)
		stashCount := getStashCount(tree.Path)
		recentCommits := getRecentCommits(tree.Path, recentCommitsLimit)

		// Agent stack info
		agentSlot := docker.FindWorktreeSlot(groveCtx.Config, tree.Path)
		agentURL := ""
		if agentSlot > 0 {
			agentURL = docker.AgentURL(groveCtx.Config, agentSlot)
		}

		// Build changes list
		var changes []string
		if tree.DirtyFiles != "" {
			for _, line := range strings.Split(tree.DirtyFiles, "\n") {
				if line != "" {
					changes = append(changes, line)
				}
			}
		}

		status := statusClean
		if tree.IsDirty {
			status = statusDirty
		}

		if contextJSON {
			result := contextOutput{
				Name:     displayName,
				FullName: tree.Name,
				Project:  projectName,
				Branch:   tree.Branch,
				Path:     tree.Path,
				Commit: commitInfo{
					Hash:      tree.Commit,
					ShortHash: tree.ShortCommit,
					Message:   tree.CommitMessage,
					Age:       tree.CommitAge,
				},
				Remote:        remote,
				Status:        status,
				Changes:       changes,
				StashCount:    stashCount,
				RecentCommits: recentCommits,
				Tmux: tmuxInfo{
					Session: tmuxSessionName,
					Status:  tmuxStatus,
				},
			}
			return output.PrintJSON(result)
		}

		// Human-readable output
		w := cli.NewStdout()

		cli.Header(w, "%s (%s)", displayName, tree.Branch)
		cli.Label(w, "Path:   ", tree.Path)
		cli.Label(w, "Branch: ", tree.Branch)

		// Commit info
		if tree.ShortCommit != "" && tree.CommitMessage != "" {
			cli.Label(w, "Commit: ", fmt.Sprintf("%s - %s (%s)", tree.ShortCommit, tree.CommitMessage, tree.CommitAge))
		} else {
			cli.Label(w, "Commit: ", tree.Commit)
		}

		// Remote sync status
		if remote.Tracking != "" {
			syncParts := []string{remote.Tracking}
			if remote.Ahead > 0 || remote.Behind > 0 {
				syncParts = append(syncParts, fmt.Sprintf("↑%d ↓%d", remote.Ahead, remote.Behind))
			} else {
				syncParts = append(syncParts, "up to date")
			}
			cli.Label(w, "Remote: ", strings.Join(syncParts, " · "))
		} else {
			cli.Label(w, "Remote: ", cli.StatusText(w, cli.StatusNone, "no tracking branch"))
		}

		// Working tree status
		if tree.IsDirty {
			cli.Label(w, "Status: ", cli.StatusText(w, cli.StatusDirty, "● Dirty"))
		} else {
			cli.Label(w, "Status: ", cli.StatusText(w, cli.StatusClean, "✓ Clean"))
		}

		// Dirty files
		if tree.IsDirty && len(changes) > 0 {
			shown := changes
			extra := 0
			if len(shown) > maxDirtyFilesShown {
				extra = len(shown) - maxDirtyFilesShown
				shown = shown[:maxDirtyFilesShown]
			}
			for _, line := range shown {
				cli.Faint(w, "         %s", line)
			}
			if extra > 0 {
				cli.Faint(w, "         ... and %d more", extra)
			}
		}

		// Stash count
		if stashCount > 0 {
			cli.Label(w, "Stash:  ", fmt.Sprintf("%d stashed", stashCount))
		}

		// Agent stack info
		if agentSlot > 0 {
			cli.Label(w, "Stack:  ", fmt.Sprintf("isolated (slot %d)", agentSlot))
			if agentURL != "" {
				cli.Label(w, "URL:    ", agentURL)
			}
		}

		// tmux status
		tmuxValue := tmuxSessionName
		if tmuxStatus != tmuxStatusNone {
			tmuxValue = fmt.Sprintf("%s (%s)", tmuxSessionName, tmuxStatus)
		}
		cli.Label(w, "tmux:   ", tmuxValue)

		// Recent commits section
		if len(recentCommits) > 0 {
			_, _ = fmt.Fprintln(w)
			cli.Bold(w, "Recent commits:")
			for _, rc := range recentCommits {
				cli.Faint(w, "  %s  %s  (%s)", rc.Hash, rc.Message, rc.Age)
			}
		}

		return nil
	}),
}

// getRemoteInfo returns ahead/behind counts and the tracking branch name.
func getRemoteInfo(worktreePath string) remoteInfo {
	// Get tracking branch name
	trackingOut, err := cmdexec.Output(context.TODO(), "git",
		[]string{"rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"},
		worktreePath, cmdexec.GitLocal)
	if err != nil {
		// No upstream configured — not an error
		return remoteInfo{}
	}
	tracking := strings.TrimSpace(string(trackingOut))
	if tracking == "" || tracking == "@{upstream}" {
		return remoteInfo{}
	}

	// Get ahead/behind counts via left-right rev-list
	countOut, err := cmdexec.Output(context.TODO(), "git",
		[]string{"rev-list", "--left-right", "--count", "HEAD...@{upstream}"},
		worktreePath, cmdexec.GitLocal)
	if err != nil {
		return remoteInfo{Tracking: tracking}
	}

	fields := strings.Fields(strings.TrimSpace(string(countOut)))
	ahead, behind := 0, 0
	if len(fields) == 2 {
		ahead, _ = strconv.Atoi(fields[0])
		behind, _ = strconv.Atoi(fields[1])
	}

	return remoteInfo{
		Tracking: tracking,
		Ahead:    ahead,
		Behind:   behind,
	}
}

// getStashCount returns the number of stash entries in the worktree.
func getStashCount(worktreePath string) int {
	out, err := cmdexec.Output(context.TODO(), "git",
		[]string{"stash", "list"},
		worktreePath, cmdexec.GitLocal)
	if err != nil || len(out) == 0 {
		return 0
	}
	count := 0
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l != "" {
			count++
		}
	}
	return count
}

// getRecentCommits returns the last n commits with short hash, subject, and relative age.
func getRecentCommits(worktreePath string, n int) []recentCommit {
	out, err := cmdexec.Output(context.TODO(), "git",
		[]string{"log", fmt.Sprintf("-%d", n), "--format=%h%x1E%s%x1E%cr"},
		worktreePath, cmdexec.GitLocal)
	if err != nil || len(out) == 0 {
		return nil
	}

	var commits []recentCommit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1E", 3)
		if len(parts) != 3 {
			continue
		}
		commits = append(commits, recentCommit{
			Hash:    parts[0],
			Message: parts[1],
			Age:     parts[2],
		})
	}
	return commits
}

func init() {
	contextCmd.Flags().BoolVarP(&contextJSON, "json", "j", false, "Output as JSON")
	rootCmd.AddCommand(contextCmd)
}
