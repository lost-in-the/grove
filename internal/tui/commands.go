package tui

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

// stdioMu serializes captureStdio's process-global os.Stdout/os.Stderr swap so
// concurrent tea.Cmds (e.g. a bulk delete) can't race on it.
var stdioMu sync.Mutex

// captureStdio runs fn with os.Stdout and os.Stderr redirected to an in-memory
// pipe, returning everything written to them. The bubbletea renderer captured
// its own output handle at tea.NewProgram time, so it keeps drawing to the real
// terminal while legacy code that writes straight to os.Stdout/os.Stderr — the
// docker plugin's compose streaming, agent-slot messages, and env: directives —
// is captured here instead of scribbling over the alt screen. Subprocesses that
// wire cmd.Stderr = os.Stderr inherit the pipe too, so their output is captured
// as well. On pipe-setup failure fn still runs (uncaptured) rather than being
// skipped.
func captureStdio(fn func() error) (string, error) {
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		return "", fn()
	}

	stdioMu.Lock()
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fnErr := fn()

	os.Stdout, os.Stderr = origOut, origErr
	stdioMu.Unlock()
	_ = w.Close()
	captured := <-done
	_ = r.Close()
	return captured, fnErr
}

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
		// The docker plugin's teardown streams `docker compose down` and slot
		// messages to os.Stderr; capture them so they don't corrupt the alt
		// screen (they're logged, not user-facing during a dashboard delete).
		if out, err := captureStdio(func() error { return hooks.Fire(hooks.EventPreRemove, pluginCtx) }); err != nil {
			tuilog.Printf("warning: plugin pre-remove hook failed for %q: %v (output: %s)", name, err, out)
		} else if strings.TrimSpace(out) != "" {
			tuilog.Printf("plugin pre-remove output for %q: %s", name, out)
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

		// hooks.toml post_remove actions (e.g. per-worktree cache cleanup) —
		// the CLI runs these via firePostRemoveHooks, but the dashboard used to
		// skip them, silently leaving that cleanup undone (B28 on the TUI path).
		runPostRemoveHooks(projectRoot, projectName, name, wt)

		if out, err := captureStdio(func() error { return hooks.Fire(hooks.EventPostRemove, pluginCtx) }); err != nil {
			tuilog.Printf("warning: plugin post-remove hook failed for %q: %v (output: %s)", name, err, out)
		} else if strings.TrimSpace(out) != "" {
			tuilog.Printf("plugin post-remove output for %q: %s", name, out)
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
	// Hook output can't go to the terminal (it would corrupt the TUI); keep the
	// executor's own progress in a buffer, and capture any docker subprocess
	// streaming (which writes to os.Stderr) via captureStdio. MainPath is set so
	// working_dir = "main" resolves to the project root and {{.main_path}} is
	// non-empty — without it a `rm -rf {{.main_path}}/...` cleanup would run from
	// the dashboard's cwd against a root-anchored path.
	var out bytes.Buffer
	hookExecutor.Output = &out
	hookCtx := &hooks.ExecutionContext{
		Event:    hooks.EventPreRemove,
		Worktree: name,
		Project:  projectName,
		MainPath: projectRoot,
	}
	if wt != nil {
		hookCtx.Branch = wt.Branch
		hookCtx.NewPath = wt.Path
		hookCtx.WorktreeFull = filepath.Base(wt.Path)
	}
	captured, execErr := captureStdio(func() error {
		return hookExecutor.Execute(hooks.EventPreRemove, hookCtx)
	})
	if execErr != nil {
		tuilog.Printf("pre-remove hook aborted delete of %q: %v (output: %s%s)", name, execErr, out.String(), captured)
		return execErr
	}
	return nil
}

// runPostRemoveHooks executes hooks.toml post-remove actions for a TUI delete,
// mirroring the CLI's firePostRemoveHooks. The worktree is already gone, so
// these are warn-only (never abort) and default to working_dir = "main"; the
// .grove dir is pinned to the project root so hooks resolve identically
// wherever the dashboard was launched from.
func runPostRemoveHooks(projectRoot, projectName, name string, wt *worktree.Worktree) {
	hookExecutor, err := hooks.NewExecutor(filepath.Join(projectRoot, ".grove"))
	if err != nil {
		tuilog.Printf("warning: failed to load hooks config for post-remove: %v", err)
		return
	}
	if !hookExecutor.HasHooksForEvent(hooks.EventPostRemove) {
		return
	}
	var out bytes.Buffer
	hookExecutor.Output = &out
	hookCtx := &hooks.ExecutionContext{
		Event:    hooks.EventPostRemove,
		Worktree: name,
		Project:  projectName,
		MainPath: projectRoot,
	}
	if wt != nil {
		hookCtx.Branch = wt.Branch
		hookCtx.NewPath = wt.Path
		hookCtx.WorktreeFull = filepath.Base(wt.Path)
	}
	captured, execErr := captureStdio(func() error {
		return hookExecutor.Execute(hooks.EventPostRemove, hookCtx)
	})
	if execErr != nil {
		tuilog.Printf("warning: post-remove hook had errors for %q: %v (output: %s%s)", name, execErr, out.String(), captured)
	}
}

// runPreCreateHooks executes hooks.toml pre-create actions before a dashboard
// create, mirroring `grove new`. A required (on_failure="fail") action failing
// returns a non-nil error and the caller must abort before creating the
// worktree (B7); the worktree doesn't exist yet, so actions default to the main
// worktree. Output is captured so it never reaches the alt screen.
func runPreCreateHooks(projectRoot, projectName, name, branch, futurePath string) error {
	hookExecutor, err := hooks.NewExecutor(filepath.Join(projectRoot, ".grove"))
	if err != nil {
		tuilog.Printf("warning: failed to load hooks config for pre-create: %v", err)
		return nil
	}
	if !hookExecutor.HasHooksForEvent(hooks.EventPreCreate) {
		return nil
	}
	var out bytes.Buffer
	hookExecutor.Output = &out
	hookCtx := &hooks.ExecutionContext{
		Event:        hooks.EventPreCreate,
		Worktree:     name,
		Project:      projectName,
		Branch:       branch,
		NewPath:      futurePath,
		WorktreeFull: filepath.Base(futurePath),
		MainPath:     projectRoot,
	}
	captured, execErr := captureStdio(func() error {
		return hookExecutor.Execute(hooks.EventPreCreate, hookCtx)
	})
	if execErr != nil {
		tuilog.Printf("pre-create hook aborted create of %q: %v (output: %s%s)", name, execErr, out.String(), captured)
		return execErr
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
	// Config-hook progress goes to bootBuf via the writer; the plugin
	// post-create hook (docker env: directive, persist warnings) writes to
	// os.Stdout/os.Stderr, so wrap the whole call in captureStdio too — either
	// stream would corrupt the alt screen. Both are surfaced as log lines below.
	var bootBuf bytes.Buffer
	var bootErr error
	pluginOut, _ := captureStdio(func() error {
		bootErr = worktree.BootstrapWorktree(stateMgr, cfg, worktree.BootstrapOpts{
			Name:         name,
			Branch:       wt.Branch,
			WorktreePath: wt.Path,
			MainPath:     projectRoot,
			ProjectName:  projectName,
		}, cli.NewWriter(&bootBuf, false))
		return bootErr
	})
	if bootErr != nil {
		tuilog.Printf("bootstrap failed for %q: %v", name, bootErr)
	}
	for _, line := range strings.Split(strings.TrimRight(bootBuf.String()+pluginOut, "\n"), "\n") {
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
func streamingCreateCmd(mgr *worktree.Manager, stateMgr *state.Manager, cfg *config.Config, projectRoot, name, branch, source string, logLines []string, createFn func() error) tea.Cmd {
	ch := make(chan creationEvent, 10)

	go func() {
		defer close(ch)

		for _, line := range logLines {
			ch <- creationEvent{line: line}
		}

		// Fire pre_create hooks before the worktree exists — a required action
		// failing aborts creation, same as `grove new` (B7). The dashboard used
		// to create unconditionally, bypassing policy gates like a quota check.
		if err := runPreCreateHooks(projectRoot, mgr.GetProjectName(), name, branch, mgr.PathForName(name)); err != nil {
			ch <- creationEvent{done: true, name: name, err: err}
			return
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
	var branch string
	if baseBranch != "" {
		// Split: check out an existing branch into the new worktree.
		branch = baseBranch
		logLines = []string{fmt.Sprintf("Creating worktree '%s' from branch '%s'...", name, baseBranch)}
		// Refresh the local base branch to origin (fast-forward only) so the new
		// worktree starts at the remote's current tip, not a stale local commit.
		createFn = func() error { return mgr.CreateFromBranchRefreshing(name, baseBranch) }
	} else {
		// New branch (optionally forked from fromRef). When no explicit branch
		// name is given (e.g. the fork action), name the branch after the worktree.
		branch = newBranch
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
	return streamingCreateCmd(mgr, stateMgr, cfg, projectRoot, name, branch, "create", logLines, createFn)
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
