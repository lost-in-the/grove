package tui

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/lost-in-the/grove/internal/config"
)

// The panel keeps the field set it had before the rebuild.
func TestPopulateConfigFields_FieldSet(t *testing.T) {
	want := [ConfigTabCount][]string{
		ConfigTabGeneral:    {"project_name", "projects_dir", "default_base_branch"},
		ConfigTabBehavior:   {"switch.dirty_handling", "switch.container_switch", "session.open_in", "tmux.mode", "naming.pattern"},
		ConfigTabPlugins:    {"plugins.docker.enabled", "plugins.docker.auto_start", "plugins.docker.auto_stop"},
		ConfigTabProtection: {"protection.protected", "protection.immutable"},
	}
	fields := populateConfigFields(config.LoadDefaults())
	for tab := range ConfigTabCount {
		var got []string
		for _, f := range fields[tab] {
			got = append(got, f.Key)
		}
		if !slices.Equal(got, want[tab]) {
			t.Errorf("%s: %v, want %v", tabName(tab), got, want[tab])
		}
	}
}

// Every field says what it is and what each choice does, so nothing has
// to be remembered or looked up.
func TestPopulateConfigFields_EveryFieldExplainsItself(t *testing.T) {
	for _, fields := range populateConfigFields(config.LoadDefaults()) {
		for _, f := range fields {
			if f.Label == "" || f.Description == "" || f.Default == "" {
				t.Errorf("%s: label %q, description %q, default %q; all are needed", f.Key, f.Label, f.Description, f.Default)
			}
			if f.Label == f.Key || strings.Contains(f.Label, "_") {
				t.Errorf("%s: label %q should be plain words, not the key", f.Key, f.Label)
			}
			switch f.Type {
			case ConfigEnum, ConfigBool:
				defaults := 0
				for _, o := range f.Options {
					if o.Meaning == "" {
						t.Errorf("%s: option %q has no meaning", f.Key, o.Value)
					}
					if o.Value == f.Default {
						defaults++
					}
				}
				if defaults != 1 {
					t.Errorf("%s: default %q matches %d options, want 1", f.Key, f.Default, defaults)
				}
			case ConfigString, ConfigList:
				if len(f.Notes) == 0 {
					t.Errorf("%s: a text field needs notes on what to type", f.Key)
				}
			}
		}
	}
}

func optionValues(f ConfigField) []string {
	var out []string
	for _, o := range f.Options {
		out = append(out, o.Value)
	}
	return out
}

// The choices are the ones config loading accepts, in the order grove
// documents them.
func TestPopulateConfigFields_Choices(t *testing.T) {
	want := map[string][]string{
		"switch.dirty_handling":     config.DirtyHandlingValues,
		"switch.container_switch":   {"off", "prompt", "auto"},
		"session.open_in":           {config.OpenInSession, config.OpenInHere},
		"tmux.mode":                 {"auto", "manual", "off"},
		"plugins.docker.enabled":    {boolTrue, boolFalse},
		"plugins.docker.auto_start": {boolTrue, boolFalse},
		"plugins.docker.auto_stop":  {boolTrue, boolFalse},
	}
	defaults := config.LoadDefaults()
	for _, fields := range populateConfigFields(defaults) {
		for _, f := range fields {
			w, ok := want[f.Key]
			if !ok {
				continue
			}
			if got := optionValues(f); !slices.Equal(got, w) {
				t.Errorf("%s options %v, want %v", f.Key, got, w)
			}
			for _, v := range w {
				if err := f.validate(defaults, v); err != nil {
					t.Errorf("%s = %q does not validate: %v", f.Key, v, err)
				}
			}
		}
	}
}

// Each field's documented default is what a config with nothing set shows.
func TestPopulateConfigFields_DefaultsMatchTheDefaults(t *testing.T) {
	cfg := config.LoadDefaults()
	cfg.ProjectsDir = "~/projects"
	for _, fields := range populateConfigFields(cfg) {
		for _, f := range fields {
			switch f.Key {
			case "project_name", "protection.protected", "protection.immutable":
				continue // empty by default; the default text says what that means
			}
			if f.Value != f.Default {
				t.Errorf("%s: shows %q on the defaults, but its default reads %q", f.Key, f.Value, f.Default)
			}
		}
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

// A list is written as a TOML array of strings: the comma-separated text
// split, trimmed, and quoted, and an empty list as [].
func TestTOMLList(t *testing.T) {
	if got, want := tomlList(" main,staging , ,"), `["main", "staging"]`; got != want {
		t.Errorf("tomlList = %s, want %s", got, want)
	}
	if got := tomlList(" "); got != "[]" {
		t.Errorf("empty list = %s, want []", got)
	}
}
