package mux

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHerdr records invocations and replays canned responses keyed by the
// first two argv words (the herdr command group and subcommand).
type fakeHerdr struct {
	calls     [][]string
	responses map[string]string
	errs      map[string]error
	attached  int
}

func newFakeHerdr() *fakeHerdr {
	return &fakeHerdr{responses: map[string]string{}, errs: map[string]error{}}
}

func (f *fakeHerdr) key(args []string) string {
	if len(args) >= 2 {
		return args[0] + " " + args[1]
	}
	if len(args) == 1 {
		return args[0]
	}
	return ""
}

func (f *fakeHerdr) run(args []string) ([]byte, error) {
	f.calls = append(f.calls, args)
	k := f.key(args)
	if err, ok := f.errs[k]; ok {
		return []byte(f.responses[k]), err
	}
	return []byte(f.responses[k]), nil
}

func (f *fakeHerdr) backend() *HerdrBackend {
	b := NewHerdr()
	b.run = f.run
	b.attach = func() error { f.attached++; return nil }
	b.available = func() bool { return true }
	b.env = func(string) string { return "" }
	return b
}

// called reports whether any invocation matched every supplied fragment.
func (f *fakeHerdr) called(fragments ...string) bool {
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
	if main.Status != StatusAttached {
		t.Errorf("focused workspace Status = %q, want %q", main.Status, StatusAttached)
	}
	if main.Agent != AgentWorking {
		t.Errorf("Agent = %q, want %q", main.Agent, AgentWorking)
	}
	if main.Windows != 2 {
		t.Errorf("Windows = %d, want pane_count 2", main.Windows)
	}

	if sessions[1].Status != StatusDetached {
		t.Errorf("unfocused workspace Status = %q, want %q", sessions[1].Status, StatusDetached)
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

// mainCheckoutRefusal is the error herdr returns for `worktree open` against a
// repository's own checkout: it is not a linked worktree, so there is nothing
// to open.
func mainCheckoutRefusal(f *fakeHerdr) {
	f.responses["worktree open"] = `{"id":"x","error":{"code":"worktree_not_found","message":"worktree cannot be opened"}}`
	f.errs["worktree open"] = errors.New("exit status 1")
}

func TestHerdrNeverCreatesAWorkspace(t *testing.T) {
	// grove owns worktree lifecycle, not herdr's workspace inventory. The
	// repository's own workspace is herdr's to create — it is the thing herdr's
	// sidebar groups a project's worktrees under, and grove imposing one
	// reaches past the boundary the integration is built on. Ensure must report
	// the target unmanaged instead, and the caller falls through to a plain
	// directory switch.
	f := newFakeHerdr()
	mainCheckoutRefusal(f)
	f.responses["workspace list"] = `{"id":"x","result":{"type":"workspace_list","workspaces":[]}}`

	err := f.backend().Ensure(Target{Name: "grove", Path: "/repos/grove", Repo: "/repos/grove"})
	if !ErrUnmanaged(err) {
		t.Fatalf("Ensure() error = %v, want an unmanaged-target error", err)
	}
	if f.called("workspace", "create") {
		t.Errorf("Ensure created a workspace for the repository checkout; calls: %v", f.calls)
	}
}

func TestHerdrEnsureAdoptsAnExistingRepositoryWorkspace(t *testing.T) {
	// herdr materializes the repository's workspace itself when it opens any
	// linked worktree. When that workspace is already there, `grove to root`
	// should land in it rather than report the target unmanaged.
	checkout := t.TempDir()
	f := newFakeHerdr()
	mainCheckoutRefusal(f)
	f.responses["workspace list"] = fmt.Sprintf(
		`{"id":"x","result":{"type":"workspace_list","workspaces":[`+
			`{"workspace_id":"w1","number":1,"label":"grove","focused":false,`+
			`"pane_count":1,"tab_count":1,"active_tab_id":"w1:t1","agent_status":"idle",`+
			`"worktree":{"repo_key":"k","repo_name":"grove","repo_root":%q,`+
			`"checkout_path":%q,"is_linked_worktree":false}}]}}`, checkout, checkout)

	if err := f.backend().Ensure(Target{Name: "grove", Path: checkout, Repo: checkout}); err != nil {
		t.Fatalf("Ensure() error = %v, want nil for an already-present repository workspace", err)
	}
	if f.called("workspace", "create") {
		t.Errorf("Ensure created a workspace instead of adopting; calls: %v", f.calls)
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
	f.responses["workspace close"] = `{"id":"x","result":{"type":"workspace_closed"}}`

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
	f.responses["pane list"] = `{"id":"x","result":{"type":"pane_list","panes":[{"pane_id":"w2:p1","terminal_id":"t","workspace_id":"w2","tab_id":"w2:t1","focused":true,"cwd":"/elsewhere","agent_status":"unknown"}]}}`
	f.responses["pane process-info"] = `{"id":"x","result":{"type":"pane_process_info","pane_id":"w2:p1","foreground_processes":[{"pid":1,"name":"zsh"}]}}`

	info, err := f.backend().PaneInfo(Target{Name: "grove-testing", Path: "/repos/grove-testing"})
	if err != nil {
		t.Fatalf("PaneInfo() error = %v", err)
	}
	if !info.IsShell() {
		t.Errorf("IsShell() = false for a zsh pane (command=%q)", info.CurrentCommand)
	}
}

func TestHerdrPaneInfoPrefersForegroundCwd(t *testing.T) {
	f := newFakeHerdr()
	f.responses["workspace list"] = workspaceListJSON
	f.responses["pane list"] = `{"id":"x","result":{"type":"pane_list","panes":[{"pane_id":"w2:p1","terminal_id":"t","workspace_id":"w2","tab_id":"w2:t1","focused":true,"cwd":"/stale","foreground_cwd":"/live","agent_status":"unknown"}]}}`
	f.responses["pane process-info"] = `{"id":"x","result":{"type":"pane_process_info","pane_id":"w2:p1","foreground_processes":[{"pid":1,"name":"bash"}]}}`

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
	// The main checkout is its own repo root; herdr reuses the parent
	// workspace and reports already_open rather than erroring.
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

	if err := f.backend().Notify("grove: worktree not tracked", "run grove adopt"); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if !f.called("notification", "show", "grove: worktree not tracked", "--body", "run grove adopt") {
		t.Errorf("Notify did not pass title and body; calls: %v", f.calls)
	}
}

func TestHerdrNotifyOmitsAnEmptyBody(t *testing.T) {
	f := newFakeHerdr()
	f.responses["notification show"] = `{"id":"x","result":{"type":"notification_show","shown":true}}`

	if err := f.backend().Notify("title only", ""); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if f.called("--body") {
		t.Errorf("Notify passed an empty --body; calls: %v", f.calls)
	}
}
