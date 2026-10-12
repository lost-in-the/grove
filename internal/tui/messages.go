package tui

import (
	"github.com/lost-in-the/grove/plugins/tracker"
)

// worktreesFetchedMsg is sent when worktree data has been loaded.
type worktreesFetchedMsg struct {
	items []WorktreeItem
	err   error
}

// detailMetricsLoadedMsg is sent after the deferred detail-panel metrics
// (CommitCount, StashCount) have been computed for the visible worktrees.
// Keyed by worktree path.
//
// gen identifies the FetchWorktrees iteration this result belongs to; the
// handler drops messages whose gen no longer matches the model's current
// generation, so a slow fetch can't apply old numbers to a fresh item set.
type detailMetricsLoadedMsg struct {
	gen     int
	metrics map[string]DetailMetrics
}

// worktreeDeletedMsg is sent after a worktree deletion attempt.
type worktreeDeletedMsg struct {
	name         string
	deleteBranch bool
	err          error
	branchErr    error // non-nil if worktree removed but branch deletion failed
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

// creationTracker is implemented by the state that shows streaming worktree
// creation progress (the branch form). It lets creationDoneMsg and
// creationLogMsg be handled in one place.
type creationTracker interface {
	getActivityLog() *ActivityLog
	setCreatingDone(errMsg string) // ends the busy state and records errMsg
}

// creationLogMsg carries a single log line from a streaming creation goroutine.
// The channel is carried so Update can chain the next read.
type creationLogMsg struct {
	source string // routes to the tracker; only "create" is used
	line   string
	ch     <-chan creationEvent
}

// creationDoneMsg signals that streaming creation has finished.
type creationDoneMsg struct {
	source     string // routes to the tracker; only "create" is used
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
