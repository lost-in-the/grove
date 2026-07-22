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
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/tuilog"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/docker"
	"github.com/lost-in-the/grove/plugins/tracker"
)

// dockerAutoUp is a seam over docker.AutoUp so tests can stub the stack start.
var dockerAutoUp = docker.AutoUp

var errWorktreeNotFound = errors.New("worktree created but not found")

// stdioMu serializes captureStdio's process-global os.Stdout/os.Stderr swap so
// concurrent tea.Cmds (e.g. a bulk delete) can't race on it. Any read of
// os.Stdout/os.Stderr on a capture path (e.g. hooks.NewExecutor's default
// Output) must also happen while holding it — in practice, inside a
// captureStdio-wrapped fn.
var stdioMu sync.Mutex

// captureDrainTimeout bounds how long captureStdio waits for the pipe drain
// after fn returns and the write end is closed. Package var so tests can
// shorten it.
var captureDrainTimeout = 3 * time.Second

// syncBuffer is a mutex-guarded bytes.Buffer so captureStdio can read a
// partial capture while the drain goroutine may still be appending.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

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

	buf := &syncBuffer{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(buf, r)
	}()

	// Restore the globals, release the lock, and close our write end via defer
	// so they also run if fn panics — otherwise the globals would stay swapped,
	// stdioMu would stay locked (deadlocking every later capture), and
	// bubbletea's panic recovery would print its diagnostics into the abandoned
	// pipe. The normal path calls restore explicitly before draining.
	restored := false
	restore := func() {
		if restored {
			return
		}
		restored = true
		os.Stdout, os.Stderr = origOut, origErr
		stdioMu.Unlock()
		_ = w.Close()
	}
	defer restore()

	fnErr := fn()
	restore()

	// io.Copy reaches EOF only once every copy of the write end is closed.
	// Ours is, but a subprocess may have inherited the pipe and backgrounded a
	// grandchild (`sh -c 'x &'` from a hook or docker) that keeps its copy open
	// indefinitely — don't let that hang this tea.Cmd forever. On timeout,
	// return what has been captured so far; the drain goroutine (and the read
	// end) linger until the straggler finally closes its copy. That lone
	// blocked goroutine is deliberate and harmless — it does not grow per call
	// unless stragglers accumulate.
	select {
	case <-done:
		_ = r.Close()
	case <-time.After(captureDrainTimeout):
	}
	return buf.String(), fnErr
}

// joinCaptured concatenates two captured output chunks, inserting a newline
// when the first doesn't end with one so its last (unterminated) line can't
// fuse with the first line of the second chunk.
func joinCaptured(a, b string) string {
	if a != "" && b != "" && !strings.HasSuffix(a, "\n") {
		return a + "\n" + b
	}
	return a + b
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

		// Abort when the worktree can't be resolved — `grove rm` exits before
		// any hooks in the same situation. Continuing with wt == nil used to
		// run pre-remove hooks anchored to the process cwd against a worktree
		// that doesn't exist (ghost delete).
		wt, findErr := mgr.Find(name)
		if findErr != nil {
			return worktreeDeletedMsg{name: name, deleteBranch: deleteBranch, err: fmt.Errorf("failed to find worktree %q: %w", name, findErr)}
		}
		if wt == nil {
			return worktreeDeletedMsg{name: name, deleteBranch: deleteBranch, err: fmt.Errorf("worktree %q not found", name)}
		}
		wtPath := wt.Path

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

		// Delete the branch before the post-remove hooks — same order as
		// `grove rm` (handleBranchDeletion, then firePostRemoveHooks) so hooks
		// observe the same repository state on both surfaces.
		branchErr := deleteBranchIfRequested(deleteBranch, wt, projectRoot)

		// hooks.toml post_remove actions (e.g. per-worktree cache cleanup) —
		// the CLI runs these via firePostRemoveHooks, but the dashboard used to
		// skip them, silently leaving that cleanup undone (B28 on the TUI path).
		runPostRemoveHooks(projectRoot, projectName, name, wt)

		if out, err := captureStdio(func() error { return hooks.Fire(hooks.EventPostRemove, pluginCtx) }); err != nil {
			tuilog.Printf("warning: plugin post-remove hook failed for %q: %v (output: %s)", name, err, out)
		} else if strings.TrimSpace(out) != "" {
			tuilog.Printf("plugin post-remove output for %q: %s", name, out)
		}

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
	// Hook output can't go to the terminal (it would corrupt the TUI); keep the
	// executor's own progress in a buffer, and capture any docker subprocess
	// streaming (which writes to os.Stderr) via captureStdio. The executor is
	// constructed INSIDE the capture window because NewExecutor reads os.Stdout
	// for its default Output — done outside, that read races with another
	// goroutine's captureStdio swap. MainPath is set so working_dir = "main"
	// resolves to the project root and {{.main_path}} is non-empty — without it
	// a `rm -rf {{.main_path}}/...` cleanup would run from the dashboard's cwd
	// against a root-anchored path.
	var out bytes.Buffer
	var loadErr error
	captured, execErr := captureStdio(func() error {
		hookExecutor, err := hooks.NewExecutor(filepath.Join(projectRoot, ".grove"))
		if err != nil {
			loadErr = err
			return nil
		}
		if !hookExecutor.HasHooksForEvent(hooks.EventPreRemove) {
			return nil
		}
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
		return hookExecutor.Execute(hooks.EventPreRemove, hookCtx)
	})
	if loadErr != nil {
		// A broken hooks config shouldn't brick dashboard deletes — same
		// soft-fail as the CLI path.
		tuilog.Printf("warning: failed to load hooks config: %v", loadErr)
		return nil
	}
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
	// Executor constructed inside the capture window — see runPreRemoveHooks.
	var out bytes.Buffer
	var loadErr error
	captured, execErr := captureStdio(func() error {
		hookExecutor, err := hooks.NewExecutor(filepath.Join(projectRoot, ".grove"))
		if err != nil {
			loadErr = err
			return nil
		}
		if !hookExecutor.HasHooksForEvent(hooks.EventPostRemove) {
			return nil
		}
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
		return hookExecutor.Execute(hooks.EventPostRemove, hookCtx)
	})
	if loadErr != nil {
		tuilog.Printf("warning: failed to load hooks config for post-remove: %v", loadErr)
		return
	}
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
	// Executor constructed inside the capture window — see runPreRemoveHooks.
	var out bytes.Buffer
	var loadErr error
	captured, execErr := captureStdio(func() error {
		hookExecutor, err := hooks.NewExecutor(filepath.Join(projectRoot, ".grove"))
		if err != nil {
			loadErr = err
			return nil
		}
		if !hookExecutor.HasHooksForEvent(hooks.EventPreCreate) {
			return nil
		}
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
		return hookExecutor.Execute(hooks.EventPreCreate, hookCtx)
	})
	if loadErr != nil {
		tuilog.Printf("warning: failed to load hooks config for pre-create: %v", loadErr)
		return nil
	}
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
	pluginOut, bootErr := captureStdio(func() error {
		return worktree.BootstrapWorktree(stateMgr, cfg, worktree.BootstrapOpts{
			Name:         name,
			Branch:       wt.Branch,
			WorktreePath: wt.Path,
			MainPath:     projectRoot,
			ProjectName:  projectName,
		}, cli.NewWriter(&bootBuf, false))
	})
	for _, line := range strings.Split(strings.TrimRight(joinCaptured(bootBuf.String(), pluginOut), "\n"), "\n") {
		if line != "" {
			ch <- creationEvent{line: line}
		}
	}
	if bootErr != nil {
		// Match the CLI: setupCreatedWorktree returns before autoStartDocker
		// when bootstrap fails (required-hook failure included), and `grove
		// new` never reaches its tmux epilogue on that error — skip both
		// instead of provisioning a half-bootstrapped worktree. The error is
		// still surfaced as a warning toast via hookErr (handleCreationDone).
		tuilog.Printf("bootstrap failed for %q: %v", name, bootErr)
		ch <- creationEvent{line: fmt.Sprintf("Bootstrap failed: %v", bootErr)}
		return postCreateResult{hookOutput: bootBuf.String(), hookErr: bootErr}
	}

	// Auto-start the docker stack when auto_up opts in — the same epilogue
	// `grove new` runs after bootstrap (autoStartDocker), so dashboard-created
	// worktrees come up provisioned identically (#141). Runs before the tmux
	// session exists so the stack is up by the time the user attaches.
	runDockerAutoUp(ch, cfg, name, wt.Path)

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

// runDockerAutoUp brings the docker stack up for a dashboard-created worktree
// when [plugins.docker] auto_up opts in, streaming progress to ch. Compose
// output goes to os.Stderr, so the call is wrapped in captureStdio and
// re-emitted as log lines; failures are surfaced as a log line (warn-only,
// same as the CLI epilogue — the worktree itself was created fine).
func runDockerAutoUp(ch chan<- creationEvent, cfg *config.Config, name, wtPath string) {
	if !docker.ShouldAutoUp(cfg) {
		return
	}
	ch <- creationEvent{line: "Starting Docker stack..."}
	var started bool
	captured, err := captureStdio(func() error {
		var upErr error
		started, upErr = dockerAutoUp(cfg, wtPath)
		return upErr
	})
	for _, line := range strings.Split(strings.TrimRight(captured, "\n"), "\n") {
		if line != "" {
			ch <- creationEvent{line: line}
		}
	}
	if err != nil {
		tuilog.Printf("warning: docker auto-start failed for %q: %v", name, err)
		ch <- creationEvent{line: fmt.Sprintf("Docker auto-start failed: %v", err)}
		return
	}
	if started {
		ch <- creationEvent{line: "Docker stack started"}
	}
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
