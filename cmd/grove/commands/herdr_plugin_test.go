package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/state"
)

func TestParseHerdrContext(t *testing.T) {
	raw := `{"workspace_id":"w2","workspace_label":"grove-testing","workspace_cwd":"/repos/grove-testing",
"worktree":{"repo_key":"k","repo_name":"grove","repo_root":"/repos/grove","checkout_path":"/repos/grove-testing","is_linked_worktree":true},
"focused_pane_id":"w2:p1"}`

	got, err := parseHerdrContext(raw)
	if err != nil {
		t.Fatalf("parseHerdrContext() error = %v", err)
	}
	if got.WorkspaceID != "w2" {
		t.Errorf("WorkspaceID = %q, want w2", got.WorkspaceID)
	}
	if got.CheckoutPath() != "/repos/grove-testing" {
		t.Errorf("CheckoutPath() = %q, want the worktree checkout", got.CheckoutPath())
	}
}

func TestParseHerdrContextFallsBackToWorkspaceCwd(t *testing.T) {
	// A workspace with no git provenance carries no worktree block, but its
	// cwd is still the directory the user means.
	raw := `{"workspace_id":"w3","workspace_label":"scratch","workspace_cwd":"/tmp/scratch"}`

	got, err := parseHerdrContext(raw)
	if err != nil {
		t.Fatalf("parseHerdrContext() error = %v", err)
	}
	if got.CheckoutPath() != "/tmp/scratch" {
		t.Errorf("CheckoutPath() = %q, want the workspace cwd", got.CheckoutPath())
	}
}

func TestParseHerdrContextRejectsGarbage(t *testing.T) {
	if _, err := parseHerdrContext("not json"); err == nil {
		t.Error("parseHerdrContext() accepted non-JSON input")
	}
}

func TestParseHerdrContextRejectsEmpty(t *testing.T) {
	// The env var is absent when grove is run outside a plugin invocation.
	if _, err := parseHerdrContext(""); err == nil {
		t.Error("parseHerdrContext() accepted empty input")
	}
}

// Real worktree.opened payload from herdr 0.9.1, captured with a probe plugin
// (paths rewritten). herdr always sends the workspace object; the top-level
// workspace_id appears only on worktree.removed.
const herdrOpenedEventJSON = `{"event":"worktree_opened","data":{"type":"worktree_opened","workspace":{"workspace_id":"wH","number":8,"label":"grove-testing","focused":false,"pane_count":1,"tab_count":1,"active_tab_id":"wH:t1","agent_status":"unknown","worktree":{"repo_key":"/repos/grove/.git","repo_name":"grove","repo_root":"/repos/grove","checkout_path":"/repos/grove-testing","is_linked_worktree":true}},"worktree":{"path":"/repos/grove-testing","branch":"feat/x","is_bare":false,"is_detached":false,"is_prunable":false,"is_linked_worktree":true,"open_workspace_id":"wH","label":"grove"},"already_open":false}}`

func TestParseHerdrEvent(t *testing.T) {
	// herdr serializes the envelope's event field in snake_case even though
	// manifests and HERDR_PLUGIN_EVENT use the dotted form.
	got, err := parseHerdrEvent(herdrOpenedEventJSON)
	if err != nil {
		t.Fatalf("parseHerdrEvent() error = %v", err)
	}
	if got.Event != "worktree.opened" {
		t.Errorf("Event = %q, want worktree.opened", got.Event)
	}
	if got.CheckoutPath() != "/repos/grove-testing" {
		t.Errorf("CheckoutPath() = %q, want /repos/grove-testing", got.CheckoutPath())
	}
	if got.WorkspaceID() != "wH" {
		t.Errorf("WorkspaceID() = %q, want wH from data.workspace", got.WorkspaceID())
	}
	if got.RepoRoot() != "/repos/grove" {
		t.Errorf("RepoRoot() = %q, want /repos/grove", got.RepoRoot())
	}
	if got.AlreadyOpen {
		t.Error("AlreadyOpen = true, want false")
	}
	if got.Token(mux.HerdrTokenName) != "" {
		t.Errorf("Token() = %q, want empty for a workspace with no tokens", got.Token(mux.HerdrTokenName))
	}
}

func TestParseHerdrEventAlreadyOpen(t *testing.T) {
	// A re-open (grove's own `grove to`, or herdr's picker) must be
	// distinguishable, so the hook raises its prompt only once.
	raw := strings.Replace(herdrOpenedEventJSON, `"already_open":false`, `"already_open":true`, 1)

	got, err := parseHerdrEvent(raw)
	if err != nil {
		t.Fatalf("parseHerdrEvent() error = %v", err)
	}
	if !got.AlreadyOpen {
		t.Error("AlreadyOpen = false, want true")
	}
}

func TestParseHerdrEventReadsWorkspaceTokens(t *testing.T) {
	raw := strings.Replace(herdrOpenedEventJSON, `"agent_status":"unknown",`, `"agent_status":"unknown","tokens":{"grove":"untracked"},`, 1)

	got, err := parseHerdrEvent(raw)
	if err != nil {
		t.Fatalf("parseHerdrEvent() error = %v", err)
	}
	if got.Token(mux.HerdrTokenName) != mux.HerdrUntracked {
		t.Errorf("Token() = %q, want %q", got.Token(mux.HerdrTokenName), mux.HerdrUntracked)
	}
}

func TestNormalizeHerdrEventNameResolvesBothForms(t *testing.T) {
	// Manifests and HERDR_PLUGIN_EVENT use dots; the JSON envelope uses
	// underscores. Both must resolve to the same event.
	for _, in := range []string{"worktree.opened", "worktree_opened"} {
		if got := normalizeHerdrEventName(in); got != "worktree.opened" {
			t.Errorf("normalizeHerdrEventName(%q) = %q, want worktree.opened", in, got)
		}
	}
}

func TestParseHerdrEventAcceptsDottedForm(t *testing.T) {
	raw := `{"event":"worktree.opened","data":{"worktree":{"path":"/repos/x"},"already_open":false}}`

	got, err := parseHerdrEvent(raw)
	if err != nil {
		t.Fatalf("parseHerdrEvent() error = %v", err)
	}
	if got.Event != "worktree.opened" {
		t.Errorf("Event = %q, want worktree.opened", got.Event)
	}
}

func TestHerdrContextResolveDirSkipsStalePath(t *testing.T) {
	// herdr captures worktree provenance when a workspace opens and does not
	// follow the checkout afterwards, so `grove rename` leaves checkout_path
	// dangling. The live workspace cwd must win over the dead path.
	live := t.TempDir()
	c := &herdrContext{
		WorkspaceCwd: live,
		Worktree: &struct {
			RepoName     string `json:"repo_name"`
			RepoRoot     string `json:"repo_root"`
			CheckoutPath string `json:"checkout_path"`
		}{CheckoutPath: filepath.Join(live, "does-not-exist")},
	}

	if got := c.ResolveDir(); got != live {
		t.Errorf("ResolveDir() = %q, want the live workspace cwd %q", got, live)
	}
}

func TestHerdrContextResolveDirPrefersCheckout(t *testing.T) {
	live := t.TempDir()
	other := t.TempDir()
	c := &herdrContext{
		WorkspaceCwd: other,
		Worktree: &struct {
			RepoName     string `json:"repo_name"`
			RepoRoot     string `json:"repo_root"`
			CheckoutPath string `json:"checkout_path"`
		}{CheckoutPath: live},
	}

	if got := c.ResolveDir(); got != live {
		t.Errorf("ResolveDir() = %q, want the checkout path %q", got, live)
	}
}

func TestHerdrContextResolveDirReturnsEmptyWhenAllPathsGone(t *testing.T) {
	// After `grove rename`, herdr's checkout path, workspace cwd, and pane cwd
	// are all the pre-rename directory. Reporting "" lets the caller explain
	// that rather than blaming the dead path for not being a grove project.
	gone := filepath.Join(t.TempDir(), "gone")
	c := &herdrContext{
		WorkspaceCwd:   gone,
		FocusedPaneCwd: gone,
		Worktree: &struct {
			RepoName     string `json:"repo_name"`
			RepoRoot     string `json:"repo_root"`
			CheckoutPath string `json:"checkout_path"`
		}{CheckoutPath: gone},
	}

	if got := c.ResolveDir(); got != "" {
		t.Errorf("ResolveDir() = %q, want empty when nothing exists", got)
	}
}

func TestHerdrContextResolveDirFallsBackToPaneCwd(t *testing.T) {
	live := t.TempDir()
	c := &herdrContext{
		WorkspaceCwd:   filepath.Join(live, "gone"),
		FocusedPaneCwd: live,
	}

	if got := c.ResolveDir(); got != live {
		t.Errorf("ResolveDir() = %q, want the pane cwd %q", got, live)
	}
}

// TestHerdrPluginManifestCommandsResolve guards the plugin's argv against
// drift in grove's own command set.
//
// herdr runs plugin commands as plain argv, so a manifest naming a subcommand
// grove does not have fails only at runtime, on the user's machine, with the
// error buried in herdr's plugin log. Parsing the manifest proves it is
// well-formed, not that it is callable — `herdr plugin link` accepted the
// broken version happily.
//
// This is not hypothetical: the manifest shipped `["grove", "tui"]` for a
// dashboard pane while the TUI is reached through bare `grove`, so `grove tui`
// exited with `unknown command "tui"` and the pane died instantly. That pane
// has since been removed, but the same failure mode applies to every action and
// event command left in the manifest, and to anything added later.
func TestHerdrPluginManifestCommandsResolve(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "..", "integrations", "herdr", "herdr-plugin.toml")

	var manifest struct {
		Panes []struct {
			ID      string   `toml:"id"`
			Command []string `toml:"command"`
		} `toml:"panes"`
		Actions []struct {
			ID      string   `toml:"id"`
			Command []string `toml:"command"`
		} `toml:"actions"`
		Events []struct {
			On      string   `toml:"on"`
			Command []string `toml:"command"`
		} `toml:"events"`
		Startup []struct {
			Command []string `toml:"command"`
		} `toml:"startup"`
	}

	if _, err := toml.DecodeFile(manifestPath, &manifest); err != nil {
		t.Fatalf("decode %s: %v", manifestPath, err)
	}

	type entry struct {
		what string
		argv []string
	}
	var entries []entry
	for _, p := range manifest.Panes {
		entries = append(entries, entry{"pane " + p.ID, p.Command})
	}
	for _, a := range manifest.Actions {
		entries = append(entries, entry{"action " + a.ID, a.Command})
	}
	for _, e := range manifest.Events {
		entries = append(entries, entry{"event " + e.On, e.Command})
	}
	for i, st := range manifest.Startup {
		entries = append(entries, entry{fmt.Sprintf("startup hook %d", i), st.Command})
	}
	if len(entries) == 0 {
		t.Fatal("manifest declared no commands; the test is not looking at the right file")
	}

	for _, ent := range entries {
		if len(ent.argv) == 0 {
			t.Errorf("%s: empty command", ent.what)
			continue
		}
		if ent.argv[0] != "grove" {
			continue // not ours to validate
		}
		// Bare `grove` is the TUI entrypoint — rootCmd's own RunE.
		if len(ent.argv) == 1 {
			if rootCmd.RunE == nil {
				t.Errorf("%s: bare `grove` but rootCmd has no RunE", ent.what)
			}
			continue
		}

		// Only the first token after `grove` is a subcommand; the rest are
		// arguments (e.g. `grove herdr-action status`).
		name := ent.argv[1]
		var found bool
		for _, c := range rootCmd.Commands() {
			if c.Name() == name {
				found = true
				break
			}
			for _, alias := range c.Aliases {
				if alias == name {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Errorf("%s: manifest runs %q, but grove has no such subcommand", ent.what, strings.Join(ent.argv, " "))
		}
	}
}

func TestNormalizeHerdrEventName(t *testing.T) {
	// Only the first separator is a dot. Verified against herdr 0.8.0 and
	// 0.9.1: it recognizes "pane.agent_status_changed" and warns that both
	// "pane.agent.status.changed" and "pane_agent_status_changed" are unknown
	// events (whose hooks then never fire). The old ReplaceAll mangled every multi-word name, in both
	// directions — the wire form and the already-dotted HERDR_PLUGIN_EVENT.
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "wire form, one word each side", in: "worktree_created", want: "worktree.created"},
		{name: "wire form, multi-word tail", in: "pane_agent_status_changed", want: "pane.agent_status_changed"},
		{name: "already dotted is left alone", in: "worktree.opened", want: "worktree.opened"},
		{name: "already dotted multi-word is not re-mangled", in: "pane.agent_status_changed", want: "pane.agent_status_changed"},
		{name: "no separator at all", in: "opened", want: "opened"},
		{name: "empty", in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeHerdrEventName(tt.in); got != tt.want {
				t.Errorf("normalizeHerdrEventName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// removedEventJSON is the worktree.removed payload shape captured live from
// herdr 0.8.0 (`herdr worktree remove --workspace <id>`), with paths templated.
// The removed checkout is in data.worktree.path; the surviving main repo is in
// data.workspace.worktree.repo_root — the only path that still exists when the
// hook runs.
func removedEventJSON(repoRoot, checkout string) string {
	return fmt.Sprintf(`{"event":"worktree_removed","data":{"type":"worktree_removed","workspace_id":"w1G",
"workspace":{"workspace_id":"w1G","number":11,"label":"probe2","focused":false,"pane_count":1,"tab_count":1,
"active_tab_id":"w1G:t1","agent_status":"unknown","worktree":{"repo_key":"%s/.git","repo_name":"main-repo",
"repo_root":"%s","checkout_path":"%s","is_linked_worktree":true}},
"worktree":{"path":"%s","branch":"probe2","is_bare":false,"is_detached":false,"is_prunable":false,
"is_linked_worktree":true,"label":"main-repo"},"forced":false}}`,
		repoRoot, repoRoot, checkout, checkout)
}

func TestParseHerdrEventWorktreeRemoved(t *testing.T) {
	got, err := parseHerdrEvent(removedEventJSON("/repos/grove", "/repos/grove-feat"))
	if err != nil {
		t.Fatalf("parseHerdrEvent() error = %v", err)
	}
	if got.Event != "worktree.removed" {
		t.Errorf("Event = %q, want worktree.removed", got.Event)
	}
	if got.CheckoutPath() != "/repos/grove-feat" {
		t.Errorf("CheckoutPath() = %q, want the removed checkout", got.CheckoutPath())
	}
	if got.RepoRoot() != "/repos/grove" {
		t.Errorf("RepoRoot() = %q, want the surviving main repo", got.RepoRoot())
	}
}

// fakeMux records Kill calls so tests never touch a real tmux/herdr server.
type fakeMux struct {
	*mux.OffBackend
	backend mux.Backend
	exists  bool
	killed  []mux.Target
}

func (f *fakeMux) Backend() mux.Backend            { return f.backend }
func (f *fakeMux) Available() bool                 { return true }
func (f *fakeMux) Exists(mux.Target) (bool, error) { return f.exists, nil }
func (f *fakeMux) Kill(t mux.Target) error         { f.killed = append(f.killed, t); return nil }

// removalFixture builds an on-disk grove project: a real git repo (FindRoot
// needs one) with a .grove dir, one tracked-but-gone worktree in state, and
// last_worktree pointing at it. Paths are EvalSymlinks-resolved because
// FindRoot canonicalizes and macOS temp dirs live behind /private symlinks.
func removalFixture(t *testing.T) (repoRoot, removedPath string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("commit", "--allow-empty", "-m", "init")

	groveDir := filepath.Join(repoRoot, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mgr, err := state.NewManager(groveDir)
	if err != nil {
		t.Fatal(err)
	}
	// The checkout herdr removed: a sibling dir that no longer exists —
	// exactly what the hook sees.
	removedPath = repoRoot + "-probe2"
	if err := mgr.AddWorktree("probe2", &state.WorktreeState{
		Path:          removedPath,
		Branch:        "probe2",
		DockerProject: "grove-probe2",
	}); err != nil {
		t.Fatal(err)
	}
	if err := mgr.SetLastWorktree("probe2"); err != nil {
		t.Fatal(err)
	}
	return repoRoot, removedPath
}

func reloadState(t *testing.T, repoRoot string) *state.Manager {
	t.Helper()
	mgr, err := state.NewManager(filepath.Join(repoRoot, ".grove"))
	if err != nil {
		t.Fatal(err)
	}
	return mgr
}

func TestReconcileRemovedWorktreeDropsStateAndLast(t *testing.T) {
	repoRoot, removedPath := removalFixture(t)
	event, err := parseHerdrEvent(removedEventJSON(repoRoot, removedPath))
	if err != nil {
		t.Fatal(err)
	}

	fake := &fakeMux{backend: mux.BackendTmux, exists: true}
	if err := reconcileRemovedWorktree(event, fake); err != nil {
		t.Fatalf("reconcileRemovedWorktree() error = %v", err)
	}

	mgr := reloadState(t, repoRoot)
	if ws, _ := mgr.GetWorktree("probe2"); ws != nil {
		t.Error("state entry survived — herdr removed the worktree but grove still tracks it")
	}
	if last, _ := mgr.GetLastWorktree(); last != "" {
		t.Errorf("last_worktree = %q, want cleared — `grove last` would error on it", last)
	}
	if len(fake.killed) != 1 {
		t.Errorf("Kill calls = %d, want 1 (orphaned tmux session should be reaped)", len(fake.killed))
	}
}

func TestReconcileRemovedWorktreeSkipsMuxOnHerdrBackend(t *testing.T) {
	// On the herdr backend the workspace close is herdr's own removal flow;
	// calling back into the server mid-event is unverified territory. State
	// still gets reconciled — only the session step is skipped.
	repoRoot, removedPath := removalFixture(t)
	event, err := parseHerdrEvent(removedEventJSON(repoRoot, removedPath))
	if err != nil {
		t.Fatal(err)
	}

	fake := &fakeMux{backend: mux.BackendHerdr, exists: true}
	if err := reconcileRemovedWorktree(event, fake); err != nil {
		t.Fatalf("reconcileRemovedWorktree() error = %v", err)
	}

	if len(fake.killed) != 0 {
		t.Errorf("Kill calls = %d, want 0 on the herdr backend", len(fake.killed))
	}
	mgr := reloadState(t, repoRoot)
	if ws, _ := mgr.GetWorktree("probe2"); ws != nil {
		t.Error("state entry survived on the herdr backend — reconciliation must not depend on the mux step")
	}
}

func TestReconcileRemovedWorktreeIgnoresUntrackedPath(t *testing.T) {
	repoRoot, _ := removalFixture(t)
	event, err := parseHerdrEvent(removedEventJSON(repoRoot, repoRoot+"-never-tracked"))
	if err != nil {
		t.Fatal(err)
	}

	fake := &fakeMux{backend: mux.BackendTmux, exists: true}
	if err := reconcileRemovedWorktree(event, fake); err != nil {
		t.Fatalf("reconcileRemovedWorktree() error = %v", err)
	}

	mgr := reloadState(t, repoRoot)
	if ws, _ := mgr.GetWorktree("probe2"); ws == nil {
		t.Error("unrelated state entry was removed")
	}
	if last, _ := mgr.GetLastWorktree(); last != "probe2" {
		t.Errorf("last_worktree = %q, want untouched probe2", last)
	}
	if len(fake.killed) != 0 {
		t.Errorf("Kill calls = %d, want 0 for an untracked path", len(fake.killed))
	}
}

func TestReconcileRemovedWorktreeIgnoresNonGroveRepo(t *testing.T) {
	// A herdr-removed worktree of a repo grove doesn't manage is not an error —
	// the hook fires for every worktree removal, grove-related or not.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	event, err := parseHerdrEvent(removedEventJSON(repoRoot, repoRoot+"-feat"))
	if err != nil {
		t.Fatal(err)
	}
	if err := reconcileRemovedWorktree(event, &fakeMux{backend: mux.BackendTmux}); err != nil {
		t.Errorf("reconcileRemovedWorktree() error = %v, want nil for a non-grove repo", err)
	}
}

func TestReconcileRemovedWorktreeIgnoresPartialPayload(t *testing.T) {
	// Defensive: a payload missing the worktree or workspace block must be a
	// quiet no-op, not a crash inside a background hook.
	for name, raw := range map[string]string{
		"no data":         `{"event":"worktree_removed"}`,
		"no worktree":     `{"event":"worktree_removed","data":{"workspace_id":"w1"}}`,
		"no workspace":    `{"event":"worktree_removed","data":{"worktree":{"path":"/gone"}}}`,
		"empty repo_root": `{"event":"worktree_removed","data":{"worktree":{"path":"/gone"},"workspace":{"worktree":{"repo_root":""}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			event, err := parseHerdrEvent(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := reconcileRemovedWorktree(event, &fakeMux{backend: mux.BackendTmux}); err != nil {
				t.Errorf("reconcileRemovedWorktree() error = %v, want nil", err)
			}
		})
	}
}

// fakeTokenReporter records sidebar-token reports.
type fakeTokenReporter struct {
	reports  []string // "workspace=value", value empty for a clear
	sessions []mux.Session
}

func (f *fakeTokenReporter) Available() bool { return true }
func (f *fakeTokenReporter) ReportToken(id, value string) error {
	f.reports = append(f.reports, id+"="+value)
	return nil
}
func (f *fakeTokenReporter) List() ([]mux.Session, error) { return f.sessions, nil }

func openedEvent(t *testing.T, tokens string) *herdrEvent {
	t.Helper()
	raw := herdrOpenedEventJSON
	if tokens != "" {
		raw = strings.Replace(raw, `"agent_status":"unknown",`, `"agent_status":"unknown","tokens":`+tokens+`,`, 1)
	}
	ev, err := parseHerdrEvent(raw)
	if err != nil {
		t.Fatal(err)
	}
	return ev
}

// The marker follows grove's state, and herdr is only called when the marker
// is wrong — grove's own `grove to` re-opens tracked worktrees constantly.
func TestSyncUntrackedToken(t *testing.T) {
	cases := []struct {
		name    string
		tokens  string
		tracked bool
		want    []string
	}{
		{"untracked, unmarked: mark it", "", false, []string{"wH=untracked"}},
		{"untracked, already marked: leave it", `{"grove":"untracked"}`, false, nil},
		{"tracked, still marked: clear it", `{"grove":"untracked"}`, true, []string{"wH="}},
		{"tracked, unmarked: leave it", "", true, nil},
	}
	for _, tc := range cases {
		f := &fakeTokenReporter{}
		syncUntrackedToken(cli.NewStderr(), f, openedEvent(t, tc.tokens), tc.tracked)
		if strings.Join(f.reports, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: reports = %v, want %v", tc.name, f.reports, tc.want)
		}
	}
}

// After a herdr restart every token is gone; the startup hook marks each
// untracked worktree of a grove project again, and nothing else.
func TestMarkUntrackedWorkspaces(t *testing.T) {
	repoRoot, _ := removalFixture(t)

	// An untracked worktree of the same repository.
	untracked := repoRoot + "-herdr-made"
	add := exec.Command("git", "worktree", "add", "-q", "-b", "herdr-made", untracked)
	add.Dir = repoRoot
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, out)
	}
	// A directory in no grove project at all.
	stranger := t.TempDir()

	f := &fakeTokenReporter{sessions: []mux.Session{
		{ID: "w1", Path: repoRoot},                         // main checkout: always tracked
		{ID: "w2", Path: untracked},                        // untracked: mark
		{ID: "w3", Path: stranger},                         // not grove's business
		{ID: "w4", Path: filepath.Join(untracked, "gone")}, // missing dir: skip
		{ID: "w5"}, // no git provenance
	}}
	markUntrackedWorkspaces(cli.NewStderr(), f)

	if strings.Join(f.reports, ",") != "w2=untracked" {
		t.Errorf("reports = %v, want only w2=untracked", f.reports)
	}
}
