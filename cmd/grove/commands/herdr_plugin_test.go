package commands

import (
	"path/filepath"
	"testing"
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
	if got.RepoRoot() != "/repos/grove" {
		t.Errorf("RepoRoot() = %q, want /repos/grove", got.RepoRoot())
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
	if got.RepoRoot() != "" {
		t.Errorf("RepoRoot() = %q, want empty without git provenance", got.RepoRoot())
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

func TestParseHerdrEvent(t *testing.T) {
	// herdr serializes the envelope's event field in snake_case even though
	// manifests and HERDR_PLUGIN_EVENT use the dotted form.
	raw := `{"event":"worktree_opened","data":{"type":"worktree_opened","workspace_id":"w2",
"worktree":{"path":"/repos/grove-testing","branch":"feat/x","is_bare":false,"is_detached":false,
"is_prunable":false,"is_linked_worktree":true,"label":"grove-testing"},"already_open":false}}`

	got, err := parseHerdrEvent(raw)
	if err != nil {
		t.Fatalf("parseHerdrEvent() error = %v", err)
	}
	if got.Event != "worktree.opened" {
		t.Errorf("Event = %q, want worktree.opened", got.Event)
	}
	if got.CheckoutPath() != "/repos/grove-testing" {
		t.Errorf("CheckoutPath() = %q, want /repos/grove-testing", got.CheckoutPath())
	}
	if got.AlreadyOpen {
		t.Error("AlreadyOpen = true, want false")
	}
}

func TestParseHerdrEventAlreadyOpen(t *testing.T) {
	// Re-opening a workspace grove already knows about must be distinguishable,
	// so the hook can stay quiet instead of nagging on every focus.
	raw := `{"event":"worktree_opened","data":{"type":"worktree_opened","workspace_id":"w2",
"worktree":{"path":"/repos/grove-testing","label":"grove-testing","is_bare":false,
"is_detached":false,"is_prunable":false,"is_linked_worktree":true},"already_open":true}}`

	got, err := parseHerdrEvent(raw)
	if err != nil {
		t.Fatalf("parseHerdrEvent() error = %v", err)
	}
	if !got.AlreadyOpen {
		t.Error("AlreadyOpen = false, want true")
	}
}

func TestNormalizeHerdrEventName(t *testing.T) {
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
