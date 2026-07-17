package tui

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
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

// fetchDetailMetricsCmd returns a tea.Cmd that loads the detail-panel-only
// numbers (CommitCount, StashCount) for the given items in the background.
// Use after the dashboard has rendered so the user doesn't wait on N extra
// git calls before first paint.
//
// gen is the model's detail-metrics generation at dispatch time and is
// echoed back in the result so the handler can drop late deliveries from
// a superseded fetch.
func fetchDetailMetricsCmd(gen int, items []WorktreeItem, defaultBranch string) tea.Cmd {
	return func() tea.Msg {
		return detailMetricsLoadedMsg{
			gen:     gen,
			metrics: FetchDetailMetrics(items, defaultBranch),
		}
	}
}

func deleteWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, cfg *config.Config, projectRoot, name string, deleteBranch bool) tea.Cmd {
	return func() tea.Msg {
		projectName := mgr.GetProjectName()

		wt, findErr := mgr.Find(name)
		if findErr != nil {
			tuilog.Printf("warning: failed to find worktree %q for branch capture: %v", name, findErr)
		}
		var wtPath string
		if wt != nil {
			wtPath = wt.Path
		}

		// Fire the config-file (hooks.toml) and plugin pre-remove hooks. The
		// plugin hook (docker agent-slot teardown + env cleanup) was skipped by
		// the TUI, so deleting a worktree with an isolated stack from the
		// dashboard leaked the running stack and its slot (B8).
		//
		// A required (on_failure="fail") hooks.toml action failing aborts the
		// delete before the worktree is touched — same B7 guarantee as `grove
		// rm` (removeWorktreeWithHooks); plugin hook failures stay warn-only
		// on both paths.
		if err := runPreRemoveHooks(projectRoot, projectName, name, wt); err != nil {
			return worktreeDeletedMsg{name: name, deleteBranch: deleteBranch, err: err}
		}
		pluginCtx := &hooks.Context{Worktree: name, Config: cfg, WorktreePath: wtPath, MainPath: projectRoot}
		if err := hooks.Fire(hooks.EventPreRemove, pluginCtx); err != nil {
			tuilog.Printf("warning: plugin pre-remove hook failed for %q: %v", name, err)
		}

		// force=true: the dashboard shows dirty/warning state and the user
		// confirmed the delete. A git-locked worktree is still refused by Remove.
		// Kill the tmux session only after removal succeeds — killing it first
		// (as before) lost the session even when removal failed.
		if err := mgr.Remove(name, true); err != nil {
			return worktreeDeletedMsg{name: name, deleteBranch: deleteBranch, err: err}
		}

		killTmuxSessionForWorktree(projectName, name)

		if err := stateMgr.RemoveWorktree(name); err != nil {
			tuilog.Printf("warning: failed to remove %q from state: %v", name, err)
		}

		if err := hooks.Fire(hooks.EventPostRemove, pluginCtx); err != nil {
			tuilog.Printf("warning: plugin post-remove hook failed for %q: %v", name, err)
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

// runPreRemoveHooks executes hooks.toml pre-remove actions for a TUI delete.
// The .grove dir is passed explicitly (rather than discovered from cwd) so
// hooks resolve identically wherever the dashboard was launched from. The
// returned error is non-nil only for a required (on_failure="fail") action —
// Execute warns internally for everything else — and the caller must abort
// the removal on it (B7 parity with `grove rm`).
func runPreRemoveHooks(projectRoot, projectName, name string, wt *worktree.Worktree) error {
	hookExecutor, err := hooks.NewExecutor(filepath.Join(projectRoot, ".grove"))
	if err != nil {
		// A broken hooks config shouldn't brick dashboard deletes — same
		// soft-fail as the CLI path.
		tuilog.Printf("warning: failed to load hooks config: %v", err)
		return nil
	}
	if !hookExecutor.HasHooksForEvent(hooks.EventPreRemove) {
		return nil
	}
	// Hook output can't go to the terminal (it would corrupt the TUI); keep
	// it in a buffer and surface it in the error when the hook aborts.
	var out bytes.Buffer
	hookExecutor.Output = &out
	hookCtx := &hooks.ExecutionContext{
		Event:    hooks.EventPreRemove,
		Worktree: name,
		Project:  projectName,
	}
	if wt != nil {
		hookCtx.Branch = wt.Branch
		hookCtx.NewPath = wt.Path
		hookCtx.WorktreeFull = filepath.Base(wt.Path)
	}
	if err := hookExecutor.Execute(hooks.EventPreRemove, hookCtx); err != nil {
		tuilog.Printf("pre-remove hook aborted delete of %q: %v (output: %s)", name, err, out.String())
		return err
	}
	return nil
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

// runPostCreateStreaming runs the shared bootstrap sequence for a worktree the
// dashboard just created, then creates its tmux session. It sends progress
// lines to ch and returns the result.
//
// Bootstrap goes through worktree.BootstrapWorktree — the same sequence as
// `grove new`/`grove adopt` — so dashboard-created worktrees get the git
// excludes, SetupFiles (external compose artifacts), plugin post-create hooks
// (docker container Up), and the required-hook abort semantics. The TUI used
// to hand-roll state registration + config hooks only, leaving
// external-compose projects unprovisioned when created from the dashboard.
func runPostCreateStreaming(ch chan<- creationEvent, mgr *worktree.Manager, stateMgr *state.Manager, cfg *config.Config, projectRoot, name string, wt *worktree.Worktree) postCreateResult {
	projectName := mgr.GetProjectName()

	ch <- creationEvent{line: "Bootstrapping worktree..."}
	// All bootstrap output (hook progress, warnings) is captured — writing to
	// the terminal would corrupt the TUI — and streamed as log lines below.
	var bootBuf bytes.Buffer
	bootErr := worktree.BootstrapWorktree(stateMgr, cfg, worktree.BootstrapOpts{
		Name:         name,
		Branch:       wt.Branch,
		WorktreePath: wt.Path,
		MainPath:     projectRoot,
		ProjectName:  projectName,
	}, cli.NewWriter(&bootBuf, false))
	if bootErr != nil {
		tuilog.Printf("bootstrap failed for %q: %v", name, bootErr)
	}
	for _, line := range strings.Split(strings.TrimRight(bootBuf.String(), "\n"), "\n") {
		if line != "" {
			ch <- creationEvent{line: line}
		}
	}

	// Create tmux session after bootstrap so hooks (docker Up, bundle
	// install) have run by the time the user attaches — mirrors the CLI
	// order (setupCreatedWorktree, then switch/tmux).
	if tmux.IsTmuxAvailable() {
		ch <- creationEvent{line: "Creating tmux session..."}
		sessionName := worktree.TmuxSessionName(projectName, name)
		if err := tmux.CreateSession(sessionName, wt.Path); err != nil {
			tuilog.Printf("warning: failed to create tmux session %q: %v", sessionName, err)
		}
	}

	return postCreateResult{hookOutput: bootBuf.String(), hookErr: bootErr}
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
// worktree is looked up and the shared bootstrap sequence runs.
func streamingCreateCmd(mgr *worktree.Manager, stateMgr *state.Manager, cfg *config.Config, projectRoot, name, source string, logLines []string, createFn func() error) tea.Cmd {
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

		result := runPostCreateStreaming(ch, mgr, stateMgr, cfg, projectRoot, name, wt)
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

func createWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, cfg *config.Config, projectRoot, name, baseBranch, newBranch, fromRef string) tea.Cmd {
	var logLines []string
	var createFn func() error
	if baseBranch != "" {
		// Split: check out an existing branch into the new worktree.
		logLines = []string{fmt.Sprintf("Creating worktree '%s' from branch '%s'...", name, baseBranch)}
		createFn = func() error { return mgr.CreateFromBranch(name, baseBranch) }
	} else {
		// New branch (optionally forked from fromRef). When no explicit branch
		// name is given (e.g. the fork action), name the branch after the worktree.
		branch := newBranch
		if branch == "" {
			branch = name
		}
		if fromRef != "" {
			logLines = []string{
				fmt.Sprintf("Creating worktree '%s'...", name),
				fmt.Sprintf("Creating branch '%s' from '%s'...", branch, fromRef),
			}
		} else {
			logLines = []string{
				fmt.Sprintf("Creating worktree '%s'...", name),
				fmt.Sprintf("Creating branch '%s'...", branch),
			}
		}
		createFn = func() error { return mgr.CreateFromRef(name, branch, fromRef) }
	}
	return streamingCreateCmd(mgr, stateMgr, cfg, projectRoot, name, "create", logLines, createFn)
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
