package tui

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/tuilog"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/tracker"
)

var errWorktreeNotFound = errors.New("worktree created but not found")

func (m Model) fetchWorktrees() tea.Msg {
	items, err := FetchWorktrees(m.worktreeMgr, m.stateMgr, m.pluginMgr)
	return worktreesFetchedMsg{items: items, err: err}
}

func deleteWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot, name string, deleteBranch bool) tea.Cmd {
	return func() tea.Msg {
		projectName := mgr.GetProjectName()

		killTmuxSessionForWorktree(projectName, name)

		wt, findErr := mgr.Find(name)
		if findErr != nil {
			tuilog.Printf("warning: failed to find worktree %q for branch capture: %v", name, findErr)
		}

		runPreRemoveHooks(projectName, name, wt)

		if err := mgr.Remove(name); err != nil {
			return worktreeDeletedMsg{name: name, deleteBranch: deleteBranch, err: err}
		}

		if err := stateMgr.RemoveWorktree(name); err != nil {
			tuilog.Printf("warning: failed to remove %q from state: %v", name, err)
		}

		branchErr := deleteBranchIfRequested(deleteBranch, wt, projectRoot)
		return worktreeDeletedMsg{name: name, deleteBranch: deleteBranch, err: nil, branchErr: branchErr}
	}
}

func killTmuxSessionForWorktree(projectName, name string) {
	if !tmux.IsTmuxAvailable() {
		return
	}
	sessionName := worktree.TmuxSessionName(projectName, name)
	exists, err := tmux.SessionExists(sessionName)
	if err != nil {
		tuilog.Printf("warning: failed to check tmux session %q: %v", sessionName, err)
		return
	}
	if !exists {
		return
	}
	if err := tmux.KillSession(sessionName); err != nil {
		tuilog.Printf("warning: failed to kill tmux session %q: %v", sessionName, err)
	}
}

func runPreRemoveHooks(projectName, name string, wt *worktree.Worktree) {
	hookExecutor, err := hooks.NewExecutor()
	if err != nil || !hookExecutor.HasHooksForEvent(hooks.EventPreRemove) {
		return
	}
	hookExecutor.Output = &bytes.Buffer{}
	hookCtx := &hooks.ExecutionContext{
		Event:    hooks.EventPreRemove,
		Worktree: name,
		Project:  projectName,
	}
	if wt != nil {
		hookCtx.Branch = wt.Branch
		hookCtx.NewPath = wt.Path
		hookCtx.WorktreeFull = projectName + "-" + name
	}
	if err := hookExecutor.Execute(hooks.EventPreRemove, hookCtx); err != nil {
		tuilog.Printf("warning: pre-remove hook failed for %q: %v", name, err)
	}
}

func deleteBranchIfRequested(deleteBranch bool, wt *worktree.Worktree, projectRoot string) error {
	if !deleteBranch || wt == nil || wt.Branch == "" {
		return nil
	}
	branchMgr, err := git.NewBranchManager(projectRoot)
	if err != nil {
		return fmt.Errorf("branch manager init failed: %w", err)
	}
	if err := branchMgr.Delete(wt.Branch, false); err != nil {
		return fmt.Errorf("failed to delete branch %q: %w", wt.Branch, err)
	}
	return nil
}

// postCreateResult holds the output of post-create setup (state, tmux, hooks).
type postCreateResult struct {
	hookOutput string
	hookErr    error
}

// runPostCreateStreaming registers the worktree in state, creates a tmux session,
// It sends progress lines to ch and returns the result.
func runPostCreateStreaming(ch chan<- creationEvent, mgr *worktree.Manager, stateMgr *state.Manager, projectRoot, name string, wt *worktree.Worktree) postCreateResult {
	projectName := mgr.GetProjectName()

	// Register in state
	ch <- creationEvent{line: "Registering state..."}
	if stateMgr != nil {
		now := time.Now()
		wsState := &state.WorktreeState{
			Path:           wt.Path,
			Branch:         wt.Branch,
			CreatedAt:      now,
			LastAccessedAt: now,
		}
		if err := stateMgr.AddWorktree(name, wsState); err != nil {
			tuilog.Printf("warning: failed to register worktree %q in state: %v", name, err)
		}
	}

	// Create tmux session
	if tmux.IsTmuxAvailable() {
		ch <- creationEvent{line: "Creating tmux session..."}
		sessionName := worktree.TmuxSessionName(projectName, name)
		if err := tmux.CreateSession(sessionName, wt.Path); err != nil {
			tuilog.Printf("warning: failed to create tmux session %q: %v", sessionName, err)
		}
	}

	// Run post-create hooks, capturing output to avoid corrupting TUI
	var hookBuf bytes.Buffer
	var hookExecErr error
	hookExecutor, hookErr := hooks.NewExecutor()
	if hookErr == nil && hookExecutor.HasHooksForEvent(hooks.EventPostCreate) {
		ch <- creationEvent{line: "Running post-create hooks..."}
		hookExecutor.Output = &hookBuf
		hookCtx := &hooks.ExecutionContext{
			Event:        hooks.EventPostCreate,
			Worktree:     name,
			WorktreeFull: projectName + "-" + name,
			Branch:       wt.Branch,
			Project:      projectName,
			MainPath:     projectRoot,
			NewPath:      wt.Path,
		}
		hookExecErr = hookExecutor.Execute(hooks.EventPostCreate, hookCtx)
		if hookExecErr != nil {
			tuilog.Printf("warning: post-create hook failed for %q: %v", name, hookExecErr)
		}
		// Stream hook output lines
		if hookBuf.Len() > 0 {
			for _, line := range strings.Split(strings.TrimRight(hookBuf.String(), "\n"), "\n") {
				if line != "" {
					ch <- creationEvent{line: line}
				}
			}
		}
	}

	return postCreateResult{hookOutput: hookBuf.String(), hookErr: hookExecErr}
}

// readCreationLog returns a tea.Cmd that reads the next event from the
// creation channel. When the channel closes, it returns a creationDoneMsg.
func readCreationLog(ch <-chan creationEvent, source string) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			// Channel closed unexpectedly — treat as done with no result.
			return creationDoneMsg{source: source}
		}
		if ev.done {
			return creationDoneMsg{
				source:     source,
				name:       ev.name,
				path:       ev.path,
				err:        ev.err,
				hookOutput: ev.hookOutput,
				hookErr:    ev.hookErr,
			}
		}
		return creationLogMsg{source: source, line: ev.line, ch: ch}
	}
}

// streamingCreateCmd runs a worktree creation in a goroutine with streaming
// log output. The createFn performs the actual worktree creation; logLines
// are sent to the channel before creation begins. After creation, the
// worktree is looked up and post-create hooks are run.
func streamingCreateCmd(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot, name, source string, logLines []string, createFn func() error) tea.Cmd {
	ch := make(chan creationEvent, 10)

	go func() {
		defer close(ch)

		for _, line := range logLines {
			ch <- creationEvent{line: line}
		}

		if err := createFn(); err != nil {
			ch <- creationEvent{done: true, name: name, err: err}
			return
		}

		wt, err := mgr.Find(name)
		if err != nil || wt == nil {
			ch <- creationEvent{done: true, name: name, err: errWorktreeNotFound}
			return
		}

		result := runPostCreateStreaming(ch, mgr, stateMgr, projectRoot, name, wt)
		ch <- creationEvent{
			done:       true,
			name:       name,
			path:       wt.Path,
			hookOutput: result.hookOutput,
			hookErr:    result.hookErr,
		}
	}()

	return readCreationLog(ch, source)
}

func createWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot, name, baseBranch string) tea.Cmd {
	var logLines []string
	var createFn func() error
	if baseBranch != "" {
		logLines = []string{fmt.Sprintf("Creating worktree '%s' from branch '%s'...", name, baseBranch)}
		createFn = func() error { return mgr.CreateFromExisting(name, baseBranch) }
	} else {
		logLines = []string{
			fmt.Sprintf("Creating worktree '%s'...", name),
			fmt.Sprintf("Creating branch '%s'...", name),
		}
		createFn = func() error { return mgr.Create(name, name) }
	}
	return streamingCreateCmd(mgr, stateMgr, projectRoot, name, "create", logLines, createFn)
}

// lookupPRsCmd fetches open PRs and maps them to branches.
// This is a lazy/expensive network call — run after initial worktree load.
func lookupPRsCmd(branches []string) tea.Cmd {
	return func() tea.Msg {
		if !tracker.IsGHInstalled() {
			tuilog.Printf("pr lookup: gh not installed, skipping")
			return prLookupMsg{prs: nil}
		}

		adapter := tracker.NewGitHubAdapter("")
		prs, err := adapter.ListPRs(tracker.ListOptions{State: "open", Limit: 100})
		if err != nil {
			tuilog.Printf("pr lookup: failed to list PRs: %v", err)
			return prLookupMsg{prs: nil}
		}

		// Build a set of branches we care about for fast lookup
		branchSet := make(map[string]struct{}, len(branches))
		for _, b := range branches {
			branchSet[b] = struct{}{}
		}

		result := make(map[string]*PRInfo)
		for _, pr := range prs {
			if _, ok := branchSet[pr.Branch]; ok {
				result[pr.Branch] = &PRInfo{
					Number:         pr.Number,
					Title:          pr.Title,
					URL:            pr.URL,
					ReviewDecision: pr.ReviewDecision,
				}
			}
		}

		tuilog.Printf("pr lookup: matched %d PRs to branches", len(result))
		return prLookupMsg{prs: result}
	}
}
