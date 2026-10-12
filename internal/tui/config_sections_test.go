package tui

import (
	"strings"
	"testing"

	"charm.land/huh/v2"

	"github.com/lost-in-the/grove/internal/config"
)

// Each section is its own form holding only that section's fields (spec 11).
func TestConfig_FormPerSection(t *testing.T) {
	m := openConfigWith(t, 100, 30, nil)
	s := m.configState
	for tab := ConfigTab(0); tab < ConfigTabCount; tab++ {
		f := s.Forms[tab]
		if f == nil {
			t.Fatalf("section %s has no form", tabName(tab))
		}
		view := stripAnsi(f.View())
		for other := ConfigTab(0); other < ConfigTabCount; other++ {
			for _, field := range s.Fields[other] {
				if got := strings.Contains(view, field.Label); got != (other == tab) {
					t.Errorf("section %s form shows %s: %v", tabName(tab), field.Label, got)
				}
			}
		}
	}
	if focusedKey(m) != "project_name" {
		t.Errorf("focused %q, want project_name", focusedKey(m))
	}
}

func TestConfigField_LoadedIsTheDiskValue(t *testing.T) {
	cfg := config.LoadDefaults()
	cfg.ProjectName = "on-disk"
	off := false
	cfg.Plugins.Docker.Enabled = &off
	fields := populateConfigFields(cfg)
	for _, f := range append(fields[ConfigTabGeneral], fields[ConfigTabPlugins]...) {
		if f.Loaded != f.Value {
			t.Errorf("%s: loaded %q, value %q; they start equal", f.Key, f.Loaded, f.Value)
		}
	}
	if fields[ConfigTabGeneral][0].Loaded != "on-disk" || fields[ConfigTabPlugins][0].Loaded != "false" {
		t.Errorf("loaded values do not come from the config: %q, %q",
			fields[ConfigTabGeneral][0].Loaded, fields[ConfigTabPlugins][0].Loaded)
	}
}

// enter on a section's last field moves to the next section; the finished
// form is rebuilt so the section can be revisited. On the last section,
// enter closes when clean and prompts when there are changes.
func TestConfig_CompletingSectionMovesOn(t *testing.T) {
	m := openConfigWith(t, 100, 30, nil)
	m = sendConfigKeys(t, m, "enter", "enter", "enter")
	s := m.configState
	if s.Tab != ConfigTabBehavior {
		t.Fatalf("after general's last field: section %s, want Behavior", tabName(s.Tab))
	}
	if s.Forms[ConfigTabGeneral].State != huh.StateNormal {
		t.Error("the finished general form should be rebuilt")
	}

	m.configState.Tab = ConfigTabProtection
	m = sendConfigKeys(t, m, "enter", "enter")
	if m.activeView() != ViewDashboard {
		t.Errorf("clean, finishing the last section: view %v, want closed", m.activeView())
	}

	m = openConfigWith(t, 100, 30, nil)
	m.configState.Tab = ConfigTabProtection
	m = sendConfigKeys(t, m, "x", "enter", "enter")
	if m.activeView() != ViewConfig || !m.configState.Confirming {
		t.Errorf("with a change, finishing the last section should prompt: view %v", m.activeView())
	}
}
