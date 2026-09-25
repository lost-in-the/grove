package mux

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lost-in-the/grove/internal/cmdexec"
)

// herdrBinary is the CLI grove shells out to. Every call is a thin client over
// herdr's local socket, so invocations are cheap but do require a live server.
const herdrBinary = "herdr"

// HerdrBackend drives herdr.
//
// Identity is the checkout path, not the label: herdr workspace labels are
// cosmetic and user-renameable, while `workspace list` reports each
// workspace's git checkout path. Every method therefore resolves a Target to
// an opaque workspace id by path before acting.
type HerdrBackend struct {
	// run executes a herdr invocation and returns its output. Injectable so
	// the command contract can be tested without a herdr server.
	run func(args []string) ([]byte, error)
	// attach runs the blocking interactive client with the given launch
	// arguments.
	attach func(args []string) error
	// available reports whether the herdr binary is installed.
	available func() bool
	// env reads an environment variable.
	env func(string) string

	// session pins every call to one named herdr session. Empty means the
	// ambient one — whatever the CLI resolves from HERDR_SOCKET_PATH (set in
	// every pane), HERDR_SESSION, or the default. Only the per-session
	// siblings built by inSession set it.
	session string
	// located records targets whose existing workspace Ensure found in
	// another session, keyed by cleaned checkout path. Attach and AttachHint
	// follow it there instead of the ambient session.
	located map[string]herdrLocation

	// Read cache. Every herdr call is a subprocess spawn, and one `grove to`
	// used to spawn ten of them — four identical `workspace list` (Exists,
	// then a fresh resolve() inside PaneInfo, SendCommand, and focus) and two
	// identical `pane list` — against the repo's <500ms command budget. A
	// backend instance lives for a single grove command (the TUI resolves a
	// fresh one per call site), so caching pure reads for the instance's
	// lifetime trades no meaningful staleness for six fewer spawns. call()
	// drops both caches before any invocation that is not a known-pure read.
	mu           sync.Mutex
	sessionCache []Session
	sessionValid bool
	paneCache    map[string][]herdrPane
	// others caches the running sessions other than the ambient one; nil
	// means not fetched yet.
	others *[]string
}

// herdrLocation is where a target's workspace lives when that is not the
// ambient session.
type herdrLocation struct {
	session     string
	workspaceID string
}

// NewHerdr returns the herdr backend wired to the real CLI.
func NewHerdr() *HerdrBackend {
	return &HerdrBackend{
		run:       runHerdr,
		attach:    attachHerdr,
		available: herdrAvailable,
		env:       os.Getenv,
	}
}

var (
	_ Multiplexer    = (*HerdrBackend)(nil)
	_ SessionLocator = (*HerdrBackend)(nil)
)

// Backend returns BackendHerdr.
func (b *HerdrBackend) Backend() Backend { return BackendHerdr }

// Available reports whether the herdr binary is installed.
func (b *HerdrBackend) Available() bool { return b.available() }

// Inside reports whether grove is running inside a herdr-managed pane.
func (b *HerdrBackend) Inside() bool { return b.env("HERDR_ENV") == "1" }

// Ensure adopts the worktree checkout as a herdr workspace.
//
// `worktree open` is idempotent — herdr reuses an existing workspace for the
// same checkout and re-applies the label — so this needs no existence check.
// It never creates a checkout: grove owns worktree lifecycle, and herdr's own
// `worktree create` would impose its own naming and placement.
//
// The repository's own checkout goes through the same call. herdr answers it
// by adopting — or, if it has none yet, creating — the repository's parent
// workspace, the one its sidebar groups the project's worktrees under. That is
// herdr's own bookkeeping, the same workspace it materializes as a side effect
// of opening any linked worktree; grove never calls `workspace create`.
//
// A workspace another named session already has for the checkout is adopted
// rather than duplicated — see adoptElsewhere.
func (b *HerdrBackend) Ensure(t Target) error {
	if loc, ok := b.locateElsewhere(t); ok {
		return b.adoptElsewhere(t, loc)
	}

	opened, err := b.open(t)
	if err == nil {
		b.labelTab(opened.Tab, t)
		return nil
	}

	if errHasCode(err, "worktree_not_found", "not_git_worktree") {
		return b.adoptUnopenable(t)
	}
	if errServerUnusable(err) {
		return serverUnmanaged(err)
	}
	return err
}

// serverUnmanaged maps an unusable server onto errUnmanaged: with no herdr
// server to talk to there is no session to manage, and callers already know
// how to degrade an unmanaged target to a plain directory switch. Surfacing it
// as a hard error instead would make every `grove to` fail outright — the
// exact opposite of what `grove doctor` promises ("grove will fall back to
// plain directory switching"). The tmux backend behaves the same way: a dead
// tmux server reads as "no sessions", never as a fatal switch error.
//
// The original error stays in the chain so callers can tell a stopped server
// (expected, silent) from a protocol mismatch (worth a hint — it follows every
// herdr upgrade until the old server is restarted).
func serverUnmanaged(err error) error {
	return fmt.Errorf("%w: herdr server unavailable: %w", errUnmanaged, err)
}

// herdrBusyRetryDelay is how long open waits before retrying a `worktree_busy`
// refusal. herdr runs worktree discovery on a small pool of background slots
// and turns a request away, rather than queueing it, when every slot is taken
// ("too many worktree checks are pending; retry shortly"). One short retry
// rides out a burst without eating much of the 500ms command budget. A var so
// tests need not sleep.
var herdrBusyRetryDelay = 150 * time.Millisecond

// open runs `worktree open` and decodes the response, which carries the
// workspace, its tab, and the root pane — everything a caller needs without a
// follow-up lookup.
func (b *HerdrBackend) open(t Target) (herdrOpened, error) {
	var opened herdrOpened

	args, err := b.openArgs(t)
	if err != nil {
		return opened, err
	}
	raw, err := b.call(args)
	if errHasCode(err, "worktree_busy") {
		time.Sleep(herdrBusyRetryDelay)
		raw, err = b.call(args)
	}
	if err != nil {
		return opened, err
	}
	if err := json.Unmarshal(raw, &opened); err != nil {
		return opened, fmt.Errorf("decode worktree open response: %w", err)
	}
	return opened, nil
}

// labelTab names the workspace's tab after the worktree.
//
// herdr labels a new tab with its own number, so nothing ever sets a real
// title and the terminal falls back to naming the window after whatever
// process launched it — which is how ghostty and cmux end up showing "grove",
// or the entire `grove to <name>` command line, where the worktree name
// belongs. A label the user (or an earlier grove run) already chose is never
// overwritten.
//
// Best-effort: a working session with a dull tab title is not a reason to fail
// the command that created it.
func (b *HerdrBackend) labelTab(tab herdrTab, t Target) {
	if !tab.hasDefaultLabel() {
		return
	}
	b.renameTab(tab.TabID, t.DisplayName())
}

// renameTab sets a tab's label, ignoring failures — see labelTab.
func (b *HerdrBackend) renameTab(tabID, name string) {
	if tabID == "" || name == "" {
		return
	}
	_, _ = b.call([]string{"tab", "rename", tabID, name})
}

// adoptUnopenable handles a target herdr refuses to open as a worktree — a
// path its git worktree list does not contain (`worktree_not_found`), or one
// outside any git checkout (`not_git_worktree`). The repository's own checkout
// is not one of these; herdr opens that as the parent workspace.
//
// grove does not create a workspace for such a path: nothing herdr recognizes
// would group or track it. So adopt a workspace that already covers the path,
// and otherwise report the target unmanaged so the caller just changes
// directory.
func (b *HerdrBackend) adoptUnopenable(t Target) error {
	exists, err := b.Exists(t)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return fmt.Errorf("%w: herdr does not recognize %s as a git worktree", errUnmanaged, t.Path)
}

// EnsureWithCommand adopts the checkout, then runs command in its root pane
// when the workspace was newly created. An already-open workspace is left
// alone, matching the tmux backend's behavior.
func (b *HerdrBackend) EnsureWithCommand(t Target, command string) error {
	if command == "" {
		return b.Ensure(t)
	}

	// An existing workspace in another session is an existing session: adopt
	// it and leave its panes alone, as for an already-open one below.
	if loc, ok := b.locateElsewhere(t); ok {
		return b.adoptElsewhere(t, loc)
	}

	opened, err := b.open(t)
	if err != nil {
		if errServerUnusable(err) {
			return serverUnmanaged(err)
		}
		if !errHasCode(err, "worktree_not_found", "not_git_worktree") {
			return err
		}
		// A path herdr will not open: if a workspace already covers it, run
		// the command in its pane; otherwise the target is unmanaged and there
		// is no pane to run anything in.
		if err := b.adoptUnopenable(t); err != nil {
			return err
		}
		pane, perr := b.rootPane(t)
		if perr != nil {
			return perr
		}
		return b.runInPane(pane, command)
	}
	b.labelTab(opened.Tab, t)
	if opened.AlreadyOpen {
		return nil
	}
	return b.runInPane(opened.RootPane.PaneID, command)
}

// Exists reports whether a herdr workspace already covers the checkout.
//
// An unusable server — stopped, or speaking a different protocol than the
// CLI after an upgrade — means no workspace is reachable, not that the
// question failed: callers treat an Exists error as fatal (it aborts `grove
// to` before the cd directive), while "false" routes them through Ensure,
// which reports the target unmanaged and lets the plain directory switch
// proceed.
func (b *HerdrBackend) Exists(t Target) (bool, error) {
	sessions, err := b.List()
	if err != nil {
		if errServerUnusable(err) {
			return false, nil
		}
		return false, err
	}
	_, ok := NewIndex(sessions).Lookup(t)
	return ok, nil
}

// List returns every herdr workspace, mapped onto grove's session model.
//
// One call yields id, label, focus, checkout path and rolled-up agent status,
// which is everything grove's listing and TUI need. The result is cached for
// the backend instance's lifetime (callers treat it as read-only) and
// refetched after any mutating call.
func (b *HerdrBackend) List() ([]Session, error) {
	b.mu.Lock()
	if b.sessionValid {
		sessions := b.sessionCache
		b.mu.Unlock()
		return sessions, nil
	}
	b.mu.Unlock()

	raw, err := b.call([]string{"workspace", "list"})
	if err != nil {
		return nil, err
	}

	var result struct {
		Workspaces []herdrWorkspace `json:"workspaces"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode workspace list: %w", err)
	}

	sessions := make([]Session, 0, len(result.Workspaces))
	for _, ws := range result.Workspaces {
		sessions = append(sessions, ws.session())
	}
	b.mu.Lock()
	b.sessionCache, b.sessionValid = sessions, true
	b.mu.Unlock()
	return sessions, nil
}

// Current returns the label of the workspace grove's pane belongs to.
func (b *HerdrBackend) Current() (string, error) {
	id := b.env("HERDR_WORKSPACE_ID")
	if id == "" {
		return "", errNotInside
	}

	raw, err := b.call([]string{"workspace", "get", id})
	if err != nil {
		return "", err
	}
	var result struct {
		Workspace herdrWorkspace `json:"workspace"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("decode workspace get: %w", err)
	}
	return result.Workspace.Label, nil
}

// AttachHint returns the command that attaches a herdr client. herdr has no
// per-workspace launch flag, so the hint is the bare client — grove has
// already focused the workspace it should land on — plus the session to
// attach to when Ensure found the target's workspace in another one.
func (b *HerdrBackend) AttachHint(t Target) string {
	if loc, ok := b.locatedFor(t); ok {
		return herdrBinary + " --session " + loc.session
	}
	return herdrBinary
}

// Attach focuses the target workspace, then starts the interactive client.
//
// herdr's launch flags carry no workspace selector, so focus must happen
// first — attaching first would land the user wherever they last were.
// Focusing moves every client attached to that session, and a client that
// attaches afterwards starts on the focused workspace (verified on 0.9.1).
func (b *HerdrBackend) Attach(t Target) error {
	if loc, ok := b.locatedFor(t); ok {
		if _, err := b.inSession(loc.session).call([]string{"workspace", "focus", loc.workspaceID}); err != nil {
			return err
		}
		return b.attach([]string{"--session", loc.session})
	}
	if err := b.focus(t); err != nil {
		return err
	}
	return b.attach(nil)
}

// Switch focuses the target workspace in the running client.
func (b *HerdrBackend) Switch(t Target) error {
	if loc, ok := b.locatedFor(t); ok {
		_, err := b.inSession(loc.session).call([]string{"workspace", "focus", loc.workspaceID})
		return err
	}
	return b.focus(t)
}

// LocatedIn names the other session whose workspace Ensure adopted for t, or
// "" when t's workspace is (or would be) in the ambient session.
func (b *HerdrBackend) LocatedIn(t Target) string {
	loc, _ := b.locatedFor(t)
	return loc.session
}

// Rename relabels the workspace. The checkout path is the identity, so this is
// cosmetic — but it keeps herdr's sidebar aligned with grove's naming.
func (b *HerdrBackend) Rename(from, to Target) error {
	id, err := b.resolve(from)
	if err != nil {
		return err
	}
	if _, err := b.call([]string{"workspace", "rename", id, to.Name}); err != nil {
		return err
	}
	// Keep the tab title in step with the workspace, or the window would go on
	// advertising the old worktree name. Only touches a tab still carrying a
	// grove-set or default label — see labelTab.
	for _, tab := range b.tabs(id) {
		if tab.Label == from.DisplayName() || tab.hasDefaultLabel() {
			b.renameTab(tab.TabID, to.DisplayName())
		}
	}
	return nil
}

// tabs lists a workspace's tabs. Best-effort: it serves cosmetic relabelling,
// so a failure yields no tabs rather than an error.
func (b *HerdrBackend) tabs(workspaceID string) []herdrTab {
	raw, err := b.call([]string{"tab", "list", "--workspace", workspaceID})
	if err != nil {
		return nil
	}
	var listed struct {
		Tabs []herdrTab `json:"tabs"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		return nil
	}
	return listed.Tabs
}

// Kill closes the target's herdr workspace — in the ambient session and in
// every other running named session that has one — leaving the checkout on
// disk.
//
// It deliberately does not use `herdr worktree remove`, which shells out to
// `git worktree remove` and would bypass grove's protection rules. Removing a
// target that has no workspace is a no-op, mirroring the tmux backend.
//
// herdr sessions are independent servers, and nothing stops the same checkout
// being open in two of them. Closing only the ambient copy left the other one
// behind as an orphan pointing at a deleted directory. Other sessions are
// matched on checkout path only: by the time grove removes a worktree its
// directory is usually gone, and a label match in a session grove did not
// open could close an unrelated workspace. Stopped sessions cannot be reached;
// herdr drops such a workspace's worktree link itself when it next starts.
func (b *HerdrBackend) Kill(t Target) error {
	err := b.killAmbient(t)
	if errServerUnusable(err) {
		// The ambient server being down says nothing about the others.
		err = nil
	}
	for _, name := range b.otherSessions() {
		sib := b.inSession(name)
		sessions, lerr := sib.List()
		if lerr != nil {
			continue
		}
		for _, s := range sessions {
			if !sameGonePath(s.Path, t.Path) {
				continue
			}
			if _, cerr := sib.call([]string{"workspace", "close", s.ID}); cerr != nil && err == nil {
				err = fmt.Errorf("close workspace %s in herdr session %q: %w", s.ID, name, cerr)
			}
		}
	}
	return err
}

// killAmbient closes the target's workspace in the ambient session.
func (b *HerdrBackend) killAmbient(t Target) error {
	id, err := b.resolve(t)
	if err != nil {
		if ErrNoSession(err) {
			return nil
		}
		return err
	}
	_, err = b.call([]string{"workspace", "close", id})
	if errHasCode(err, "workspace_group_close_required") {
		// Since herdr 0.9.0, closing a repository's parent workspace while
		// worktree workspaces are open under it needs --group, which would
		// close every one of them. grove never asks for that — each worktree's
		// session is closed on its own — so name what is in the way instead.
		return fmt.Errorf("herdr workspace %s still has worktree workspaces open under it; close those first: %w", id, err)
	}
	return err
}

// PaneInfo returns the state of the workspace's focused pane.
//
// Two calls: `pane list` gives the cwd and whether an agent occupies the pane,
// and `pane process-info` names the foreground process so IsShell can match
// the tmux backend's semantics. Only the drift check reaches this path.
func (b *HerdrBackend) PaneInfo(t Target) (*PaneInfo, error) {
	id, err := b.resolve(t)
	if err != nil {
		return nil, err
	}

	panes, err := b.paneList(id)
	if err != nil {
		return nil, err
	}
	pane, ok := focusedPane(panes)
	if !ok {
		return nil, errNoSession
	}

	info := &PaneInfo{
		CurrentPath: pane.cwd(),
		HasAgent:    pane.Agent != nil && *pane.Agent != "",
	}
	if info.HasAgent {
		// No need to name the foreground process: an agent occupying the pane
		// already disqualifies it from receiving a `cd`.
		info.CurrentCommand = *pane.Agent
		return info, nil
	}

	if name, err := b.foregroundCommand(pane.PaneID); err == nil {
		info.CurrentCommand = name
	}
	return info, nil
}

// Notify raises a herdr notification.
//
// This is the only way a plugin event hook can reach a person. A hook's stdout
// and stderr go to `herdr plugin log list` and nowhere else, so a hook that
// only writes there has effectively said nothing.
//
// Verified against herdr 0.8.0 that a hook may call back into the socket API
// while the server is still running it — the call returns `shown: true` and
// does not deadlock against the server that spawned the hook.
// It returns how herdr handled the request: "shown" means it reached the user,
// while "disabled", "rate_limited", "no_foreground_client" and "busy" mean it
// did not. Those are not errors — delivery is the user's configuration
// (`[ui.toast] delivery`, which ships commented out as `off`) and herdr's own
// rate limiter, neither of which a plugin can override. Callers should log the
// reason rather than report a failure.
func (b *HerdrBackend) Notify(title, body string) (string, error) {
	if title == "" {
		return "", fmt.Errorf("notification title cannot be empty")
	}
	args := []string{"notification", "show", title}
	if body != "" {
		args = append(args, "--body", body)
	}
	raw, err := b.call(args)
	if err != nil {
		return "", err
	}
	var result struct {
		Shown  bool   `json:"shown"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		// The notification may well have been shown; only the report is
		// unreadable, and that is not worth failing over.
		return "", nil
	}
	return result.Reason, nil
}

// Notify raises a herdr notification through the real CLI. Returns an error
// when herdr is not installed, so callers can stay quiet rather than reporting
// a failure the user cannot act on.
func Notify(title, body string) (string, error) {
	b := NewHerdr()
	if !b.Available() {
		return "", fmt.Errorf("herdr is not installed")
	}
	return b.Notify(title, body)
}

// SendCommand runs a command line in the workspace's focused pane.
func (b *HerdrBackend) SendCommand(t Target, command string) error {
	pane, err := b.rootPane(t)
	if err != nil {
		return err
	}
	return b.runInPane(pane, command)
}

// --- internals ---

// openArgs builds the `worktree open` invocation.
//
// herdr resolves two different things here: --cwd names the *source
// repository* whose worktree list is searched, and --path names the checkout
// to open. They are not interchangeable — passing the linked worktree as --cwd
// fails with linked_worktree_source ("New and open worktree actions start from
// the repo parent workspace"), and omitting --cwd entirely falls back to the
// focused workspace, which does not exist when grove runs outside a client.
func (b *HerdrBackend) openArgs(t Target) ([]string, error) {
	if t.Repo == "" {
		return nil, fmt.Errorf("herdr needs the repository root to open %s: %w", t.Path, errNoRepoRoot)
	}
	args := []string{
		"worktree", "open",
		"--cwd", t.Repo,
		"--path", t.Path,
		"--no-focus",
	}
	if t.Name != "" {
		args = append(args, "--label", t.Name)
	}
	return args, nil
}

// --- named sessions ---
//
// A herdr session is its own server: its own socket, state, and workspaces,
// with no awareness of the others (verified on 0.9.1 — `worktree open` checks
// only its own server for an existing workspace). So the same checkout opened
// under two sessions gets two workspaces, and closing one leaves the other
// pointing at whatever grove later deletes. grove talks to the ambient session
// and looks across the others only on the two paths where duplicates are made
// or orphaned: opening a workspace the ambient session lacks, and removal.

// herdrSessionInfo is one entry of `herdr session list --json`.
type herdrSessionInfo struct {
	Name       string `json:"name"`
	Default    bool   `json:"default"`
	Running    bool   `json:"running"`
	SocketPath string `json:"socket_path"`
}

// inSession returns a backend that addresses the named session, sharing this
// one's process plumbing but none of its caches.
func (b *HerdrBackend) inSession(name string) *HerdrBackend {
	return &HerdrBackend{run: b.run, attach: b.attach, available: b.available, env: b.env, session: name}
}

// otherSessions lists the running herdr sessions other than the ambient one.
//
// `herdr session list --json` reads the sessions directory and probes each
// socket without sending a request, so it is one cheap process even with many
// sessions (it prints bare JSON, not an API envelope). Any failure yields no
// sessions: looking elsewhere is an improvement over the ambient-only
// behavior, never a reason to fail the command.
func (b *HerdrBackend) otherSessions() []string {
	b.mu.Lock()
	if b.others != nil {
		others := *b.others
		b.mu.Unlock()
		return others
	}
	b.mu.Unlock()

	var others []string
	if out, err := b.run([]string{"session", "list", "--json"}); err == nil {
		var listed struct {
			Sessions []herdrSessionInfo `json:"sessions"`
		}
		if json.Unmarshal(bytes.TrimSpace(out), &listed) == nil {
			ambient := b.ambientSession(listed.Sessions)
			for _, s := range listed.Sessions {
				if s.Running && s.Name != "" && s.Name != ambient {
					others = append(others, s.Name)
				}
			}
		}
	}

	b.mu.Lock()
	b.others = &others
	b.mu.Unlock()
	return others
}

// ambientSession names the session a bare herdr call reaches, following the
// CLI's own precedence: HERDR_SOCKET_PATH (every pane sets it to its own
// server, and it outranks HERDR_SESSION), then HERDR_SESSION, then the
// default session. Returns "" for a socket no listed session owns.
func (b *HerdrBackend) ambientSession(sessions []herdrSessionInfo) string {
	if b.session != "" {
		return b.session
	}
	if sock := b.env("HERDR_SOCKET_PATH"); sock != "" {
		for _, s := range sessions {
			if s.SocketPath != "" && filepath.Clean(s.SocketPath) == filepath.Clean(sock) {
				return s.Name
			}
		}
		return ""
	}
	if name := b.env("HERDR_SESSION"); name != "" {
		return name
	}
	for _, s := range sessions {
		if s.Default {
			return s.Name
		}
	}
	return "default"
}

// locateElsewhere finds the target's workspace in another running session,
// but only when the ambient session has none — the one case where opening it
// here would make a duplicate. The lookup is by checkout path alone: a label
// is not proof of identity in a session grove may never have touched.
func (b *HerdrBackend) locateElsewhere(t Target) (herdrLocation, bool) {
	if t.Path == "" {
		return herdrLocation{}, false
	}
	if exists, err := b.Exists(t); err != nil || exists {
		return herdrLocation{}, false
	}
	for _, name := range b.otherSessions() {
		sessions, err := b.inSession(name).List()
		if err != nil {
			continue
		}
		if s, ok := NewIndex(sessions).Lookup(Target{Path: t.Path}); ok {
			return herdrLocation{session: name, workspaceID: s.ID}, true
		}
	}
	return herdrLocation{}, false
}

// adoptElsewhere settles a target whose workspace lives in another session.
//
// Outside herdr, that workspace is simply the target's session: remember it,
// so Attach focuses it there and attaches that session's client. Inside a
// herdr pane there is no way to move the user's client to another session from
// the CLI (a nested attach is refused by design), and opening a second
// workspace here is the duplicate this exists to prevent — so report the
// target unmanaged, with a hint naming where it is open.
func (b *HerdrBackend) adoptElsewhere(t Target, loc herdrLocation) error {
	if b.Inside() {
		return fmt.Errorf("%w: %w", errUnmanaged, &openElsewhereError{name: t.DisplayName(), session: loc.session})
	}
	b.mu.Lock()
	if b.located == nil {
		b.located = map[string]herdrLocation{}
	}
	b.located[filepath.Clean(t.Path)] = loc
	b.mu.Unlock()
	return nil
}

// locatedFor returns where Ensure found the target's workspace, when that was
// another session.
func (b *HerdrBackend) locatedFor(t Target) (herdrLocation, bool) {
	if t.Path == "" {
		return herdrLocation{}, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	loc, ok := b.located[filepath.Clean(t.Path)]
	return loc, ok
}

// openElsewhereError says a target is already open in another herdr session.
type openElsewhereError struct {
	name    string
	session string
}

func (e *openElsewhereError) Error() string {
	return fmt.Sprintf("'%s' is already open in herdr session %q", e.name, e.session)
}

func (b *HerdrBackend) focus(t Target) error {
	id, err := b.resolve(t)
	if err != nil {
		return err
	}
	_, err = b.call([]string{"workspace", "focus", id})
	return err
}

// resolve maps a target to herdr's opaque workspace id via the checkout path.
func (b *HerdrBackend) resolve(t Target) (string, error) {
	sessions, err := b.List()
	if err != nil {
		return "", err
	}
	s, ok := NewIndex(sessions).Lookup(t)
	if !ok {
		return "", fmt.Errorf("%w: %s", errNoSession, t.Path)
	}
	return s.ID, nil
}

func (b *HerdrBackend) rootPane(t Target) (string, error) {
	id, err := b.resolve(t)
	if err != nil {
		return "", err
	}
	panes, err := b.paneList(id)
	if err != nil {
		return "", err
	}
	pane, ok := focusedPane(panes)
	if !ok {
		return "", errNoSession
	}
	return pane.PaneID, nil
}

// paneList lists a workspace's panes, cached like List — the drift check and
// a follow-up SendCommand used to fetch the identical list back to back.
func (b *HerdrBackend) paneList(workspaceID string) ([]herdrPane, error) {
	b.mu.Lock()
	if panes, ok := b.paneCache[workspaceID]; ok {
		b.mu.Unlock()
		return panes, nil
	}
	b.mu.Unlock()

	raw, err := b.call([]string{"pane", "list", "--workspace", workspaceID})
	if err != nil {
		return nil, err
	}
	var listed struct {
		Panes []herdrPane `json:"panes"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		return nil, fmt.Errorf("decode pane list: %w", err)
	}
	b.mu.Lock()
	if b.paneCache == nil {
		b.paneCache = map[string][]herdrPane{}
	}
	b.paneCache[workspaceID] = listed.Panes
	b.mu.Unlock()
	return listed.Panes, nil
}

func (b *HerdrBackend) runInPane(paneID, command string) error {
	if paneID == "" {
		return errNoSession
	}
	_, err := b.call([]string{"pane", "run", paneID, command})
	return err
}

func (b *HerdrBackend) foregroundCommand(paneID string) (string, error) {
	raw, err := b.call([]string{"pane", "process-info", "--pane", paneID})
	if err != nil {
		return "", err
	}
	// The process list sits one level down, under process_info — verified
	// against herdr 0.8.0 and 0.9.1. Reading it from the top level decodes
	// cleanly into an empty list, which silently made every pane look like a
	// non-shell and disabled drift correction under herdr.
	var result struct {
		ProcessInfo struct {
			ForegroundProcesses []struct {
				Name string `json:"name"`
			} `json:"foreground_processes"`
		} `json:"process_info"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	info := result.ProcessInfo
	if len(info.ForegroundProcesses) == 0 {
		return "", nil
	}
	// These are the pane's foreground process *group* — siblings, not a nesting
	// chain, so there is no "innermost" entry to pick. The last one is the
	// newest, which is what names the pane in the cases IsShell cares about:
	// verified against herdr 0.8.0, a shell sitting at its prompt reports
	// exactly ["zsh"], and a shell running `sleep 45` reports exactly
	// ["sleep"].
	return info.ForegroundProcesses[len(info.ForegroundProcesses)-1].Name, nil
}

// call runs a herdr command and returns the decoded `result` object.
func (b *HerdrBackend) call(args []string) (json.RawMessage, error) {
	// Anything that may change the state the caches snapshot (open, close,
	// rename, focus …) drops them before running — unconditionally, since a
	// failed mutation may still have had effects.
	if !preservesHerdrReadCaches(args) {
		b.mu.Lock()
		b.sessionValid = false
		b.sessionCache = nil
		b.paneCache = nil
		b.mu.Unlock()
	}

	out, runErr := b.run(b.sessionArgs(args))

	// herdr prints a JSON envelope on both success and failure, and the error
	// body carries an actionable code — prefer it over the process exit status.
	if envelope, ok := parseEnvelope(out); ok {
		if envelope.Error != nil {
			return nil, &HerdrError{Code: envelope.Error.Code, Message: envelope.Error.Message}
		}
		if runErr == nil {
			return envelope.Result, nil
		}
	}

	if runErr != nil {
		return nil, fmt.Errorf("herdr %s: %w", strings.Join(args, " "), runErr)
	}
	// herdr's acknowledgement-only verbs — `pane run` is the one grove uses —
	// print nothing at all on success and exit 0; only their failures carry an
	// envelope. Treating that silence as "unexpected output" made every
	// `pane run` report failure after the command had already run. Callers
	// that expect a result still fail, at decode time, on the empty payload.
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
	}
	return nil, fmt.Errorf("herdr %s: unexpected output: %s", strings.Join(args, " "), truncate(string(out), 200))
}

// sessionArgs prefixes a pinned session. The global --session flag outranks
// HERDR_SOCKET_PATH, which every pane sets to its own server, so it reaches
// another session even from inside herdr where HERDR_SESSION would not.
func (b *HerdrBackend) sessionArgs(args []string) []string {
	if b.session == "" {
		return args
	}
	return append([]string{"--session", b.session}, args...)
}

// preservesHerdrReadCaches reports whether a herdr invocation leaves the
// cached snapshots valid: the pure reads, plus the verbs that touch nothing
// the caches hold (`pane run` starts a process in an existing pane —
// SendCommand runs it right after a resolve that just warmed the cache — and
// `notification show` touches no session state at all). The list is
// deliberately closed: an unknown verb counts as a mutation, so a new command
// can at worst waste a refetch, never serve stale data.
func preservesHerdrReadCaches(args []string) bool {
	if len(args) < 2 {
		return false
	}
	switch args[0] + " " + args[1] {
	case "workspace list", "workspace get", "pane list", "pane process-info", "tab list",
		"pane run", "notification show":
		return true
	}
	return false
}

type herdrEnvelope struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *herdrErrorBody `json:"error"`
}

// parseEnvelope extracts herdr's response from combined stdout+stderr.
//
// herdr writes one compact JSON object per line, but grove reads stdout and
// stderr together and an update notice or warning can share the stream. So
// rather than trusting position, scan for the line that actually parses as a
// response envelope. Later lines win: the error envelope herdr writes to
// stderr is the authoritative outcome.
func parseEnvelope(out []byte) (herdrEnvelope, bool) {
	var (
		found herdrEnvelope
		ok    bool
	)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var envelope herdrEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			continue
		}
		// An envelope always carries an id plus exactly one of result/error;
		// requiring that keeps stray JSON log lines from being mistaken for it.
		if envelope.ID == "" || (envelope.Result == nil && envelope.Error == nil) {
			continue
		}
		found, ok = envelope, true
	}
	return found, ok
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// --- wire types ---

type herdrErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HerdrError is a structured failure reported by the herdr CLI.
type HerdrError struct {
	Code    string
	Message string
}

func (e *HerdrError) Error() string { return fmt.Sprintf("herdr: %s (%s)", e.Message, e.Code) }

// ErrServerNotRunning reports whether err means no herdr server is listening.
func ErrServerNotRunning(err error) bool { return errHasCode(err, "server_not_running") }

// ErrProtocolMismatch reports whether err means the herdr CLI and the running
// server speak different protocol versions. herdr's JSON API needs an exact
// match, so this is what every call returns after herdr is upgraded until the
// old server is restarted.
func ErrProtocolMismatch(err error) bool { return errHasCode(err, "protocol_mismatch") }

// errServerUnusable reports whether err means grove cannot talk to the herdr
// server at all: none is running, or it cannot understand this CLI.
func errServerUnusable(err error) bool {
	return ErrServerNotRunning(err) || ErrProtocolMismatch(err)
}

// DegradedHint returns a one-line explanation for an unmanaged target that
// deserves one, or "" when degrading is the expected, silent outcome.
//
// A stopped server is expected — grove falls back to a plain directory switch
// and `grove doctor` explains it. A protocol mismatch is not: it follows every
// herdr upgrade until the old server is restarted, and a `grove to` that
// quietly stops switching workspaces would look like a grove bug.
//
// A worktree already open in another named session is likewise worth naming:
// grove declined to open a duplicate, and the user needs to know where the
// existing workspace is.
func DegradedHint(err error) string {
	var elsewhere *openElsewhereError
	if errors.As(err, &elsewhere) {
		return fmt.Sprintf("%s — changing directory only (switch sessions in herdr, or run `herdr --session %s` outside it)", elsewhere.Error(), elsewhere.session)
	}
	var he *HerdrError
	if !ErrProtocolMismatch(err) || !errors.As(err, &he) {
		return ""
	}
	return fmt.Sprintf("herdr sessions unavailable, changing directory only: %s", he.Message)
}

func errHasCode(err error, codes ...string) bool {
	var he *HerdrError
	if !errors.As(err, &he) {
		return false
	}
	for _, code := range codes {
		if he.Code == code {
			return true
		}
	}
	return false
}

type herdrWorkspace struct {
	WorkspaceID string `json:"workspace_id"`
	Label       string `json:"label"`
	Focused     bool   `json:"focused"`
	PaneCount   int    `json:"pane_count"`
	AgentStatus string `json:"agent_status"`
	Worktree    *struct {
		RepoRoot     string `json:"repo_root"`
		CheckoutPath string `json:"checkout_path"`
	} `json:"worktree"`
}

func (w herdrWorkspace) session() Session {
	s := Session{
		Name:    w.Label,
		ID:      w.WorkspaceID,
		Status:  focusStatus(w.Focused),
		Agent:   parseAgentStatus(w.AgentStatus),
		Windows: w.PaneCount,
	}
	if w.Worktree != nil {
		s.Path = w.Worktree.CheckoutPath
	}
	return s
}

// focusStatus maps herdr's server-wide focus onto grove's session status —
// see StatusActive for why this is not attached/detached.
func focusStatus(focused bool) Status {
	if focused {
		return StatusActive
	}
	return StatusOpen
}

// herdrOpened is the `worktree open` response. It reports the workspace, its
// tab, and the root pane in one shot.
type herdrOpened struct {
	RootPane    herdrPane `json:"root_pane"`
	Tab         herdrTab  `json:"tab"`
	AlreadyOpen bool      `json:"already_open"`
}

type herdrTab struct {
	TabID  string `json:"tab_id"`
	Label  string `json:"label"`
	Number int    `json:"number"`
}

// hasDefaultLabel reports whether the tab still carries the label herdr
// generated for it rather than one a user or grove chose.
//
// herdr's generated label is the tab's current *position* plus one, while
// TabInfo.number is a stable counter that never renumbers — so after an
// earlier tab closes the two disagree, and comparing against number would
// mistake herdr's own label for a user's. Any all-digit label is treated as
// generated instead: nobody names a tab a bare number on purpose.
func (t herdrTab) hasDefaultLabel() bool {
	if t.Label == "" {
		return true
	}
	for _, r := range t.Label {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

type herdrPane struct {
	PaneID        string  `json:"pane_id"`
	Focused       bool    `json:"focused"`
	Cwd           *string `json:"cwd"`
	ForegroundCwd *string `json:"foreground_cwd"`
	Agent         *string `json:"agent"`
}

func (p herdrPane) cwd() string {
	if p.ForegroundCwd != nil && *p.ForegroundCwd != "" {
		return *p.ForegroundCwd
	}
	if p.Cwd != nil {
		return *p.Cwd
	}
	return ""
}

func focusedPane(panes []herdrPane) (herdrPane, bool) {
	if len(panes) == 0 {
		return herdrPane{}, false
	}
	for _, p := range panes {
		if p.Focused {
			return p, true
		}
	}
	return panes[0], true
}

// parseAgentStatus maps herdr's snake_case status onto grove's. An
// unrecognized value means herdr knows something grove doesn't, which is
// closer to "unknown" than to "no agent".
func parseAgentStatus(s string) AgentStatus {
	switch AgentStatus(s) {
	case AgentIdle, AgentWorking, AgentBlocked, AgentDone, AgentUnknown:
		return AgentStatus(s)
	case AgentUnreported:
		return AgentUnreported
	default:
		return AgentUnknown
	}
}

// --- real process execution ---

var (
	herdrAvailableOnce   sync.Once
	herdrAvailableResult bool
)

func herdrAvailable() bool {
	herdrAvailableOnce.Do(func() {
		_, err := exec.LookPath(HerdrBinary())
		herdrAvailableResult = err == nil
	})
	return herdrAvailableResult
}

// HerdrBinary returns the herdr executable grove should run.
//
// herdr exports HERDR_BIN_PATH into every pane and plugin process, naming the
// binary of the server that owns them. That is herdr's documented way to call
// back into itself, and it stays protocol-compatible with the server even
// after a client-only update replaced whatever `herdr` is first on PATH. A
// value that does not name a file is ignored in favor of PATH.
func HerdrBinary() string {
	if p := os.Getenv("HERDR_BIN_PATH"); p != "" {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return herdrBinary
}

// HerdrServerStatus is herdr's own report on the server the CLI would talk to
// — the ambient session, as selected by HERDR_SOCKET_PATH / HERDR_SESSION.
type HerdrServerStatus struct {
	Running bool   `json:"running"`
	Version string `json:"version"`
	Session string `json:"session"`
	// Compatible is nil when no server is running.
	Compatible *bool `json:"compatible"`
	// RestartNeeded means the running server predates the installed CLI and
	// has to be restarted before the CLI can drive it.
	RestartNeeded bool `json:"restart_needed"`
}

// Usable reports whether the CLI can drive the server right now.
func (s HerdrServerStatus) Usable() bool {
	return s.Running && !s.RestartNeeded && (s.Compatible == nil || *s.Compatible)
}

// ProbeHerdrServer asks herdr whether its server is running and compatible
// with the installed CLI. `herdr status server --json` answers without an API
// envelope and exits 0 either way — verified on 0.8.0 and 0.9.1 — so this is
// the one check that tells "stopped" apart from "running, but needs a restart
// after an upgrade".
func ProbeHerdrServer() (HerdrServerStatus, error) {
	var status HerdrServerStatus
	out, err := cmdexec.Output(context.TODO(), HerdrBinary(), []string{"status", "server", "--json"}, "", cmdexec.Herdr)
	if err != nil {
		return status, fmt.Errorf("herdr status server: %w", err)
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &status); err != nil {
		return status, fmt.Errorf("decode herdr status server: %w", err)
	}
	return status, nil
}

// runHerdr executes a herdr CLI command under the shared timeout budget.
// Combined output is returned because herdr writes error envelopes to stderr.
func runHerdr(args []string) ([]byte, error) {
	return cmdexec.CombinedOutput(context.TODO(), HerdrBinary(), args, "", cmdexec.Herdr)
}

// attachHerdr starts the interactive client, blocking until the user detaches.
func attachHerdr(args []string) error {
	cmd := exec.Command(HerdrBinary(), args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
