// Package worktreeinfo provides shared git data-fetching helpers used by
// multiple grove commands and the TUI to read per-worktree state.
package worktreeinfo

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/lost-in-the/grove/internal/cmdexec"
)

// RecentCommit holds a short SHA and subject line for a single commit.
type RecentCommit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

// UpstreamInfo returns upstream tracking information for the worktree at
// worktreePath. hasRemote is false whenever no upstream is configured or
// the rev-list output is malformed; in that case ahead, behind are 0 and
// trackingBranch is empty.
func UpstreamInfo(worktreePath string) (ahead, behind int, hasRemote bool, trackingBranch string) {
	out, err := cmdexec.Output(context.TODO(), "git",
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

	tbOut, tbErr := cmdexec.Output(context.TODO(), "git",
		[]string{"rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"},
		worktreePath, cmdexec.GitLocal)
	if tbErr == nil {
		trackingBranch = strings.TrimSpace(string(tbOut))
	}

	return a, b, true, trackingBranch
}

// CommitCountAhead returns the number of commits the worktree branch is ahead
// of defaultBranch. Returns 0 if on the default branch, if defaultBranch does
// not exist, or on any git error.
func CommitCountAhead(worktreePath, defaultBranch string) int {
	out, err := cmdexec.Output(context.TODO(), "git",
		[]string{"rev-list", "--count", defaultBranch + "..HEAD"},
		worktreePath, cmdexec.GitLocal)
	if err != nil {
		return 0
	}
	count, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return count
}

// RecentCommits returns the last n commits (short SHA + subject) for the
// worktree at worktreePath. Returns nil on error, on an empty repository,
// or when n produces no output.
func RecentCommits(worktreePath string, n int) []RecentCommit {
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
	commits := make([]RecentCommit, 0, len(lines))
	for _, line := range lines {
		if sha, msg, ok := strings.Cut(line, " "); ok {
			commits = append(commits, RecentCommit{SHA: sha, Message: msg})
		}
	}
	return commits
}

// StashCount returns the number of stashes in the worktree at worktreePath.
// Returns 0 on error or when there are no stashes.
func StashCount(worktreePath string) int {
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
