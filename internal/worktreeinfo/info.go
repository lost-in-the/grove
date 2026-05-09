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
//
// For bulk lookup across many worktrees, prefer AllBranchUpstreams: it
// returns the same info for every local branch in a single git call.
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

// BranchUpstream describes a single branch's upstream tracking state.
type BranchUpstream struct {
	Ahead     int
	Behind    int
	Tracking  string // upstream branch name, e.g. "origin/main" (empty if no upstream)
	HasRemote bool   // true when an upstream is configured
}

// AllBranchUpstreams returns upstream tracking info for every local branch in
// repoRoot via a single `git for-each-ref` call. The result is keyed by short
// branch name (e.g. "feat/foo"). Branches with no configured upstream still
// appear in the map with HasRemote=false.
//
// Compared to calling UpstreamInfo per worktree, this collapses 2N git
// invocations into one — significant when N is large.
func AllBranchUpstreams(repoRoot string) (map[string]BranchUpstream, error) {
	const sep = "\x1F" // unit separator — safe inside branch names
	format := strings.Join([]string{
		"%(refname:short)",
		"%(upstream:short)",
		"%(upstream:track,nobracket)",
	}, sep)

	out, err := cmdexec.Output(context.TODO(), "git",
		[]string{"for-each-ref", "--format=" + format, "refs/heads/"},
		repoRoot, cmdexec.GitLocal)
	if err != nil {
		return nil, err
	}

	result := make(map[string]BranchUpstream)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, sep, 3)
		if len(parts) < 3 {
			continue
		}
		branch, upstream, track := parts[0], parts[1], parts[2]
		info := BranchUpstream{
			Tracking:  upstream,
			HasRemote: upstream != "",
		}
		// track is one of: "" (in sync), "gone", "ahead N", "behind N",
		// "ahead N, behind M". Parse the segments we recognize.
		if track != "" && track != "gone" {
			for _, segment := range strings.Split(track, ", ") {
				switch {
				case strings.HasPrefix(segment, "ahead "):
					info.Ahead, _ = strconv.Atoi(strings.TrimPrefix(segment, "ahead "))
				case strings.HasPrefix(segment, "behind "):
					info.Behind, _ = strconv.Atoi(strings.TrimPrefix(segment, "behind "))
				}
			}
		}
		result[branch] = info
	}
	return result, nil
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

// HeadCommitInfo holds enriched metadata for the HEAD commit of a worktree.
type HeadCommitInfo struct {
	ShortHash string
	Message   string // commit subject
	Age       string // relative date, e.g. "2 hours ago"
}

// HeadAndRecentCommits returns both the HEAD commit info and the n most
// recent commits in a single git log call. Combines what would otherwise
// require separate getCommitInfo + RecentCommits invocations.
//
// Returns a zero HeadCommitInfo and nil recent slice on error or empty repo.
// When n < 1, behaves as if n=1 (always returns the head info if available).
func HeadAndRecentCommits(worktreePath string, n int) (HeadCommitInfo, []RecentCommit) {
	if n < 1 {
		n = 1
	}
	// Format: full_hash<RS>short_hash<RS>subject<RS>relative_date
	out, err := cmdexec.Output(context.TODO(), "git",
		[]string{"log", fmt.Sprintf("-%d", n), "--format=%H%x1E%h%x1E%s%x1E%cr"},
		worktreePath, cmdexec.GitLocal)
	if err != nil {
		return HeadCommitInfo{}, nil
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return HeadCommitInfo{}, nil
	}

	var head HeadCommitInfo
	var recents []RecentCommit
	for i, line := range strings.Split(raw, "\n") {
		parts := strings.Split(line, "\x1E")
		if len(parts) < 4 {
			continue
		}
		short, subject, age := parts[1], parts[2], parts[3]
		if i == 0 {
			head = HeadCommitInfo{ShortHash: short, Message: subject, Age: age}
		}
		recents = append(recents, RecentCommit{SHA: short, Message: subject})
	}
	return head, recents
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
