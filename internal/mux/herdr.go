package mux

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

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
	// attach runs the blocking interactive client.
	attach func() error
	// available reports whether the herdr binary is installed.
	available func() bool
	// env reads an environment variable.
	env func(string) string
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

var _ Multiplexer = (*HerdrBackend)(nil)

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
func (b *HerdrBackend) Ensure(t Target) error {
	opened, err := b.open(t)
	if err == nil {
		b.labelTab(opened.Tab, t)
		return nil
	}

	if errHasCode(err, "worktree_not_found", "not_git_worktree") {
		return b.mainCheckout(t)
	}
	return err
}

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

// mainCheckout handles a target herdr will not "open" because it is not a
// linked worktree — in practice the repository's own checkout.
//
// grove does not create a workspace here. herdr already materializes one for
// the repository when it opens any linked worktree, and that workspace is what
// its sidebar groups the worktrees under; creating and owning it from grove
// would reach past worktree lifecycle into territory herdr and the user's
// agents manage. So: adopt the repository workspace if it exists, and
// otherwise report the target as unmanaged so the caller just changes
// directory.
func (b *HerdrBackend) mainCheckout(t Target) error {
	exists, err := b.Exists(t)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return fmt.Errorf("%w: %s is a repository checkout, not a linked worktree", errUnmanaged, t.Path)
}

// EnsureWithCommand adopts the checkout, then runs command in its root pane
// when the workspace was newly created. An already-open workspace is left
// alone, matching the tmux backend's behavior.
func (b *HerdrBackend) EnsureWithCommand(t Target, command string) error {
	if command == "" {
		return b.Ensure(t)
	}

	opened, err := b.open(t)
	if err != nil {
		if !errHasCode(err, "worktree_not_found", "not_git_worktree") {
			return err
		}
		// Repository checkout: grove does not create a workspace for it. If
		// herdr already has one, run the command in its pane; otherwise the
		// target is unmanaged and there is no pane to run anything in.
		if err := b.mainCheckout(t); err != nil {
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
func (b *HerdrBackend) Exists(t Target) (bool, error) {
	sessions, err := b.List()
	if err != nil {
		return false, err
	}
	_, ok := NewIndex(sessions).Lookup(t)
	return ok, nil
}

// List returns every herdr workspace, mapped onto grove's session model.
//
// One call yields id, label, focus, checkout path and rolled-up agent status,
// which is everything grove's listing and TUI need.
func (b *HerdrBackend) List() ([]Session, error) {
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
// already focused the workspace it should land on.
func (b *HerdrBackend) AttachHint(Target) string { return herdrBinary }

// Attach focuses the target workspace, then starts the interactive client.
//
// herdr's launch flags carry no workspace selector, so focus must happen
// first — attaching first would land the user wherever they last were.
func (b *HerdrBackend) Attach(t Target) error {
	if err := b.focus(t); err != nil {
		return err
	}
	return b.attach()
}

// Switch focuses the target workspace in the running client.
func (b *HerdrBackend) Switch(t Target) error { return b.focus(t) }

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

// Kill closes the herdr workspace, leaving the checkout on disk.
//
// It deliberately does not use `herdr worktree remove`, which shells out to
// `git worktree remove` and would bypass grove's protection rules. Removing a
// target that has no workspace is a no-op, mirroring the tmux backend.
func (b *HerdrBackend) Kill(t Target) error {
	id, err := b.resolve(t)
	if err != nil {
		if ErrNoSession(err) {
			return nil
		}
		return err
	}
	_, err = b.call([]string{"workspace", "close", id})
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

	raw, err := b.call([]string{"pane", "list", "--workspace", id})
	if err != nil {
		return nil, err
	}
	var listed struct {
		Panes []herdrPane `json:"panes"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		return nil, fmt.Errorf("decode pane list: %w", err)
	}
	pane, ok := focusedPane(listed.Panes)
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
	raw, err := b.call([]string{"pane", "list", "--workspace", id})
	if err != nil {
		return "", err
	}
	var listed struct {
		Panes []herdrPane `json:"panes"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		return "", fmt.Errorf("decode pane list: %w", err)
	}
	pane, ok := focusedPane(listed.Panes)
	if !ok {
		return "", errNoSession
	}
	return pane.PaneID, nil
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
	var info struct {
		ForegroundProcesses []struct {
			Name string `json:"name"`
		} `json:"foreground_processes"`
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return "", err
	}
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
	out, runErr := b.run(args)

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
	return nil, fmt.Errorf("herdr %s: unexpected output: %s", strings.Join(args, " "), truncate(string(out), 200))
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
		Status:  attachStatus(w.Focused),
		Agent:   parseAgentStatus(w.AgentStatus),
		Windows: w.PaneCount,
	}
	if w.Worktree != nil {
		s.Path = w.Worktree.CheckoutPath
	}
	return s
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
// generated for it — its own number — rather than one a user or grove chose.
func (t herdrTab) hasDefaultLabel() bool {
	return t.Label == "" || t.Label == strconv.Itoa(t.Number)
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
		_, err := exec.LookPath(herdrBinary)
		herdrAvailableResult = err == nil
	})
	return herdrAvailableResult
}

// runHerdr executes a herdr CLI command under the shared timeout budget.
// Combined output is returned because herdr writes error envelopes to stderr.
func runHerdr(args []string) ([]byte, error) {
	return cmdexec.CombinedOutput(context.TODO(), herdrBinary, args, "", cmdexec.Herdr)
}

// attachHerdr starts the interactive client, blocking until the user detaches.
func attachHerdr() error {
	cmd := exec.Command(herdrBinary)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
