package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/lost-in-the/grove/internal/config"
)

func configHeaderLine(m Model) string {
	return strings.Split(stripAnsi(m.viewString()), "\n")[0]
}

// The unsaved count compares with the values loaded from disk, so undoing a
// change makes the editor clean again (spec 11).
func TestConfig_UnsavedCountAgainstLoaded(t *testing.T) {
	m := openConfigWith(t, 100, 30, nil)
	if h := configHeaderLine(m); strings.Contains(h, "unsaved") {
		t.Fatalf("freshly opened: %q", h)
	}
	m = sendConfigKeys(t, m, "x")
	if h := configHeaderLine(m); !strings.Contains(h, "● 1 unsaved") {
		t.Errorf("after one change: %q", h)
	}
	m = sendConfigKeys(t, m, "backspace")
	if h := configHeaderLine(m); strings.Contains(h, "unsaved") {
		t.Errorf("after undoing it: %q", h)
	}
	if m = sendConfigKeys(t, m, "esc"); m.activeView() != ViewDashboard {
		t.Errorf("esc on an undone change should close without a prompt: view %v", m.activeView())
	}
}

// A bool binds through configFormValues, not straight to its field, so
// toggling one must still count as unsaved, prompt on esc, and be saved
// alone.
func TestConfig_BoolToggleIsUnsaved(t *testing.T) {
	m := openConfigWith(t, 100, 30, nil)
	m = sendConfigKeys(t, m, "ctrl+n", "ctrl+n")
	if k := focusedKey(m); k != "plugins.docker.enabled" {
		t.Fatalf("focused %q, want plugins.docker.enabled", k)
	}
	m = sendConfigKeys(t, m, "l")
	if h := configHeaderLine(m); !strings.Contains(h, "● 1 unsaved") {
		t.Errorf("after toggling: %q", h)
	}
	got := configUpdates(m.configState)
	if len(got) != 1 || got["plugins.docker.enabled"] != "false" {
		t.Errorf("updates %v, want only plugins.docker.enabled = false", got)
	}
	m = sendConfigKeys(t, m, "esc")
	if m.activeView() != ViewConfig || !m.configState.Confirming {
		t.Errorf("esc after a toggle should ask to save: view %v", m.activeView())
	}
}

func TestConfig_SaveWritesOnlyChanged(t *testing.T) {
	m := openConfigWith(t, 100, 30, nil)
	m = sendConfigKeys(t, m, "x")
	got := configUpdates(m.configState)
	if len(got) != 1 || got["project_name"] != `"x"` {
		t.Errorf("updates %v, want only project_name", got)
	}
}

// A list field is written as a TOML array of strings: the comma-separated
// text split, trimmed, and quoted, and an empty list as [].
func TestConfigUpdates_ListsAreTOMLArrays(t *testing.T) {
	s := NewConfigState()
	s.Fields[ConfigTabProtection] = []ConfigField{
		{Key: "protection.protected", Value: " main,staging , ,", Loaded: "", Type: ConfigList},
		{Key: "protection.immutable", Value: " ", Loaded: "prod", Type: ConfigList},
	}
	got := configUpdates(s)
	if want := `["main", "staging"]`; got["protection.protected"] != want {
		t.Errorf("protected = %s, want %s", got["protection.protected"], want)
	}
	if want := `[]`; got["protection.immutable"] != want {
		t.Errorf("immutable = %s, want %s", got["protection.immutable"], want)
	}
}

// Strings and list items are written with escapes TOML 1.0 accepts, so any
// typed value decodes back to itself. The decoder also takes TOML 1.1's \x
// and \e, so the escapes are checked as well.
func TestTOMLString_RoundTrips(t *testing.T) {
	escape := regexp.MustCompile(`\\.`)
	for _, s := range []string{
		"plain", "", "bell\a", "del\x7f", "nul\x00", "vtab\v", "esc\x1b",
		"tab\t", "line\nbreak", `quote"`, `back\slash`, `back\x`, "café ✓",
	} {
		quoted := tomlString(s)
		for _, e := range escape.FindAllString(quoted, -1) {
			if !strings.Contains(`\"btnfru`, e[1:]) {
				t.Errorf("%q written as %s uses %s, not a TOML 1.0 escape", s, quoted, e)
			}
		}
		var got struct{ K string }
		if _, err := toml.Decode("k = "+quoted, &got); err != nil {
			t.Errorf("%q written as %s: %v", s, quoted, err)
		} else if got.K != s {
			t.Errorf("%q written as %s reads back as %q", s, quoted, got.K)
		}
	}
	var list struct{ K []string }
	if _, err := toml.Decode("k = "+tomlList("a\a, b"), &list); err != nil || !slices.Equal(list.K, []string{"a\a", "b"}) {
		t.Errorf("list reads back as %q, %v", list.K, err)
	}
}

// A saved protection list loads back: the written file is valid TOML.
func TestConfig_SavedListLoadsBack(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(project, ".grove"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)

	m := openConfigWith(t, 100, 30, nil)
	m.configState.Fields[ConfigTabProtection][0].Value = "main, staging"
	if err := config.SetProjectConfigValues(configUpdates(m.configState)); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		data, _ := os.ReadFile(filepath.Join(project, ".grove", "config.toml"))
		t.Fatalf("load after save: %v\n%s", err, data)
	}
	for _, name := range []string{"main", "staging"} {
		if !slices.Contains(cfg.Protection.Protected, name) {
			t.Errorf("protected %v lacks %q", cfg.Protection.Protected, name)
		}
	}
}
