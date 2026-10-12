package tui

import (
	"strings"
	"testing"
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

func TestConfig_SaveWritesOnlyChanged(t *testing.T) {
	m := openConfigWith(t, 100, 30, nil)
	m = sendConfigKeys(t, m, "x")
	got := configUpdates(m.configState)
	if len(got) != 1 || got["project_name"] != `"x"` {
		t.Errorf("updates %v, want only project_name", got)
	}
}
