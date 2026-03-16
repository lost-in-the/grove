package tui

import (
	"github.com/lost-in-the/grove/plugins/tracker"
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
	branchErr    error // non-nil if worktree removed but branch deletion failed
}

// worktreeCreatedMsg is sent after a worktree creation attempt.
type worktreeCreatedMsg struct {
	name       string
	err        error
	hookOutput string
	hookErr    error // non-nil if hook execution failed
}

// bulkDeleteDoneMsg is sent when bulk deletion completes.
type bulkDeleteDoneMsg struct {
	count  int
	failed map[string]string // name → error message for worktrees that failed
}

// prsFetchedMsg is sent when PR data has been loaded.
type prsFetchedMsg struct {
	prs []*tracker.PullRequest
	err error
}

// issuesFetchedMsg is sent when issue data has been loaded.
type issuesFetchedMsg struct {
	issues []*tracker.Issue
	err    error
}

// creationTracker is implemented by state structs that track streaming worktree
// creation (CreateState, PRViewState, IssueViewState). It allows the common
// creationDoneMsg / creationLogMsg handling to be written once.
type creationTracker interface {
	getActivityLog() *ActivityLog
	setCreatingDone(errMsg string) // sets Creating=false and Error=errMsg
}

// creationLogMsg carries a single log line from a streaming creation goroutine.
// The channel is carried so Update can chain the next read.
type creationLogMsg struct {
	source string // "create", "pr", or "issue" — routes to the right log
	line   string
	ch     <-chan creationEvent
}

// creationDoneMsg signals that streaming creation has finished.
// It carries the same fields as worktreeCreatedMsg so the existing
// completion logic can be reused.
type creationDoneMsg struct {
	source     string // "create", "pr", or "issue" — routes to the right handler
	name       string
	path       string
	err        error
	hookOutput string
	hookErr    error
}

// creationEvent is sent over the channel from the creation goroutine.
// If err is non-nil, it is the final event (creation failed).
type creationEvent struct {
	line string
	done bool
	// These are populated only on the final (done) event.
	name       string
	path       string
	err        error
	hookOutput string
	hookErr    error
}

// prLookupMsg is sent when lazy PR lookup for worktree branches completes.
type prLookupMsg struct {
	// branch name -> PRInfo mapping
	prs map[string]*PRInfo
}
