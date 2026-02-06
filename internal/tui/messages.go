package tui

import (
	"time"

	"github.com/LeahArmstrong/grove-cli/plugins/tracker"
)

// worktreesFetchedMsg is sent when worktree data has been loaded.
type worktreesFetchedMsg struct {
	items []WorktreeItem
	err   error
}

// worktreeDeletedMsg is sent after a worktree deletion attempt.
type worktreeDeletedMsg struct {
	name         string
	deleteBranch bool
	err          error
}

// worktreeCreatedMsg is sent after a worktree creation attempt.
type worktreeCreatedMsg struct {
	name       string
	path       string
	err        error
	hookOutput string
}

// statusClearMsg is sent to clear the status bar toast message.
type statusClearMsg struct {
	deadline time.Time
}

// bulkDeleteDoneMsg is sent when bulk deletion completes.
type bulkDeleteDoneMsg struct {
	count  int
	failed []string // names of worktrees that failed to delete
}

// prsFetchedMsg is sent when PR data has been loaded.
type prsFetchedMsg struct {
	prs []*tracker.PullRequest
	err error
}

// prWorktreeCreatedMsg is sent after creating a worktree from a PR.
type prWorktreeCreatedMsg struct {
	name string
	path string
	err  error
}

// issuesFetchedMsg is sent when issue data has been loaded.
type issuesFetchedMsg struct {
	issues []*tracker.Issue
	err    error
}

// issueWorktreeCreatedMsg is sent after creating a worktree from an issue.
type issueWorktreeCreatedMsg struct {
	name string
	path string
	err  error
}
