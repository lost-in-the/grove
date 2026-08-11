package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/log"
	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/state"
)

// This file implements the grove side of the herdr plugin shipped in
// integrations/herdr. herdr invokes plugin commands as plain argv processes and
// passes context through the environment, so these subcommands are hidden —
// they are an integration surface, not something a user types.
//
// See integrations/herdr/README.md for the manifest and install instructions.

// herdrContext is the invocation context herdr passes to a plugin action,
// decoded from HERDR_PLUGIN_CONTEXT_JSON.
type herdrContext struct {
	WorkspaceID    string `json:"workspace_id"`
	WorkspaceLabel string `json:"workspace_label"`
	WorkspaceCwd   string `json:"workspace_cwd"`
	FocusedPaneID  string `json:"focused_pane_id"`
	FocusedPaneCwd string `json:"focused_pane_cwd"`
	Worktree       *struct {
		RepoName     string `json:"repo_name"`
		RepoRoot     string `json:"repo_root"`
		CheckoutPath string `json:"checkout_path"`
	} `json:"worktree"`
}

// CheckoutPath returns the directory the invocation refers to: the workspace's
// git checkout, or its plain cwd when the workspace has no git provenance.
//
// It does not check the filesystem — use ResolveDir for that.
func (c *herdrContext) CheckoutPath() string {
	if c.Worktree != nil && c.Worktree.CheckoutPath != "" {
		return c.Worktree.CheckoutPath
	}
	return c.WorkspaceCwd
}

// ResolveDir returns the first directory in the context that still exists, or
// "" when every path herdr reported is gone.
//
// herdr captures a workspace's paths when it opens and does not update them
// afterwards, so `grove rename` leaves all three stale at once: the checkout
// path, the workspace cwd, and the pane's cwd all name the pre-rename
// directory. Checking each in turn covers the cases where only some drifted;
// returning "" lets the caller say what actually happened instead of
// reporting a bare path as "not a grove project".
func (c *herdrContext) ResolveDir() string {
	for _, dir := range []string{c.CheckoutPath(), c.WorkspaceCwd, c.FocusedPaneCwd} {
		if dir == "" {
			continue
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}

func parseHerdrContext(raw string) (*herdrContext, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("no herdr plugin context (is grove being run as a herdr plugin?)")
	}
	var ctx herdrContext
	if err := json.Unmarshal([]byte(raw), &ctx); err != nil {
		return nil, fmt.Errorf("decode herdr plugin context: %w", err)
	}
	return &ctx, nil
}

// herdrEvent is the event envelope herdr passes to a plugin event hook,
// decoded from HERDR_PLUGIN_EVENT_JSON.
type herdrEvent struct {
	Event string
	Data  struct {
		WorkspaceID string `json:"workspace_id"`
		// Workspace carries the workspace's git provenance. On
		// worktree.removed its repo_root is the main checkout — the only
		// path in the payload that still exists by the time the hook runs.
		Workspace *struct {
			Worktree *struct {
				RepoRoot     string `json:"repo_root"`
				CheckoutPath string `json:"checkout_path"`
			} `json:"worktree"`
		} `json:"workspace"`
		Worktree *struct {
			Path   string `json:"path"`
			Branch string `json:"branch"`
			Label  string `json:"label"`
		} `json:"worktree"`
		AlreadyOpen bool `json:"already_open"`
	}
	AlreadyOpen bool
}

// CheckoutPath returns the worktree checkout the event refers to.
func (e *herdrEvent) CheckoutPath() string {
	if e.Data.Worktree != nil && e.Data.Worktree.Path != "" {
		return e.Data.Worktree.Path
	}
	if e.Data.Workspace != nil && e.Data.Workspace.Worktree != nil {
		return e.Data.Workspace.Worktree.CheckoutPath
	}
	return ""
}

// RepoRoot returns the main checkout of the repository the event's worktree
// belongs to, or "" when the payload carries no git provenance.
func (e *herdrEvent) RepoRoot() string {
	if e.Data.Workspace == nil || e.Data.Workspace.Worktree == nil {
		return ""
	}
	return e.Data.Workspace.Worktree.RepoRoot
}

// normalizeHerdrEventName converts herdr's wire form for an event name to the
// dotted form used in manifests and HERDR_PLUGIN_EVENT, so both resolve alike.
//
// Only the FIRST separator is a dot. The manifest name is
// `pane.agent_status_changed` and the wire form is
// `pane_agent_status_changed` — verified against herdr 0.8.0, which accepts the
// former and rejects both `pane.agent.status.changed` and
// `pane_agent_status_changed` as unknown events.
//
// So replacing every underscore would mangle every multi-word event, and this
// function is applied to HERDR_PLUGIN_EVENT too, which already arrives dotted —
// mangling it a second time. A name that already contains a dot is in manifest
// form and is left alone.
func normalizeHerdrEventName(name string) string {
	if strings.Contains(name, ".") {
		return name
	}
	return strings.Replace(name, "_", ".", 1)
}

func parseHerdrEvent(raw string) (*herdrEvent, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("no herdr plugin event (is grove being run as a herdr event hook?)")
	}
	var envelope struct {
		Event string          `json:"event"`
		Data  json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, fmt.Errorf("decode herdr plugin event: %w", err)
	}

	event := &herdrEvent{Event: normalizeHerdrEventName(envelope.Event)}
	if len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, &event.Data); err != nil {
			return nil, fmt.Errorf("decode herdr event data: %w", err)
		}
	}
	event.AlreadyOpen = event.Data.AlreadyOpen
	return event, nil
}

var herdrEventCmd = &cobra.Command{
	Use:    "herdr-event",
	Short:  "Handle a herdr plugin event (internal)",
	Hidden: true,
	Long: `Handles a herdr event hook invocation.

herdr passes the event in HERDR_PLUGIN_EVENT_JSON. This command is registered
by the plugin in integrations/herdr and is not meant to be run by hand.

On worktree.created and worktree.opened it reports whether grove already tracks
the checkout, so a worktree made through herdr's own UI does not silently bypass
grove's bootstrap. It only reports — adopting is left to 'grove adopt', which
runs post-create hooks the user should opt into.

On worktree.removed it reconciles grove's state with a removal herdr already
performed: the tracked entry is dropped (and last_worktree cleared) so 'grove
last' and 'grove ls' don't point at a checkout that no longer exists. It fires
no remove hooks and touches neither git nor the branch — the checkout is
already gone, and herdr's removal deliberately leaves the branch behind.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cli.NewStderr()

		event, err := parseHerdrEvent(os.Getenv("HERDR_PLUGIN_EVENT_JSON"))
		if err != nil {
			return err
		}

		// HERDR_PLUGIN_EVENT carries the dotted name directly; prefer it when
		// present since it is the value the manifest matched on.
		name := event.Event
		if fromEnv := os.Getenv("HERDR_PLUGIN_EVENT"); fromEnv != "" {
			name = normalizeHerdrEventName(fromEnv)
		}

		// worktree.removed fires only when herdr itself removed the worktree
		// (verified on 0.8.0: the grove-rm flow — git removes the checkout,
		// then the workspace closes — fires only workspace.closed, so there is
		// no loop between grove rm's workspace close and this branch).
		if name == "worktree.removed" {
			return reconcileRemovedWorktree(event, nil)
		}

		// worktree.created fires when herdr creates the checkout itself;
		// worktree.opened when it adopts an existing one into a workspace.
		// Both hand us a worktree grove may know nothing about, and herdr emits
		// exactly one of them per worktree — subscribing to only one silently
		// skips half the cases this hook exists for.
		if name != "worktree.opened" && name != "worktree.created" {
			return nil
		}
		// Focusing an already-open workspace re-fires worktree.opened. Staying
		// quiet keeps this from nagging every time the user clicks a workspace.
		// worktree.created carries no already_open field, so this is a no-op
		// there.
		if event.AlreadyOpen {
			return nil
		}

		path := event.CheckoutPath()
		if path == "" {
			return nil
		}

		tracked, err := groveTracksWorktree(path)
		if err != nil {
			// Not a grove project, or not readable — nothing to say.
			return nil //nolint:nilerr // a non-grove worktree is not an error
		}
		if tracked {
			return nil
		}

		cli.Warning(w, "grove does not track the worktree at %s", path)
		cli.Faint(w, "Run 'grove adopt %s' to bootstrap it (state, excludes, hooks).", path)

		// An event hook's stderr reaches `herdr plugin log list` and nothing
		// else, so on its own the one thing this hook exists to say would
		// never be seen. Raise a notification as well. Best-effort — the log
		// line is written either way, and a failure here is not the user's
		// problem to act on.
		reason, err := mux.Notify(
			"grove: worktree not tracked",
			fmt.Sprintf("%s — run 'grove adopt %s' to bootstrap it", filepath.Base(path), path),
		)
		switch {
		case err != nil:
			log.Printf("herdr notification failed: %v", err)
		case reason != "" && reason != "shown":
			// Suppressed by the user's `[ui.toast] delivery` setting or
			// herdr's rate limiter. Nothing grove can do about it, but it
			// explains an adoption prompt that never appeared.
			log.Printf("herdr suppressed the adoption notification: %s", reason)
		}
		return nil
	},
}

// groveContextForPath builds a minimal context for a directory, without the
// cwd-relative discovery, drift notices, or plugin registration that
// RequireGroveContext performs. herdr invokes plugin commands from an
// arbitrary cwd, so discovery has to start from the path herdr named.
func groveContextForPath(path string) (*GroveContext, error) {
	groveDir, err := grove.FindRoot(path)
	if err != nil {
		return nil, err
	}
	if groveDir == "" {
		return nil, fmt.Errorf("%s is not inside a grove project", path)
	}

	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize state: %w", err)
	}

	cfg, err := config.LoadFromGroveDir(groveDir)
	if err != nil {
		cfg = config.LoadDefaults()
	}

	return &GroveContext{
		GroveDir:    groveDir,
		ProjectRoot: grove.MustProjectRoot(groveDir),
		State:       stateMgr,
		Config:      cfg,
	}, nil
}

// groveTracksWorktree reports whether the checkout at path belongs to a grove
// project that already has it in state.
func groveTracksWorktree(path string) (bool, error) {
	ctx, err := groveContextForPath(path)
	if err != nil {
		return false, err
	}

	mgr, err := ctx.WorktreeManager()
	if err != nil {
		return false, err
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = path
	}
	// The main checkout is always implicitly tracked.
	if same, _ := samePath(resolved, ctx.ProjectRoot); same {
		return true, nil
	}

	name := mgr.DisplayNameForPath(resolved)
	if name == "" {
		return false, nil
	}
	ws, err := ctx.State.GetWorktree(name)
	return err == nil && ws != nil, nil
}

// reconcileRemovedWorktree drops grove's record of a worktree herdr just
// removed. herdr performs a clean `git worktree remove` (verified on 0.8.0:
// directory and .git/worktrees registration both gone, branch left behind), so
// git needs nothing from us — but .grove/state.json still tracks the checkout,
// `grove last` errors if last_worktree points at it, and on the tmux backend a
// session may linger over the dead directory. This fixes all three.
//
// muxOverride replaces the context's multiplexer in tests; pass nil in
// production.
//
// Deliberate non-actions: no user or plugin remove hooks fire (a background
// event must not run side effects the user didn't opt into — same stance as
// the adoption hook), no notification is raised (nothing here is actionable),
// and neither git nor the branch is touched. A worktree with a running docker
// stack is NOT stopped; its project name is logged before the record that
// names it is destroyed.
//
// This is a background hook, but it still honors the <500ms budget: one git
// rev-parse for discovery, one state save, and — tmux only — one has-session
// and at most one kill-session.
func reconcileRemovedWorktree(event *herdrEvent, muxOverride mux.Multiplexer) error {
	removed := event.CheckoutPath()
	repoRoot := event.RepoRoot()
	if removed == "" || repoRoot == "" {
		return nil
	}

	// Discovery must anchor on the main repo: the removed checkout no longer
	// exists, so the create/open handler's path-based discovery would fail.
	ctx, err := groveContextForPath(repoRoot)
	if err != nil {
		return nil //nolint:nilerr // a non-grove repo's worktree is not our business
	}

	// Match by the recorded path. State paths are symlink-resolved when
	// recorded (adopt/new both canonicalize), and the removed path is
	// canonicalized through its still-existing parent, so both sides compare
	// in resolved form. A miss degrades to today's behavior — a stale entry
	// that `grove repair` flags as orphan_state.
	canonical := canonicalGonePath(removed)
	var name string
	var ws *state.WorktreeState
	for n, entry := range ctx.State.GetState().Worktrees {
		if canonicalGonePath(entry.Path) == canonical {
			name, ws = n, entry
			break
		}
	}
	if name == "" {
		return nil
	}

	// The log line is the only record that survives (it reaches
	// `herdr plugin log list`), so surface anything the dropped entry knew
	// that the user might still need.
	if ws.DockerProject != "" {
		log.Printf("herdr removed worktree '%s' which had docker project %q — its containers were not stopped", name, ws.DockerProject)
	}

	// Drops the entry and clears last_worktree if it pointed here.
	if err := ctx.State.RemoveWorktree(name); err != nil {
		return fmt.Errorf("herdr removed the worktree at %s but grove state cleanup failed: %w", removed, err)
	}
	log.Printf("removed '%s' from grove state after herdr removed its worktree", name)

	// Reap a session left pointing at the dead directory. Skipped on the
	// herdr backend: the workspace close is herdr's own removal flow, and a
	// mutation callback into the server mid-event dispatch is unverified
	// territory (only `notification show` has been proven safe there).
	m := muxOverride
	if m == nil {
		m = ctx.Mux()
	}
	if m.Backend() != mux.BackendHerdr && m.Available() {
		if mgr, err := ctx.WorktreeManager(); err == nil {
			target := muxTarget(mgr, name, removed)
			if exists, err := m.Exists(target); err == nil && exists {
				if err := m.Kill(target); err != nil {
					log.Printf("failed to kill session for removed worktree '%s': %v", name, err)
				}
			}
		}
	}
	return nil
}

// canonicalGonePath canonicalizes a path that may no longer exist: the leaf is
// gone, but its parent usually survives, so resolving the parent and rejoining
// the basename yields the same form EvalSymlinks would have produced (modulo a
// symlinked leaf, which grove never creates for worktrees).
func canonicalGonePath(p string) string {
	p = filepath.Clean(p)
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
		return filepath.Join(resolved, filepath.Base(p))
	}
	return p
}

// samePath compares two paths with symlinks resolved on both sides.
func samePath(a, b string) (bool, error) {
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		ra = a
	}
	rb, err := filepath.EvalSymlinks(b)
	if err != nil {
		rb = b
	}
	return filepath.Clean(ra) == filepath.Clean(rb), nil
}

var herdrActionCmd = &cobra.Command{
	Use:    "herdr-action <action>",
	Short:  "Run a herdr plugin action (internal)",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	Long: `Runs a herdr plugin action against the invoking workspace.

herdr passes the workspace in HERDR_PLUGIN_CONTEXT_JSON. This command is
registered by the plugin in integrations/herdr and is not meant to be run by
hand.

Actions:
  status  Print grove's view of the workspace's worktree.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cli.NewStdout()

		hctx, err := parseHerdrContext(os.Getenv("HERDR_PLUGIN_CONTEXT_JSON"))
		if err != nil {
			return err
		}

		path := hctx.ResolveDir()
		if path == "" {
			// Every path herdr knows for this workspace is gone — almost
			// always a worktree that grove renamed or removed underneath it.
			// Naming that is more useful than reporting the dead path as "not
			// a grove project".
			if stale := hctx.CheckoutPath(); stale != "" {
				cli.Warning(w, "this herdr workspace points at %s, which no longer exists", stale)
				cli.Faint(w, "The worktree was renamed or removed. Close the workspace, or reopen it to refresh.")
				return nil
			}
			return fmt.Errorf("herdr workspace has no directory to act on")
		}

		switch args[0] {
		case "status":
			tracked, err := groveTracksWorktree(path)
			if err != nil {
				return fmt.Errorf("%s is not inside a grove project", path)
			}
			if tracked {
				cli.Success(w, "grove tracks %s", path)
				return nil
			}
			cli.Warning(w, "grove does not track %s", path)
			cli.Faint(w, "Run 'grove adopt %s' to bootstrap it.", path)
			return nil
		default:
			return fmt.Errorf("unknown herdr action %q", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(herdrEventCmd)
	rootCmd.AddCommand(herdrActionCmd)
}
