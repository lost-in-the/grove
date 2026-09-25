package mux

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeHerdr records invocations and replays canned responses keyed by the
// first two argv words (the herdr command group and subcommand). A call pinned
// to a named session (`--session NAME ...`) is looked up under "@NAME key"
// first, so one fake can stand in for several independent herdr servers.
type fakeHerdr struct {
	mu         sync.Mutex // peer sessions are queried concurrently
	calls      [][]string
	peerCalls  [][]string // the subset that went through the peer runner
	responses  map[string]string
	errs       map[string]error
	attached   int
	attachArgs []string
	// trees answers the worktree lister, keyed by repository.
	trees map[string][]string
}

func newFakeHerdr() *fakeHerdr {
	return &fakeHerdr{responses: map[string]string{}, errs: map[string]error{}}
}

func (f *fakeHerdr) key(args []string) string {
	if len(args) >= 2 && args[0] == "--session" {
		args = args[2:]
	}
	if len(args) >= 2 {
		return args[0] + " " + args[1]
	}
	if len(args) == 1 {
		return args[0]
	}
	return ""
}

func (f *fakeHerdr) run(args []string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, args)
	k := f.key(args)
	if len(args) >= 2 && args[0] == "--session" {
		scoped := "@" + args[1] + " " + k
		if _, ok := f.responses[scoped]; ok {
			k = scoped
		} else if _, ok := f.errs[scoped]; ok {
			k = scoped
		}
	}
	if err, ok := f.errs[k]; ok {
		return []byte(f.responses[k]), err
	}
	return []byte(f.responses[k]), nil
}

// peer is the runner for other sessions: it answers like run, and records
// that the call used the short peer budget.
func (f *fakeHerdr) peer(args []string) ([]byte, error) {
	f.mu.Lock()
	f.peerCalls = append(f.peerCalls, args)
	f.mu.Unlock()
	return f.run(args)
}

func (f *fakeHerdr) worktrees(repo string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.trees[repo], nil
}

func (f *fakeHerdr) backend() *HerdrBackend {
	b := NewHerdr()
	b.run = f.run
	b.peer = f.peer
	b.worktrees = f.worktrees
	b.attach = func(args []string) error { f.attached++; f.attachArgs = args; return nil }
	b.available = func() bool { return true }
	b.env = func(string) string { return "" }
	return b
}

// called reports whether any invocation matched every supplied fragment.
func (f *fakeHerdr) called(fragments ...string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, call := range f.calls {
		joined := strings.Join(call, " ")
		matched := true
		for _, frag := range fragments {
			if !strings.Contains(joined, frag) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// Real herdr 0.9.1 responses, captured against a live server with paths
// rewritten to /repos. Fixtures are the contract grove codes against: an
// invented shape is how two bugs lived undetected — `pane run` prints nothing
// on success, and `pane process-info` nests its process list under
// process_info — because the fakes answered the way grove expected instead of
// the way herdr does.
const (
	herdrOpenedJSON = `{"id":"cli:worktree:open","result":{"already_open":false,"root_pane":{"agent_status":"unknown","cwd":"/repos/grove-testing","focused":false,"foreground_cwd":"/repos/grove-testing","pane_id":"w2:p1","revision":0,"scroll":{"max_offset_from_bottom":0,"offset_from_bottom":0,"viewport_rows":40},"tab_id":"w2:t1","terminal_id":"term_65c537f47d25c8","workspace_id":"w2"},"tab":{"agent_status":"unknown","focused":false,"label":"1","number":1,"pane_count":1,"tab_id":"w2:t1","workspace_id":"w2"},"type":"worktree_opened","workspace":{"active_tab_id":"w2:t1","agent_status":"unknown","focused":false,"label":"grove-testing","number":2,"pane_count":1,"tab_count":1,"workspace_id":"w2","worktree":{"checkout_path":"/repos/grove-testing","is_linked_worktree":true,"repo_key":"/repos/grove/.git","repo_name":"grove","repo_root":"/repos/grove"}},"worktree":{"branch":"testing","is_bare":false,"is_detached":false,"is_linked_worktree":true,"is_prunable":false,"label":"grove","open_workspace_id":"w2","path":"/repos/grove-testing"}}}`

	herdrPaneListJSON = `{"id":"cli:pane:list","result":{"panes":[{"agent_status":"unknown","cwd":"/repos/grove-testing","focused":true,"foreground_cwd":"/repos/grove-testing","pane_id":"w2:p1","revision":0,"scroll":{"max_offset_from_bottom":0,"offset_from_bottom":0,"viewport_rows":40},"tab_id":"w2:t1","terminal_id":"term_65c537f47d25c8","workspace_id":"w2"}],"type":"pane_list"}}`

	herdrProcessInfoBashJSON = `{"id":"cli:pane:process_info","result":{"process_info":{"foreground_process_group_id":14839,"foreground_processes":[{"argv":["/bin/bash"],"cmdline":"/bin/bash","cwd":"/repos/grove-testing","name":"bash","pid":14839}],"pane_id":"w2:p1","shell_pid":14839},"type":"pane_process_info"}}`

	// `pane run` success: exit 0, no output at all.
	herdrPaneRunOK = ``

	herdrCloseOKJSON = `{"id":"cli:workspace:close","result":{"type":"ok"}}`

	herdrGroupCloseRequiredJSON = `{"error":{"code":"workspace_group_close_required","message":"workspace has linked worktree workspaces; use --group (close_group=true in the API) to close the group"},"id":"cli:workspace:close"}`

	herdrProtocolMismatchJSON = `{"id":"cli:workspace:list","error":{"code":"protocol_mismatch","message":"client protocol 19 is older than server protocol 22; upgrade the Herdr client before using this command"}}`
)

const workspaceListJSON = `{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[{"workspace_id":"w1","number":1,"label":"grove-main","focused":true,"pane_count":2,"tab_count":1,"active_tab_id":"w1:t1","agent_status":"working","worktree":{"repo_key":"k","repo_name":"grove","repo_root":"/repos/grove","checkout_path":"/repos/grove","is_linked_worktree":false}},{"workspace_id":"w2","number":2,"label":"grove-testing","focused":false,"pane_count":1,"tab_count":1,"active_tab_id":"w2:t1","agent_status":"blocked","worktree":{"repo_key":"k","repo_name":"grove","repo_root":"/repos/grove","checkout_path":"/repos/grove-testing","is_linked_worktree":true}},{"workspace_id":"w3","number":3,"label":"scratch","focused":false,"pane_count":1,"tab_count":1,"active_tab_id":"w3:t1","agent_status":"idle"}]}}`

func TestHerdrListDecodesWorkspaces(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON

	sessions, err := f.backend().List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("List() returned %d sessions, want 3", len(sessions))
	}

	main := sessions[0]
	if main.ID != "w1" || main.Name != "grove-main" {
		t.Errorf("session 0 = %+v, want id w1 / label grove-main", main)
	}
	if main.Path != "/repos/grove" {
		t.Errorf("Path = %q, want the worktree checkout_path", main.Path)
	}
	// herdr's `focused` is one server-wide value that stays set with no
	// client attached, so it maps to active/open, never attached/detached.
	if main.Status != StatusActive {
		t.Errorf("focused workspace Status = %q, want %q", main.Status, StatusActive)
	}
	if main.Agent != AgentWorking {
		t.Errorf("Agent = %q, want %q", main.Agent, AgentWorking)
	}
	if main.Windows != 2 {
		t.Errorf("Windows = %d, want pane_count 2", main.Windows)
	}

	if sessions[1].Status != StatusOpen {
		t.Errorf("unfocused workspace Status = %q, want %q", sessions[1].Status, StatusOpen)
	}
	if sessions[1].Agent != AgentBlocked {
		t.Errorf("Agent = %q, want %q", sessions[1].Agent, AgentBlocked)
	}

	// A workspace with no git provenance has no checkout path; it must still
	// decode rather than breaking the whole listing.
	if sessions[2].Path != "" {
		t.Errorf("non-worktree workspace Path = %q, want empty", sessions[2].Path)
	}
}

func TestHerdrListSurfacesErrorEnvelope(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = `{"id":"x","error":{"code":"server_not_running","message":"no herdr server is running"}}`
	f.errs["workspace list"] = errors.New("exit status 1")

	_, err := f.backend().List()
	if err == nil {
		t.Fatal("List() expected an error")
	}
	if !ErrServerNotRunning(err) {
		t.Errorf("ErrServerNotRunning(%v) = false, want true", err)
	}
	if !strings.Contains(err.Error(), "no herdr server is running") {
		t.Errorf("error %q should carry herdr's message", err)
	}
}

func TestHerdrListIgnoresUnknownFields(t *testing.T) {
	// herdr ships fast; a new field must not break decoding.
	f := newFakeHerdr()
	f.responses["workspace list"] = `{"id":"x","result":{"type":"workspace_list","brand_new":42,"workspaces":[{"workspace_id":"w1","number":1,"label":"grove-main","focused":false,"pane_count":1,"tab_count":1,"active_tab_id":"w1:t1","agent_status":"idle","future_field":{"nested":true}}]}}`

	sessions, err := f.backend().List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "w1" {
		t.Errorf("List() = %+v, want one session w1", sessions)
	}
}

func TestHerdrListMapsUnrecognizedAgentStatus(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = `{"id":"x","result":{"type":"workspace_list","workspaces":[{"workspace_id":"w1","number":1,"label":"a","focused":false,"pane_count":1,"tab_count":1,"active_tab_id":"w1:t1","agent_status":"reticulating"}]}}`

	sessions, err := f.backend().List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if sessions[0].Agent != AgentUnknown {
		t.Errorf("unrecognized agent_status = %q, want %q", sessions[0].Agent, AgentUnknown)
	}
}

func TestHerdrEnsureAdoptsExistingCheckout(t *testing.T) {
	f := newFakeHerdr()
	f.responses["worktree open"] = `{"id":"x","result":{"type":"worktree_opened","workspace":{"workspace_id":"w2","number":2,"label":"grove-testing","focused":false,"pane_count":1,"tab_count":1,"active_tab_id":"w2:t1","agent_status":"idle"},"already_open":false}}`

	err := f.backend().Ensure(Target{Name: "grove-testing", Path: "/repos/grove-testing", Repo: "/repos/grove"})
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}

	if !f.called("worktree", "open", "--path", "/repos/grove-testing", "--label", "grove-testing") {
		t.Errorf("Ensure did not adopt via worktree open; calls: %v", f.calls)
	}
	// herdr resolves the *source repo* from --cwd and rejects a linked
	// worktree there ("New and open worktree actions start from the repo
	// parent workspace"), so this must be the main checkout, not the target.
	if !f.called("--cwd", "/repos/grove") {
		t.Errorf("Ensure did not pass the repo root as --cwd; calls: %v", f.calls)
	}
	if !f.called("--no-focus") {
		t.Error("Ensure must not steal focus")
	}
	// Grove owns worktree lifecycle: herdr must never be asked to create one.
	if f.called("worktree", "create") {
		t.Error("Ensure called `worktree create` — grove owns checkout creation")
	}
}

func TestHerdrEnsureIsIdempotent(t *testing.T) {
	f := newFakeHerdr()
	f.responses["worktree open"] = `{"id":"x","result":{"type":"worktree_opened","workspace":{"workspace_id":"w2","number":2,"label":"grove-testing","focused":false,"pane_count":1,"tab_count":1,"active_tab_id":"w2:t1","agent_status":"idle"},"already_open":true}}`

	if err := f.backend().Ensure(Target{Name: "grove-testing", Path: "/repos/grove-testing", Repo: "/repos/grove"}); err != nil {
		t.Fatalf("Ensure() on an already-open workspace error = %v", err)
	}
}

// unopenableRefusal is herdr's answer to `worktree open` on a path its git
// worktree list does not contain (captured from herdr 0.9.1).
func unopenableRefusal(f *fakeHerdr) {
	f.responses["worktree open"] = `{"error":{"code":"worktree_not_found","message":"worktree path not found"},"id":"cli:worktree:open"}`
	f.errs["worktree open"] = errors.New("exit status 1")
}

func TestHerdrNeverCreatesAWorkspace(t *testing.T) {
	// A path herdr will not open as a worktree has nothing herdr would group
	// or track it under, so grove must not invent a workspace for it: Ensure
	// reports the target unmanaged and the caller falls through to a plain
	// directory switch.
	f := newFakeHerdr()
	unopenableRefusal(f)
	f.responses["workspace list"] = `{"id":"x","result":{"type":"workspace_list","workspaces":[]}}`

	err := f.backend().Ensure(Target{Name: "grove-stray", Path: "/repos/stray", Repo: "/repos/grove"})
	if !ErrUnmanaged(err) {
		t.Fatalf("Ensure() error = %v, want an unmanaged-target error", err)
	}
	if f.called("workspace", "create") {
		t.Errorf("Ensure created a workspace for a path herdr would not open; calls: %v", f.calls)
	}
	if DegradedHint(err) != "" {
		t.Errorf("DegradedHint(%v) = %q, want silence for an unopenable path", err, DegradedHint(err))
	}
}

func TestHerdrEnsureAdoptsAWorkspaceCoveringAnUnopenablePath(t *testing.T) {
	// A workspace herdr already has for the path — opened by hand, say — is
	// still the right place to land, even though herdr would not open the
	// path as a worktree now.
	checkout := t.TempDir()
	f := newFakeHerdr()
	unopenableRefusal(f)
	f.responses["workspace list"] = fmt.Sprintf(
		`{"id":"x","result":{"type":"workspace_list","workspaces":[`+
			`{"workspace_id":"w1","number":1,"label":"grove","focused":false,`+
			`"pane_count":1,"tab_count":1,"active_tab_id":"w1:t1","agent_status":"idle",`+
			`"worktree":{"repo_key":"k","repo_name":"grove","repo_root":%q,`+
			`"checkout_path":%q,"is_linked_worktree":false}}]}}`, checkout, checkout)

	if err := f.backend().Ensure(Target{Name: "grove", Path: checkout, Repo: checkout}); err != nil {
		t.Fatalf("Ensure() error = %v, want nil for an already-present workspace", err)
	}
	if f.called("workspace", "create") {
		t.Errorf("Ensure created a workspace instead of adopting; calls: %v", f.calls)
	}
}

func TestHerdrEnsureOpensTheRepositoryCheckoutAsTheParentWorkspace(t *testing.T) {
	// herdr answers `worktree open --path <repo>` by adopting — or creating —
	// the repository's parent workspace, the one its sidebar groups the
	// project's worktrees under (verified on herdr 0.8.0 and 0.9.1). That is
	// herdr's own bookkeeping; grove just calls the same verb it uses for
	// every worktree, and still never `workspace create`.
	f := newFakeHerdr()
	f.responses["worktree open"] = herdrOpenedJSON

	if err := f.backend().Ensure(Target{Name: "grove", Path: "/repos/grove", Repo: "/repos/grove"}); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if !f.called("worktree", "open", "--cwd", "/repos/grove", "--path", "/repos/grove") {
		t.Errorf("Ensure did not open the repository checkout; calls: %v", f.calls)
	}
	if f.called("workspace", "create") {
		t.Errorf("Ensure called `workspace create`; calls: %v", f.calls)
	}
}

func TestHerdrEnsureDoesNotFallBackOnUnrelatedErrors(t *testing.T) {
	f := newFakeHerdr()
	f.responses["worktree open"] = `{"id":"x","error":{"code":"server_not_running","message":"nope"}}`
	f.errs["worktree open"] = errors.New("exit status 1")

	if err := f.backend().Ensure(Target{Name: "grove", Path: "/repos/grove", Repo: "/repos/grove"}); err == nil {
		t.Fatal("Ensure() expected the server error to propagate")
	}
	if f.called("workspace", "create") {
		t.Error("Ensure fell back to workspace create on an unrelated failure")
	}
}

func TestHerdrExistsKeysOnPath(t *testing.T) {
	// Exists resolves through Index, which ignores checkout paths that are not
	// on disk — so this fixture needs a checkout that really exists.
	checkout := t.TempDir()
	f := newFakeHerdr()
	f.responses["workspace list"] = fmt.Sprintf(
		`{"id":"x","result":{"type":"workspace_list","workspaces":[`+
			`{"workspace_id":"w2","number":2,"label":"grove-testing","focused":false,`+
			`"pane_count":1,"tab_count":1,"active_tab_id":"w2:t1","agent_status":"blocked",`+
			`"worktree":{"repo_key":"k","repo_name":"grove","repo_root":"/repos/grove",`+
			`"checkout_path":%q,"is_linked_worktree":true}}]}}`, checkout)
	b := f.backend()

	// The label here deliberately disagrees with herdr's, proving path keying.
	ok, err := b.Exists(Target{Name: "totally-different", Path: checkout})
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !ok {
		t.Error("Exists() = false, want true for a known checkout path")
	}

	ok, err = b.Exists(Target{Name: "grove-nope", Path: filepath.Join(t.TempDir(), "nope")})
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if ok {
		t.Error("Exists() = true for an unknown checkout path")
	}
}

func TestHerdrKillClosesWorkspaceNotWorktree(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON
	f.responses["workspace close"] = herdrCloseOKJSON

	if err := f.backend().Kill(Target{Name: "grove-testing", Path: "/repos/grove-testing"}); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}
	if !f.called("workspace", "close", "w2") {
		t.Errorf("Kill did not close workspace w2; calls: %v", f.calls)
	}
	// `worktree remove` runs `git worktree remove`, bypassing grove's
	// protection rules. It must never be reachable from Kill.
	if f.called("worktree", "remove") {
		t.Error("Kill called `worktree remove` — that would delete the checkout")
	}
}

func TestHerdrKillOnMissingSessionIsNoOp(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON

	if err := f.backend().Kill(Target{Name: "gone", Path: "/repos/gone"}); err != nil {
		t.Fatalf("Kill() on an unknown target should be a no-op, got %v", err)
	}
	if f.called("workspace", "close") {
		t.Error("Kill closed a workspace for an unknown target")
	}
}

func TestHerdrRenameRelabelsWorkspace(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON
	f.responses["workspace rename"] = `{"id":"x","result":{"type":"workspace_info","workspace":{"workspace_id":"w2","number":2,"label":"grove-renamed","focused":false,"pane_count":1,"tab_count":1,"active_tab_id":"w2:t1","agent_status":"idle"}}}`

	err := f.backend().Rename(
		Target{Name: "grove-testing", Path: "/repos/grove-testing"},
		Target{Name: "grove-renamed", Path: "/repos/grove-renamed"},
	)
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if !f.called("workspace", "rename", "w2", "grove-renamed") {
		t.Errorf("Rename did not relabel w2; calls: %v", f.calls)
	}
}

func TestHerdrSwitchFocusesWorkspace(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON
	f.responses["workspace focus"] = `{"id":"x","result":{"type":"workspace_info","workspace":{"workspace_id":"w2","number":2,"label":"grove-testing","focused":true,"pane_count":1,"tab_count":1,"active_tab_id":"w2:t1","agent_status":"idle"}}}`

	if err := f.backend().Switch(Target{Name: "grove-testing", Path: "/repos/grove-testing"}); err != nil {
		t.Fatalf("Switch() error = %v", err)
	}
	if !f.called("workspace", "focus", "w2") {
		t.Errorf("Switch did not focus w2; calls: %v", f.calls)
	}
}

func TestHerdrSwitchOnUnknownTargetErrors(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON

	err := f.backend().Switch(Target{Name: "gone", Path: "/repos/gone"})
	if err == nil {
		t.Fatal("Switch() to an unknown target should error")
	}
	if !ErrNoSession(err) {
		t.Errorf("ErrNoSession(%v) = false, want true", err)
	}
}

func TestHerdrAttachFocusesBeforeAttaching(t *testing.T) {
	// herdr has no `--workspace` launch flag, so landing on a specific
	// worktree is focus-then-attach. Order matters: attaching first would
	// drop the user on whatever workspace was last focused.
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON
	f.responses["workspace focus"] = `{"id":"x","result":{"type":"workspace_info","workspace":{"workspace_id":"w2","number":2,"label":"grove-testing","focused":true,"pane_count":1,"tab_count":1,"active_tab_id":"w2:t1","agent_status":"idle"}}}`

	if err := f.backend().Attach(Target{Name: "grove-testing", Path: "/repos/grove-testing"}); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	if !f.called("workspace", "focus", "w2") {
		t.Fatalf("Attach did not focus first; calls: %v", f.calls)
	}
	if f.attached != 1 {
		t.Errorf("attach invoked %d times, want 1", f.attached)
	}
}

func TestHerdrInsideReadsHerdrEnv(t *testing.T) {
	b := NewHerdr()
	b.env = func(key string) string {
		if key == "HERDR_ENV" {
			return "1"
		}
		return ""
	}
	if !b.Inside() {
		t.Error("Inside() = false with HERDR_ENV=1")
	}

	b.env = func(string) string { return "" }
	if b.Inside() {
		t.Error("Inside() = true with HERDR_ENV unset")
	}
}

func TestHerdrCurrentUsesWorkspaceEnv(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace get"] = `{"id":"x","result":{"type":"workspace_info","workspace":{"workspace_id":"w2","number":2,"label":"grove-testing","focused":true,"pane_count":1,"tab_count":1,"active_tab_id":"w2:t1","agent_status":"idle"}}}`
	b := f.backend()
	b.env = func(key string) string {
		if key == "HERDR_WORKSPACE_ID" {
			return "w2"
		}
		return ""
	}

	got, err := b.Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if got != "grove-testing" {
		t.Errorf("Current() = %q, want the workspace label", got)
	}
}

func TestHerdrCurrentOutsideHerdrErrors(t *testing.T) {
	b := f0().backend()
	b.env = func(string) string { return "" }

	if _, err := b.Current(); err == nil {
		t.Error("Current() outside herdr should error")
	}
}

func f0() *fakeHerdr { return newFakeHerdr() }

func TestHerdrDoesNotImplementPopup(t *testing.T) {
	// Popup placement is only reachable through herdr's plugin pane surface,
	// so the backend must not claim the capability — callers fall back to a
	// full attach instead of silently doing nothing.
	var m Multiplexer = NewHerdr()
	if _, ok := m.(Popuper); ok {
		t.Error("HerdrBackend claims Popuper; it has no display-popup equivalent")
	}
	if _, ok := m.(ControlModer); ok {
		t.Error("HerdrBackend claims ControlModer; tmux -CC has no herdr equivalent")
	}
}

func TestHerdrIgnoresNonEnvelopeOutput(t *testing.T) {
	// grove reads stdout and stderr together, so an update notice or log line
	// can share the stream with the response.
	f := newFakeHerdr()
	f.responses["workspace list"] = "a new herdr release is available\n" +
		`{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[{"workspace_id":"w1","number":1,"label":"grove-main","focused":false,"pane_count":1,"tab_count":1,"active_tab_id":"w1:t1","agent_status":"idle"}]}}` +
		"\n"

	sessions, err := f.backend().List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "w1" {
		t.Errorf("List() = %+v, want one session w1", sessions)
	}
}

func TestHerdrIgnoresUnrelatedJSONLines(t *testing.T) {
	// A JSON log line lacking id/result must not be mistaken for a response.
	f := newFakeHerdr()
	f.responses["workspace list"] = `{"level":"warn","msg":"slow socket"}` + "\n" +
		`{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[]}}`

	sessions, err := f.backend().List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("List() = %+v, want no sessions", sessions)
	}
}

func TestHerdrGarbageOutputIsAnError(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = "herdr: command not found"

	if _, err := f.backend().List(); err == nil {
		t.Fatal("List() expected an error for unparseable output")
	}
}

func TestHerdrExitFailureWithoutEnvelopeIsAnError(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = ""
	f.errs["workspace list"] = errors.New("exit status 2")

	_, err := f.backend().List()
	if err == nil {
		t.Fatal("List() expected an error")
	}
	if !strings.Contains(err.Error(), "exit status 2") {
		t.Errorf("error %q should carry the exit status", err)
	}
}

func TestHerdrEnsureOmitsLabelWhenUnset(t *testing.T) {
	f := newFakeHerdr()
	f.responses["worktree open"] = `{"id":"x","result":{"type":"worktree_opened","already_open":false}}`

	if err := f.backend().Ensure(Target{Path: "/repos/grove", Repo: "/repos/grove"}); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if f.called("--label") {
		t.Error("Ensure passed an empty --label")
	}
}

func TestHerdrPaneInfoTreatsAgentPaneAsNonShell(t *testing.T) {
	// Sending `cd` into a pane running a coding agent would type into the
	// agent's prompt, so drift correction must skip it.
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON
	f.responses["pane list"] = `{"id":"x","result":{"type":"pane_list","panes":[{"pane_id":"w2:p1","terminal_id":"t","workspace_id":"w2","tab_id":"w2:t1","focused":true,"cwd":"/elsewhere","agent":"claude","agent_status":"working"}]}}`

	info, err := f.backend().PaneInfo(Target{Name: "grove-testing", Path: "/repos/grove-testing"})
	if err != nil {
		t.Fatalf("PaneInfo() error = %v", err)
	}
	if !info.HasAgent {
		t.Error("HasAgent = false, want true when herdr reports an agent")
	}
	if info.IsShell() {
		t.Error("IsShell() = true for a pane occupied by an agent")
	}
	if info.CurrentPath != "/elsewhere" {
		t.Errorf("CurrentPath = %q, want /elsewhere", info.CurrentPath)
	}
}

func TestHerdrPaneInfoNamesForegroundShell(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON
	f.responses["pane list"] = herdrPaneListJSON
	f.responses["pane process-info"] = herdrProcessInfoBashJSON

	info, err := f.backend().PaneInfo(Target{Name: "grove-testing", Path: "/repos/grove-testing"})
	if err != nil {
		t.Fatalf("PaneInfo() error = %v", err)
	}
	// Reading foreground_processes from the top level instead of under
	// process_info decodes cleanly into an empty list, so this is the check
	// that would have caught drift correction being silently dead.
	if info.CurrentCommand != "bash" {
		t.Errorf("CurrentCommand = %q, want bash from process_info.foreground_processes", info.CurrentCommand)
	}
	if !info.IsShell() {
		t.Errorf("IsShell() = false for a bash pane (command=%q)", info.CurrentCommand)
	}
}

func TestHerdrPaneInfoPrefersForegroundCwd(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON
	f.responses["pane list"] = `{"id":"x","result":{"type":"pane_list","panes":[{"pane_id":"w2:p1","terminal_id":"t","workspace_id":"w2","tab_id":"w2:t1","focused":true,"cwd":"/stale","foreground_cwd":"/live","agent_status":"unknown"}]}}`
	f.responses["pane process-info"] = herdrProcessInfoBashJSON

	info, err := f.backend().PaneInfo(Target{Name: "grove-testing", Path: "/repos/grove-testing"})
	if err != nil {
		t.Fatalf("PaneInfo() error = %v", err)
	}
	if info.CurrentPath != "/live" {
		t.Errorf("CurrentPath = %q, want the foreground cwd", info.CurrentPath)
	}
}

func TestHerdrEnsureRequiresRepoRoot(t *testing.T) {
	// Without a source repo, herdr can only infer one from the focused
	// workspace — which does not exist when grove runs outside a herdr client.
	// Rather than emit a call that fails at runtime, say so up front.
	f := newFakeHerdr()

	err := f.backend().Ensure(Target{Name: "grove-testing", Path: "/repos/grove-testing"})
	if err == nil {
		t.Fatal("Ensure() without a repo root should error")
	}
	if f.called("worktree", "open") {
		t.Error("Ensure issued a worktree open it knew would fail")
	}
}

func TestHerdrEnsureUsesRepoRootNotCheckoutForSource(t *testing.T) {
	f := newFakeHerdr()
	f.responses["worktree open"] = `{"id":"x","result":{"type":"worktree_opened","already_open":false}}`

	if err := f.backend().Ensure(Target{Name: "grove", Path: "/repos/grove", Repo: "/repos/grove"}); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	// The main checkout is its own repo root; herdr opens it as the parent
	// workspace rather than erroring.
	if !f.called("--cwd", "/repos/grove", "--path", "/repos/grove") {
		t.Errorf("calls: %v", f.calls)
	}
}

func TestHerdrEnsureNamesTheTab(t *testing.T) {
	// herdr labels a new tab with its number, so nothing ever titles the
	// window and the terminal falls back to naming it after the launching
	// process. Ensure must set the worktree's name on it.
	f := newFakeHerdr()
	f.responses["worktree open"] = `{"id":"x","result":{"type":"worktree_opened","already_open":false,` +
		`"root_pane":{"pane_id":"w1:p1","tab_id":"w1:t1","workspace_id":"w1"},` +
		`"tab":{"tab_id":"w1:t1","label":"1","number":1,"workspace_id":"w1"}}}`

	target := Target{Name: "grove-testing", Short: "testing", Path: "/repos/grove-testing", Repo: "/repos/grove"}
	if err := f.backend().Ensure(target); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if !f.called("tab", "rename", "w1:t1", "testing") {
		t.Errorf("Ensure did not name the tab after the worktree; calls: %v", f.calls)
	}
}

func TestHerdrEnsureKeepsAUserChosenTabLabel(t *testing.T) {
	// A label that is not herdr's generated number was chosen by someone.
	// Overwriting it on every switch would make the tab unusable as a manual
	// marker.
	f := newFakeHerdr()
	f.responses["worktree open"] = `{"id":"x","result":{"type":"worktree_opened","already_open":true,` +
		`"root_pane":{"pane_id":"w1:p1","tab_id":"w1:t1","workspace_id":"w1"},` +
		`"tab":{"tab_id":"w1:t1","label":"my notes","number":1,"workspace_id":"w1"}}}`

	target := Target{Name: "grove-testing", Short: "testing", Path: "/repos/grove-testing", Repo: "/repos/grove"}
	if err := f.backend().Ensure(target); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if f.called("tab", "rename") {
		t.Errorf("Ensure overwrote a user-chosen tab label; calls: %v", f.calls)
	}
}

func TestTargetDisplayNameFallsBackToSessionName(t *testing.T) {
	// Not every call site knows the short name; the canonical session name is
	// still a better window title than the launching process.
	if got := (Target{Name: "grove-testing"}).DisplayName(); got != "grove-testing" {
		t.Errorf("DisplayName() = %q, want the session name", got)
	}
	if got := (Target{Name: "grove-testing", Short: "testing"}).DisplayName(); got != "testing" {
		t.Errorf("DisplayName() = %q, want the short name", got)
	}
}

func TestHerdrNotify(t *testing.T) {
	f := newFakeHerdr()
	f.responses["notification show"] = `{"id":"x","result":{"type":"notification_show","shown":true,"reason":"shown"}}`

	reason, err := f.backend().Notify("grove: worktree not tracked", "run grove adopt")
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if reason != "shown" {
		t.Errorf("reason = %q, want %q", reason, "shown")
	}
	if !f.called("notification", "show", "grove: worktree not tracked", "--body", "run grove adopt") {
		t.Errorf("Notify did not pass title and body; calls: %v", f.calls)
	}
}

func TestHerdrNotifyOmitsAnEmptyBody(t *testing.T) {
	f := newFakeHerdr()
	f.responses["notification show"] = `{"id":"x","result":{"type":"notification_show","shown":true}}`

	if _, err := f.backend().Notify("title only", ""); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if f.called("--body") {
		t.Errorf("Notify passed an empty --body; calls: %v", f.calls)
	}
}

func TestHerdrNotifyReportsSuppression(t *testing.T) {
	// Delivery is the user's `[ui.toast] delivery` setting and herdr's rate
	// limiter, neither of which a plugin can override. A suppressed
	// notification is not an error — but the caller has to be able to tell,
	// or an adoption prompt that silently never appeared is undebuggable.
	f := newFakeHerdr()
	f.responses["notification show"] = `{"id":"x","result":{"type":"notification_show","shown":false,"reason":"disabled"}}`

	reason, err := f.backend().Notify("t", "b")
	if err != nil {
		t.Fatalf("Notify() error = %v, want suppression reported as a reason not an error", err)
	}
	if reason != "disabled" {
		t.Errorf("reason = %q, want %q", reason, "disabled")
	}
}

// A herdr binary with no live server must degrade, not fail: doctor promises
// "grove will fall back to plain directory switching", and treating a dead
// server as a fatal Exists/Ensure error made every `grove to` abort before
// the cd directive. Exists answers "no session"; Ensure reports the target
// unmanaged, the same shape callers already degrade on.
func TestHerdrExistsWithServerDownReportsNoSession(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = `{"id":"x","error":{"code":"server_not_running","message":"no herdr server is running"}}`

	exists, err := f.backend().Exists(Target{Name: "grove-x", Path: "/repos/grove-x"})
	if err != nil {
		t.Fatalf("Exists() error = %v, want nil for a dead server", err)
	}
	if exists {
		t.Error("Exists() = true, want false for a dead server")
	}
}

func TestHerdrEnsureWithServerDownIsUnmanaged(t *testing.T) {
	f := newFakeHerdr()
	f.responses["worktree open"] = `{"id":"x","error":{"code":"server_not_running","message":"no herdr server is running"}}`

	err := f.backend().Ensure(Target{Name: "grove-x", Path: "/repos/grove-x", Repo: "/repos/grove"})
	if !ErrUnmanaged(err) {
		t.Fatalf("Ensure() error = %v, want ErrUnmanaged for a dead server", err)
	}
}

func TestHerdrEnsureWithCommandWithServerDownIsUnmanaged(t *testing.T) {
	f := newFakeHerdr()
	f.responses["worktree open"] = `{"id":"x","error":{"code":"server_not_running","message":"no herdr server is running"}}`

	err := f.backend().EnsureWithCommand(Target{Name: "grove-x", Path: "/repos/grove-x", Repo: "/repos/grove"}, "npm run dev")
	if !ErrUnmanaged(err) {
		t.Fatalf("EnsureWithCommand() error = %v, want ErrUnmanaged for a dead server", err)
	}
}

// List must keep surfacing the raw server_not_running error — doctor's server
// check depends on recognizing it.
func TestHerdrListWithServerDownSurfacesError(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = `{"id":"x","error":{"code":"server_not_running","message":"no herdr server is running"}}`

	_, err := f.backend().List()
	if !ErrServerNotRunning(err) {
		t.Fatalf("List() error = %v, want ErrServerNotRunning", err)
	}
}

// countCalls returns how many recorded invocations match the given key
// (command group + subcommand).
func (f *fakeHerdr) countCalls(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, call := range f.calls {
		if f.key(call) == key {
			n++
		}
	}
	return n
}

// One grove command used to spawn `workspace list` four times (Exists, then a
// fresh resolve inside PaneInfo, SendCommand, and focus) and `pane list`
// twice, against the <500ms budget. Reads are cached for the instance's
// lifetime.
func TestHerdrCachesReadsWithinAnInstance(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON
	f.responses["workspace focus"] = `{"id":"x","result":{"type":"ok"}}`
	f.responses["pane list"] = herdrPaneListJSON
	f.responses["pane process-info"] = herdrProcessInfoBashJSON
	f.responses["pane run"] = herdrPaneRunOK
	b := f.backend()
	target := Target{Name: "grove-testing", Path: "/repos/grove-testing"}

	// The `grove to` shape: exists check, drift check, drift correction, focus.
	if exists, err := b.Exists(target); err != nil || !exists {
		t.Fatalf("Exists() = %v, %v", exists, err)
	}
	if _, err := b.PaneInfo(target); err != nil {
		t.Fatalf("PaneInfo() error = %v", err)
	}
	if err := b.SendCommand(target, "cd /repos/grove-testing"); err != nil {
		t.Fatalf("SendCommand() error = %v", err)
	}
	if err := b.Switch(target); err != nil {
		t.Fatalf("Switch() error = %v", err)
	}

	if got := f.countCalls("workspace list"); got != 1 {
		t.Errorf("workspace list spawned %d times, want 1; calls: %v", got, f.calls)
	}
	if got := f.countCalls("pane list"); got != 1 {
		t.Errorf("pane list spawned %d times, want 1; calls: %v", got, f.calls)
	}
}

// Mutations must drop the caches: a Kill changes the workspace set, so the
// next Exists has to refetch rather than answer from the stale snapshot.
func TestHerdrMutationInvalidatesReadCache(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON
	f.responses["workspace close"] = herdrCloseOKJSON
	b := f.backend()
	target := Target{Name: "grove-testing", Path: "/repos/grove-testing"}

	if _, err := b.Exists(target); err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if err := b.Kill(target); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}
	if _, err := b.Exists(target); err != nil {
		t.Fatalf("Exists() after Kill error = %v", err)
	}

	if got := f.countCalls("workspace list"); got != 2 {
		t.Errorf("workspace list spawned %d times, want 2 (cache must drop on close); calls: %v", got, f.calls)
	}
}

// `pane run` prints nothing on success. Treating that silence as "unexpected
// output" made SendCommand report failure after the command had already run.
func TestHerdrSendCommandAcceptsSilentSuccess(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON
	f.responses["pane list"] = herdrPaneListJSON
	f.responses["pane run"] = herdrPaneRunOK

	if err := f.backend().SendCommand(Target{Name: "grove-testing", Path: "/repos/grove-testing"}, "npm run dev"); err != nil {
		t.Fatalf("SendCommand() error = %v, want nil for herdr's silent success", err)
	}
	if !f.called("pane", "run", "w2:p1", "npm run dev") {
		t.Errorf("SendCommand did not run in the workspace's pane; calls: %v", f.calls)
	}
}

// The `grove open` + `[session] command` path: a new workspace, then the
// command in its root pane. This failed end to end — the command ran, and
// grove then reported "failed to create session".
func TestHerdrEnsureWithCommandRunsInTheNewPane(t *testing.T) {
	f := newFakeHerdr()
	f.responses["worktree open"] = herdrOpenedJSON
	f.responses["pane run"] = herdrPaneRunOK

	target := Target{Name: "grove-testing", Short: "testing", Path: "/repos/grove-testing", Repo: "/repos/grove"}
	if err := f.backend().EnsureWithCommand(target, "claude"); err != nil {
		t.Fatalf("EnsureWithCommand() error = %v", err)
	}
	if !f.called("pane", "run", "w2:p1", "claude") {
		t.Errorf("EnsureWithCommand did not run the command in the root pane; calls: %v", f.calls)
	}
}

// A silent success is only acceptable where no result is expected. A read
// that comes back empty must still fail rather than look like "no sessions".
func TestHerdrEmptyReadIsAnError(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = ""

	if _, err := f.backend().List(); err == nil {
		t.Fatal("List() on empty output should error")
	}
}

// After a herdr upgrade the old server keeps running but refuses every call
// from the new CLI until it is restarted. That must degrade like a stopped
// server — `grove to` falling back to a plain cd — and, unlike a stopped
// server, say why.
func TestHerdrProtocolMismatchDegradesWithAHint(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = herdrProtocolMismatchJSON
	f.errs["workspace list"] = errors.New("exit status 1")
	f.responses["worktree open"] = strings.Replace(herdrProtocolMismatchJSON, "workspace:list", "worktree:open", 1)
	f.errs["worktree open"] = errors.New("exit status 1")
	b := f.backend()
	target := Target{Name: "grove-testing", Path: "/repos/grove-testing", Repo: "/repos/grove"}

	exists, err := b.Exists(target)
	if err != nil || exists {
		t.Fatalf("Exists() = %v, %v; want false, nil on a protocol mismatch", exists, err)
	}

	err = b.Ensure(target)
	if !ErrUnmanaged(err) {
		t.Fatalf("Ensure() error = %v, want ErrUnmanaged on a protocol mismatch", err)
	}
	if !ErrProtocolMismatch(err) {
		t.Errorf("ErrProtocolMismatch(%v) = false; the cause must stay in the chain", err)
	}
	hint := DegradedHint(err)
	if !strings.Contains(hint, "client protocol 19 is older than server protocol 22") {
		t.Errorf("DegradedHint() = %q, want herdr's own explanation", hint)
	}

	if err := b.EnsureWithCommand(target, "claude"); !ErrUnmanaged(err) {
		t.Errorf("EnsureWithCommand() error = %v, want ErrUnmanaged on a protocol mismatch", err)
	}
}

// A stopped server is the expected, documented fallback — no hint.
func TestHerdrStoppedServerDegradesSilently(t *testing.T) {
	f := newFakeHerdr()
	f.responses["worktree open"] = `{"id":"cli:worktree:open","error":{"code":"server_not_running","message":"no herdr server is running"}}`
	f.errs["worktree open"] = errors.New("exit status 1")

	err := f.backend().Ensure(Target{Name: "grove-x", Path: "/repos/grove-x", Repo: "/repos/grove"})
	if !ErrUnmanaged(err) {
		t.Fatalf("Ensure() error = %v, want ErrUnmanaged", err)
	}
	if hint := DegradedHint(err); hint != "" {
		t.Errorf("DegradedHint() = %q, want silence for a stopped server", hint)
	}
}

// herdr turns worktree discovery away, rather than queueing it, when its
// background slots are all taken. One retry rides out the burst.
func TestHerdrEnsureRetriesWorktreeBusyOnce(t *testing.T) {
	defer func(d time.Duration) { herdrBusyRetryDelay = d }(herdrBusyRetryDelay)
	herdrBusyRetryDelay = 0

	f := newFakeHerdr()
	b := f.backend()
	opens := 0
	b.run = func(args []string) ([]byte, error) {
		f.calls = append(f.calls, args)
		if f.key(args) != "worktree open" {
			return []byte(`{"id":"x","result":{"type":"tab_info"}}`), nil
		}
		opens++
		if opens == 1 {
			return []byte(`{"error":{"code":"worktree_busy","message":"too many worktree checks are pending; retry shortly"},"id":"cli:worktree:open"}`), errors.New("exit status 1")
		}
		return []byte(herdrOpenedJSON), nil
	}

	if err := b.Ensure(Target{Name: "grove-testing", Path: "/repos/grove-testing", Repo: "/repos/grove"}); err != nil {
		t.Fatalf("Ensure() error = %v, want the retry to succeed", err)
	}
	if opens != 2 {
		t.Errorf("worktree open ran %d times, want 2", opens)
	}
}

func TestHerdrEnsureGivesUpAfterOneBusyRetry(t *testing.T) {
	defer func(d time.Duration) { herdrBusyRetryDelay = d }(herdrBusyRetryDelay)
	herdrBusyRetryDelay = 0

	f := newFakeHerdr()
	f.responses["worktree open"] = `{"error":{"code":"worktree_busy","message":"too many worktree checks are pending; retry shortly"},"id":"cli:worktree:open"}`
	f.errs["worktree open"] = errors.New("exit status 1")

	err := f.backend().Ensure(Target{Name: "grove-testing", Path: "/repos/grove-testing", Repo: "/repos/grove"})
	if err == nil {
		t.Fatal("Ensure() should fail when herdr stays busy")
	}
	if got := f.countCalls("worktree open"); got != 2 {
		t.Errorf("worktree open ran %d times, want exactly 2", got)
	}
}

// Since herdr 0.9.0 closing a parent workspace with worktree workspaces under
// it needs --group, which would close all of them. grove must never pass it,
// and must say what is in the way.
func TestHerdrKillExplainsAGroupCloseRefusal(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON
	f.responses["workspace close"] = herdrGroupCloseRequiredJSON
	f.errs["workspace close"] = errors.New("exit status 1")

	err := f.backend().Kill(Target{Name: "grove-main", Path: "/repos/grove"})
	if err == nil || !strings.Contains(err.Error(), "worktree workspaces open") {
		t.Fatalf("Kill() error = %v, want an explanation of the group refusal", err)
	}
	if f.called("--group") {
		t.Errorf("Kill passed --group, which closes every worktree workspace; calls: %v", f.calls)
	}
}

// herdr's generated tab label is the tab's position plus one, while `number`
// is a stable counter — after an earlier tab closes they disagree. A generated
// label must still be recognized as one, or grove leaves the tab unnamed.
func TestHerdrTabLabelFromPositionCountsAsDefault(t *testing.T) {
	f := newFakeHerdr()
	f.responses["worktree open"] = strings.Replace(herdrOpenedJSON, `"label":"1","number":1`, `"label":"2","number":3`, 1)

	target := Target{Name: "grove-testing", Short: "testing", Path: "/repos/grove-testing", Repo: "/repos/grove"}
	if err := f.backend().Ensure(target); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if !f.called("tab", "rename", "w2:t1", "testing") {
		t.Errorf("Ensure left a generated tab label in place; calls: %v", f.calls)
	}
}

func TestHerdrBinaryPrefersHerdrBinPath(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "herdr-0.9.1")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HERDR_BIN_PATH", bin)
	if got := HerdrBinary(); got != bin {
		t.Errorf("HerdrBinary() = %q, want HERDR_BIN_PATH %q", got, bin)
	}

	t.Setenv("HERDR_BIN_PATH", filepath.Join(t.TempDir(), "missing"))
	if got := HerdrBinary(); got != "herdr" {
		t.Errorf("HerdrBinary() = %q, want PATH fallback for a stale HERDR_BIN_PATH", got)
	}

	t.Setenv("HERDR_BIN_PATH", "")
	if got := HerdrBinary(); got != "herdr" {
		t.Errorf("HerdrBinary() = %q, want PATH fallback when unset", got)
	}
}

func TestHerdrServerStatusUsable(t *testing.T) {
	yes, no := true, false
	cases := []struct {
		name   string
		status HerdrServerStatus
		want   bool
	}{
		{"running and compatible", HerdrServerStatus{Running: true, Compatible: &yes}, true},
		{"stopped", HerdrServerStatus{Running: false}, false},
		{"incompatible", HerdrServerStatus{Running: true, Compatible: &no}, false},
		{"restart needed", HerdrServerStatus{Running: true, Compatible: &yes, RestartNeeded: true}, false},
	}
	for _, tc := range cases {
		if got := tc.status.Usable(); got != tc.want {
			t.Errorf("%s: Usable() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// --- named sessions ---

// Real `herdr session list --json` shape (0.9.1): bare JSON, no envelope.
const herdrSessionListJSON = `{"sessions":[{"default":true,"name":"default","running":true,"session_dir":"/home/u/.config/herdr","socket_path":"/home/u/.config/herdr/herdr.sock"},{"default":false,"name":"other","running":true,"session_dir":"/home/u/.config/herdr/sessions/other","socket_path":"/home/u/.config/herdr/sessions/other/herdr.sock"},{"default":false,"name":"asleep","running":false,"session_dir":"/home/u/.config/herdr/sessions/asleep","socket_path":"/home/u/.config/herdr/sessions/asleep/herdr.sock"}]}`

// workspaceListFor returns a one-workspace listing for checkout.
func workspaceListFor(id, label, checkout string) string {
	return fmt.Sprintf(`{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[`+
		`{"workspace_id":%q,"number":1,"label":%q,"focused":false,"pane_count":1,"tab_count":1,`+
		`"active_tab_id":"%s:t1","agent_status":"unknown","worktree":{"repo_key":"k","repo_name":"grove",`+
		`"repo_root":"/repos/grove","checkout_path":%q,"is_linked_worktree":true}}]}}`, id, label, id, checkout)
}

const emptyWorkspaceListJSON = `{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[]}}`

// herdr sessions are independent servers; `worktree open` only checks its own
// for an existing workspace. Outside herdr, a checkout already open in another
// session must be adopted there — focused, and attached with --session —
// rather than opened a second time in the ambient one.
func TestHerdrEnsureAdoptsAWorkspaceFromAnotherSession(t *testing.T) {
	checkout := t.TempDir()
	f := newFakeHerdr()
	f.responses["workspace list"] = emptyWorkspaceListJSON
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = workspaceListFor("w7", "grove-testing", checkout)
	f.responses["@other workspace focus"] = `{"id":"cli:workspace:focus","result":{"type":"workspace_info"}}`
	b := f.backend()
	target := Target{Name: "grove-testing", Path: checkout, Repo: "/repos/grove"}

	if err := b.Ensure(target); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if f.called("worktree", "open") {
		t.Fatalf("Ensure opened a duplicate workspace; calls: %v", f.calls)
	}
	if got := b.LocatedIn(target); got != "other" {
		t.Errorf("LocatedIn() = %q, want other", got)
	}
	if got := b.AttachHint(target); got != "herdr --session other" {
		t.Errorf("AttachHint() = %q, want the session named", got)
	}

	if err := b.Attach(target); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	if !f.called("--session", "other", "workspace", "focus", "w7") {
		t.Errorf("Attach did not focus the workspace in its own session; calls: %v", f.calls)
	}
	if strings.Join(f.attachArgs, " ") != "--session other" {
		t.Errorf("attach args = %v, want --session other", f.attachArgs)
	}
}

// Inside a herdr pane the CLI cannot move the user's client to another
// session (a nested attach is refused by design). Opening a second workspace
// here is the bug, so decline with a hint that names where it is open.
func TestHerdrEnsureInsideHerdrDeclinesADuplicate(t *testing.T) {
	checkout := t.TempDir()
	f := newFakeHerdr()
	f.responses["workspace list"] = emptyWorkspaceListJSON
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = workspaceListFor("w7", "grove-testing", checkout)
	b := f.backend()
	b.env = func(key string) string {
		switch key {
		case "HERDR_ENV":
			return "1"
		case "HERDR_SOCKET_PATH":
			return "/home/u/.config/herdr/herdr.sock"
		}
		return ""
	}

	err := b.Ensure(Target{Name: "grove-testing", Short: "testing", Path: checkout, Repo: "/repos/grove"})
	if !ErrUnmanaged(err) {
		t.Fatalf("Ensure() error = %v, want ErrUnmanaged", err)
	}
	if f.called("worktree", "open") {
		t.Errorf("Ensure opened a duplicate workspace; calls: %v", f.calls)
	}
	hint := DegradedHint(err)
	if !strings.Contains(hint, `open in herdr session "other"`) || !strings.Contains(hint, "herdr --session other") {
		t.Errorf("DegradedHint() = %q, want the session named", hint)
	}
}

func TestHerdrEnsureOpensHereWhenNoOtherSessionHasIt(t *testing.T) {
	checkout := t.TempDir()
	f := newFakeHerdr()
	f.responses["workspace list"] = emptyWorkspaceListJSON
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = workspaceListFor("w7", "grove-elsewhere", t.TempDir())
	f.responses["worktree open"] = herdrOpenedJSON
	b := f.backend()
	target := Target{Name: "grove-testing", Path: checkout, Repo: "/repos/grove"}

	if err := b.Ensure(target); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if !f.called("worktree", "open") {
		t.Errorf("Ensure did not open the workspace; calls: %v", f.calls)
	}
	if b.LocatedIn(target) != "" {
		t.Errorf("LocatedIn() = %q, want the ambient session", b.LocatedIn(target))
	}
}

// Matching across sessions is by checkout path only: a label in a session
// grove never touched is not proof of identity.
func TestHerdrEnsureIgnoresALabelMatchInAnotherSession(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = emptyWorkspaceListJSON
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = workspaceListFor("w7", "grove-testing", t.TempDir())
	f.responses["worktree open"] = herdrOpenedJSON
	b := f.backend()

	if err := b.Ensure(Target{Name: "grove-testing", Path: t.TempDir(), Repo: "/repos/grove"}); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if !f.called("worktree", "open") {
		t.Errorf("a label match in another session suppressed the open; calls: %v", f.calls)
	}
}

func TestHerdrSkipsStoppedSessions(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = emptyWorkspaceListJSON
	f.responses["session list"] = herdrSessionListJSON
	f.responses["worktree open"] = herdrOpenedJSON

	if err := f.backend().Ensure(Target{Name: "grove-testing", Path: t.TempDir(), Repo: "/repos/grove"}); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if f.called("--session", "asleep") {
		t.Errorf("queried a stopped session; calls: %v", f.calls)
	}
	if f.called("--session", "default") {
		t.Errorf("queried the ambient session as if it were another; calls: %v", f.calls)
	}
}

func TestHerdrEnsureWithCommandAdoptsElsewhereWithoutRunningIt(t *testing.T) {
	checkout := t.TempDir()
	f := newFakeHerdr()
	f.responses["workspace list"] = emptyWorkspaceListJSON
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = workspaceListFor("w7", "grove-testing", checkout)

	if err := f.backend().EnsureWithCommand(Target{Name: "grove-testing", Path: checkout, Repo: "/repos/grove"}, "claude"); err != nil {
		t.Fatalf("EnsureWithCommand() error = %v", err)
	}
	if f.called("pane", "run") || f.called("worktree", "open") {
		t.Errorf("an existing workspace elsewhere must be left alone; calls: %v", f.calls)
	}
}

// `grove rm` used to close only the ambient copy, orphaning the other
// session's workspace on a deleted directory.
func TestHerdrKillClosesCopiesInOtherSessions(t *testing.T) {
	dir := t.TempDir()
	gone := filepath.Join(dir, "grove-testing") // removed before Kill runs
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListFor("w2", "grove-testing", gone)
	f.responses["workspace close"] = herdrCloseOKJSON
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = fmt.Sprintf(`{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[`+
		`{"workspace_id":"w7","number":1,"label":"grove-testing","focused":false,"pane_count":1,"tab_count":1,"active_tab_id":"w7:t1","agent_status":"unknown","worktree":{"repo_key":"k","repo_name":"grove","repo_root":"/repos/grove","checkout_path":%q,"is_linked_worktree":true}},`+
		`{"workspace_id":"w8","number":2,"label":"grove-testing","focused":false,"pane_count":1,"tab_count":1,"active_tab_id":"w8:t1","agent_status":"unknown","worktree":{"repo_key":"k","repo_name":"grove","repo_root":"/repos/grove","checkout_path":"/somewhere/else","is_linked_worktree":true}}]}}`, gone)
	f.responses["@other workspace close"] = herdrCloseOKJSON

	if err := f.backend().Kill(Target{Name: "grove-testing", Path: gone}); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}
	if !f.called("workspace", "close", "w2") {
		t.Errorf("Kill did not close the ambient copy; calls: %v", f.calls)
	}
	if !f.called("--session", "other", "workspace", "close", "w7") {
		t.Errorf("Kill did not close the copy in the other session; calls: %v", f.calls)
	}
	if f.called("workspace", "close", "w8") {
		t.Errorf("Kill closed a workspace that only shares the label; calls: %v", f.calls)
	}
}

func TestHerdrKillReachesOtherSessionsWhenTheAmbientOneIsDown(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "grove-testing")
	f := newFakeHerdr()
	f.responses["workspace list"] = `{"id":"cli:workspace:list","error":{"code":"server_not_running","message":"no herdr server is running"}}`
	f.errs["workspace list"] = errors.New("exit status 1")
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = workspaceListFor("w7", "grove-testing", gone)
	f.responses["@other workspace close"] = herdrCloseOKJSON

	if err := f.backend().Kill(Target{Name: "grove-testing", Path: gone}); err != nil {
		t.Fatalf("Kill() error = %v, want nil with the ambient server down", err)
	}
	if !f.called("--session", "other", "workspace", "close", "w7") {
		t.Errorf("Kill did not reach the other session; calls: %v", f.calls)
	}
}

func TestHerdrAmbientSessionFollowsTheCLIPrecedence(t *testing.T) {
	var listed struct {
		Sessions []herdrSessionInfo `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(herdrSessionListJSON), &listed); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		// Every pane sets HERDR_SOCKET_PATH, and it outranks HERDR_SESSION.
		{"pane socket", map[string]string{"HERDR_SOCKET_PATH": "/home/u/.config/herdr/sessions/other/herdr.sock", "HERDR_SESSION": "default"}, "other"},
		{"HERDR_SESSION", map[string]string{"HERDR_SESSION": "other"}, "other"},
		{"default", nil, "default"},
		{"unknown socket", map[string]string{"HERDR_SOCKET_PATH": "/tmp/stray.sock"}, ""},
	}
	for _, tc := range cases {
		b := newFakeHerdr().backend()
		b.env = func(k string) string { return tc.env[k] }
		if got := b.ambientSession(listed.Sessions); got != tc.want {
			t.Errorf("%s: ambientSession() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestStatusForegroundAndBackground(t *testing.T) {
	for _, s := range []Status{StatusAttached, StatusActive} {
		if !s.Foreground() || s.Background() {
			t.Errorf("%q: Foreground/Background = %v/%v, want true/false", s, s.Foreground(), s.Background())
		}
	}
	for _, s := range []Status{StatusDetached, StatusOpen} {
		if s.Foreground() || !s.Background() {
			t.Errorf("%q: Foreground/Background = %v/%v, want false/true", s, s.Foreground(), s.Background())
		}
	}
	if StatusNone.Foreground() || StatusNone.Background() {
		t.Error("StatusNone must be neither foreground nor background")
	}
}

// --- sidebar marker ---

func TestHerdrReportTokenSetsAndClears(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace report-metadata"] = "" // silent success, as herdr 0.9.1 answers
	b := f.backend()

	if err := b.ReportToken("w2", HerdrUntracked); err != nil {
		t.Fatalf("ReportToken(set) error = %v", err)
	}
	if !f.called("workspace", "report-metadata", "w2", "--source", "lost-in-the.grove", "--token", "grove=untracked") {
		t.Errorf("set did not report the token under grove's fixed source; calls: %v", f.calls)
	}
	if err := b.ReportToken("w2", ""); err != nil {
		t.Fatalf("ReportToken(clear) error = %v", err)
	}
	if !f.called("workspace", "report-metadata", "w2", "--source", "lost-in-the.grove", "--clear-token", "grove") {
		t.Errorf("clear did not clear the token; calls: %v", f.calls)
	}
}

// A worktree created in herdr's UI has a workspace labeled with its branch,
// a generated tab label, and the plugin's untracked marker. Once grove adopts
// it, all three come in line with grove.
func TestHerdrAdoptedRelabelsAndClearsTheMarker(t *testing.T) {
	checkout := t.TempDir()
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListFor("w9", "feat-x", checkout)
	f.responses["workspace rename"] = `{"id":"cli:workspace:rename","result":{"type":"workspace_info"}}`
	f.responses["tab list"] = `{"id":"cli:tab:list","result":{"tabs":[{"agent_status":"unknown","focused":false,"label":"1","number":1,"pane_count":1,"tab_id":"w9:t1","workspace_id":"w9"}],"type":"tab_list"}}`
	f.responses["tab rename"] = `{"id":"cli:tab:rename","result":{"type":"tab_info"}}`
	f.responses["workspace report-metadata"] = ""

	if err := f.backend().Adopted(Target{Name: "app-feat-x", Short: "feat-x", Path: checkout}); err != nil {
		t.Fatalf("Adopted() error = %v", err)
	}
	if !f.called("workspace", "rename", "w9", "app-feat-x") {
		t.Errorf("Adopted did not apply the canonical label; calls: %v", f.calls)
	}
	if !f.called("tab", "rename", "w9:t1", "feat-x") {
		t.Errorf("Adopted did not name the generated tab; calls: %v", f.calls)
	}
	if !f.called("report-metadata", "w9", "--clear-token", "grove") {
		t.Errorf("Adopted did not clear the untracked marker; calls: %v", f.calls)
	}
}

func TestHerdrAdoptedWithoutAWorkspaceIsANoOp(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = emptyWorkspaceListJSON

	if err := f.backend().Adopted(Target{Name: "app-feat-x", Path: t.TempDir()}); err != nil {
		t.Fatalf("Adopted() error = %v", err)
	}
	if f.called("workspace", "rename") || f.called("report-metadata") {
		t.Errorf("Adopted touched herdr with no workspace to update; calls: %v", f.calls)
	}
}

// --- review findings: peer budget, renamed worktrees, attach prep ---

// panesIn returns a one-pane `pane list` response whose shell sits in cwd.
func panesIn(wsID, cwd string) string {
	return fmt.Sprintf(`{"id":"cli:pane:list","result":{"panes":[{"agent_status":"unknown","cwd":%q,"focused":true,"foreground_cwd":%q,"pane_id":"%s:p1","tab_id":"%s:t1","workspace_id":%q}],"type":"pane_list"}}`, cwd, cwd, wsID, wsID, wsID)
}

// Calls to other sessions must run under the short peer budget: herdr reports
// a wedged server as running, and a full-timeout call to it stalled `grove
// to` and `grove rm` by five seconds.
func TestHerdrOtherSessionsUseThePeerRunner(t *testing.T) {
	checkout := t.TempDir()
	f := newFakeHerdr()
	f.responses["workspace list"] = emptyWorkspaceListJSON
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = workspaceListFor("w7", "grove-testing", checkout)

	if err := f.backend().Ensure(Target{Name: "grove-testing", Path: checkout, Repo: "/repos/grove"}); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	peer := map[string]bool{}
	for _, c := range f.peerCalls {
		peer[strings.Join(c, " ")] = true
	}
	if !peer["session list --json"] || !peer["--session other workspace list"] {
		t.Errorf("session list and the other session's listing must use the peer runner; peer calls: %v", f.peerCalls)
	}
	if peer["workspace list"] {
		t.Errorf("the ambient listing went through the peer runner; peer calls: %v", f.peerCalls)
	}
}

// Other sessions are listed side by side, so N sessions cost one peer budget
// rather than N.
func TestHerdrOtherSessionsAreQueriedConcurrently(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = emptyWorkspaceListJSON
	f.responses["session list"] = `{"sessions":[{"default":true,"name":"default","running":true,"socket_path":"/s/default"},{"name":"one","running":true,"socket_path":"/s/one"},{"name":"two","running":true,"socket_path":"/s/two"}]}`
	f.responses["worktree open"] = herdrOpenedJSON
	b := f.backend()

	var arrived sync.WaitGroup
	arrived.Add(2)
	release := make(chan struct{})
	go func() { arrived.Wait(); close(release) }()
	b.peer = func(args []string) ([]byte, error) {
		if len(args) > 3 && args[0] == "--session" && args[2] == "workspace" && args[3] == "list" {
			arrived.Done()
			select {
			case <-release: // both listings were in flight at once
			case <-time.After(2 * time.Second):
				return nil, errors.New("listings ran one after another")
			}
			return []byte(emptyWorkspaceListJSON), nil
		}
		return f.peer(args)
	}

	if err := b.Ensure(Target{Name: "grove-testing", Path: t.TempDir(), Repo: "/repos/grove"}); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	select {
	case <-release:
	default:
		t.Error("the two sessions were not listed concurrently")
	}
}

// A session that does not answer in time cannot be checked, and its copy may
// survive the removal — so say so rather than stay silent.
func TestHerdrKillReportsSessionsItCouldNotCheck(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "grove-testing")
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListFor("w2", "grove-testing", gone)
	f.responses["workspace close"] = herdrCloseOKJSON
	f.responses["session list"] = herdrSessionListJSON
	f.errs["@other workspace list"] = errors.New("signal: killed")
	f.responses["@other workspace list"] = ""

	target := Target{Name: "grove-testing", Path: gone}
	b := f.backend()
	report, err := b.KillEverywhere(target)
	if err != nil {
		t.Fatalf("KillEverywhere() error = %v; an unchecked session is not a failure to close", err)
	}
	if strings.Join(report.Unchecked, ",") != "other" || len(report.ClosedIn) != 0 {
		t.Errorf("report = %+v, want other unchecked and nothing closed elsewhere", report)
	}
	if w := UncheckedWarning(report, target); !strings.Contains(w, `herdr session(s) other did not answer`) {
		t.Errorf("UncheckedWarning() = %q, want the session named", w)
	}
	if !f.called("workspace", "close", "w2") {
		t.Errorf("the ambient copy must still be closed; calls: %v", f.calls)
	}
	// A plain Kill has no report to carry it, so it says so in its error.
	if err := b.Kill(target); err == nil || !strings.Contains(err.Error(), "did not answer") {
		t.Errorf("Kill() error = %v, want the unchecked session reported", err)
	}
}

// A session that stopped between `session list` and its listing is not a
// failure: herdr reconciles a stopped session's workspaces when it restarts.
func TestHerdrKillIgnoresASessionThatJustStopped(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "grove-testing")
	f := newFakeHerdr()
	f.responses["workspace list"] = emptyWorkspaceListJSON
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = `{"id":"cli:workspace:list","error":{"code":"server_not_running","message":"no herdr server is running"}}`
	f.errs["@other workspace list"] = errors.New("exit status 1")

	report, err := f.backend().KillEverywhere(Target{Name: "grove-testing", Path: gone})
	if err != nil || len(report.Unchecked) != 0 {
		t.Errorf("KillEverywhere() = %+v, %v; want a stopped session skipped silently", report, err)
	}
}

func TestHerdrKillEverywhereNamesTheSessionsItClosedIn(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "grove-testing")
	f := newFakeHerdr()
	f.responses["workspace list"] = emptyWorkspaceListJSON
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = workspaceListFor("w7", "grove-testing", gone)
	f.responses["@other workspace close"] = herdrCloseOKJSON

	report, err := f.backend().KillEverywhere(Target{Name: "grove-testing", Path: gone})
	if err != nil {
		t.Fatalf("KillEverywhere() error = %v", err)
	}
	if strings.Join(report.ClosedIn, ",") != "other" {
		t.Errorf("ClosedIn = %v, want [other]", report.ClosedIn)
	}
}

// renamedFixture lays out the rename hazard: the repo, a live worktree "b"
// (renamed from "a"), and the path "a" that a new worktree now occupies. herdr
// still records "a" for the renamed worktree's workspace, whose shell followed
// the rename into "b".
func renamedFixture(t *testing.T) (repo, oldPath, renamed string, f *fakeHerdr) {
	t.Helper()
	root := t.TempDir()
	repo = filepath.Join(root, "grove")
	oldPath = filepath.Join(root, "grove-a")
	renamed = filepath.Join(root, "grove-b")
	for _, d := range []string{repo, oldPath, renamed} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f = newFakeHerdr()
	f.trees = map[string][]string{repo: {repo, oldPath, renamed}}
	return repo, oldPath, renamed, f
}

func workspaceListInRepo(id, label, repo, checkout string) string {
	return fmt.Sprintf(`{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[`+
		`{"workspace_id":%q,"number":1,"label":%q,"focused":false,"pane_count":1,"tab_count":1,`+
		`"active_tab_id":"%s:t1","agent_status":"unknown","worktree":{"repo_key":"k","repo_name":"grove",`+
		`"repo_root":%q,"checkout_path":%q,"is_linked_worktree":true}}]}}`, id, label, id, repo, checkout)
}

// After `grove rename a b` and a new worktree at "a", herdr's `worktree open
// --path a` would hand over — and re-label — the renamed worktree's
// workspace. grove must decline instead, and not count it as a's session.
func TestHerdrRefusesAWorkspaceARenamedWorktreeStillOwns(t *testing.T) {
	repo, oldPath, renamed, f := renamedFixture(t)
	f.responses["workspace list"] = workspaceListInRepo("w5", "grove-a", repo, oldPath)
	f.responses["pane list"] = panesIn("w5", renamed)
	b := f.backend()
	target := Target{Name: "grove-a", Short: "a", Path: oldPath, Repo: repo}

	if exists, err := b.Exists(target); err != nil || exists {
		t.Errorf("Exists() = %v, %v; want false — the workspace serves the renamed worktree", exists, err)
	}
	err := b.Ensure(target)
	if !ErrUnmanaged(err) {
		t.Fatalf("Ensure() error = %v, want ErrUnmanaged", err)
	}
	if f.called("worktree", "open") {
		t.Errorf("Ensure let herdr hand over the renamed worktree's workspace; calls: %v", f.calls)
	}
	if hint := DegradedHint(err); !strings.Contains(hint, "still belongs to the worktree at "+renamed) {
		t.Errorf("DegradedHint() = %q, want it to name the renamed worktree", hint)
	}
}

// ...and `grove rm a` must not close it: its shells are the renamed
// worktree's live work.
func TestHerdrKillLeavesARenamedWorktreesWorkspace(t *testing.T) {
	repo, oldPath, renamed, f := renamedFixture(t)
	f.responses["workspace list"] = workspaceListInRepo("w5", "grove-a", repo, oldPath)
	f.responses["pane list"] = panesIn("w5", renamed)
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = workspaceListInRepo("w7", "grove-a", repo, oldPath)
	f.responses["@other pane list"] = panesIn("w7", renamed)

	if err := f.backend().Kill(Target{Name: "grove-a", Path: oldPath, Repo: repo}); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}
	if f.called("workspace", "close") {
		t.Errorf("Kill closed a workspace serving the renamed worktree; calls: %v", f.calls)
	}
}

// The same guard across sessions: never adopt another session's workspace
// that serves a different worktree.
func TestHerdrDoesNotAdoptARenamedWorktreesWorkspaceElsewhere(t *testing.T) {
	repo, oldPath, renamed, f := renamedFixture(t)
	f.responses["workspace list"] = emptyWorkspaceListJSON
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = workspaceListInRepo("w7", "grove-a", repo, oldPath)
	f.responses["@other pane list"] = panesIn("w7", renamed)
	f.responses["worktree open"] = herdrOpenedJSON
	b := f.backend()
	target := Target{Name: "grove-a", Path: oldPath, Repo: repo}

	if err := b.Ensure(target); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if b.LocatedIn(target) != "" {
		t.Errorf("adopted the renamed worktree's workspace from session %q", b.LocatedIn(target))
	}
}

// A pane that merely wandered off — `cd /tmp`, or into the main checkout — is
// not evidence of a rename; an ordinary workspace must still be closed.
func TestHerdrKillStillClosesAWorkspaceWhosePaneWandered(t *testing.T) {
	for name, cwd := range map[string]func(repo string) string{
		"elsewhere":     func(string) string { return t.TempDir() },
		"main checkout": func(repo string) string { return repo },
	} {
		t.Run(name, func(t *testing.T) {
			repo, oldPath, _, f := renamedFixture(t)
			f.responses["workspace list"] = workspaceListInRepo("w5", "grove-a", repo, oldPath)
			f.responses["pane list"] = panesIn("w5", cwd(repo))
			f.responses["workspace close"] = herdrCloseOKJSON

			if err := f.backend().Kill(Target{Name: "grove-a", Path: oldPath, Repo: repo}); err != nil {
				t.Fatalf("Kill() error = %v", err)
			}
			if !f.called("workspace", "close", "w5") {
				t.Errorf("Kill left an ordinary workspace open; calls: %v", f.calls)
			}
		})
	}
}

// Under shell integration grove prints an attach hint instead of attaching.
// herdr's client starts on the focused workspace, so the hint only works once
// the target is focused — in the session that holds it.
func TestHerdrPrepareAttachFocusesInTheOwningSession(t *testing.T) {
	checkout := t.TempDir()
	f := newFakeHerdr()
	f.responses["workspace list"] = emptyWorkspaceListJSON
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = workspaceListFor("w7", "grove-testing", checkout)
	f.responses["@other workspace focus"] = `{"id":"cli:workspace:focus","result":{"type":"workspace_info"}}`
	b := f.backend()
	target := Target{Name: "grove-testing", Path: checkout, Repo: "/repos/grove"}

	if err := b.Ensure(target); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if err := b.PrepareAttach(target); err != nil {
		t.Fatalf("PrepareAttach() error = %v", err)
	}
	if !f.called("--session", "other", "workspace", "focus", "w7") {
		t.Errorf("PrepareAttach did not focus the workspace in its session; calls: %v", f.calls)
	}
}

// Pane operations on an adopted workspace reach it in its own session — the
// ambient session has no such workspace.
func TestHerdrSendCommandReachesAnAdoptedWorkspace(t *testing.T) {
	checkout := t.TempDir()
	f := newFakeHerdr()
	f.responses["workspace list"] = emptyWorkspaceListJSON
	f.responses["session list"] = herdrSessionListJSON
	f.responses["@other workspace list"] = workspaceListFor("w7", "grove-testing", checkout)
	f.responses["@other pane list"] = panesIn("w7", checkout)
	f.responses["@other pane run"] = herdrPaneRunOK
	b := f.backend()
	target := Target{Name: "grove-testing", Path: checkout, Repo: "/repos/grove"}

	if err := b.Ensure(target); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if err := b.SendCommand(target, "claude"); err != nil {
		t.Fatalf("SendCommand() error = %v", err)
	}
	if !f.called("--session", "other", "pane", "run", "w7:p1", "claude") {
		t.Errorf("SendCommand did not run in the adopted workspace's session; calls: %v", f.calls)
	}
}
