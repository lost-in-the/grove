package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
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
// grove does not have fails only at runtime, in a popup, on the user's machine.
// The manifest shipped `["grove", "tui"]` for the dashboard pane while the TUI
// is reached through bare `grove` — `grove tui` exits with
// `unknown command "tui"`, so the plugin's headline feature never worked.
// Parsing the manifest proves it is well-formed, not that it is callable.
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
