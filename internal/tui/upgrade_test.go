package tui

import (
	"strings"
	"testing"
)

// The badge shows in the header only when update notices are allowed; the
// footer no longer carries one (spec 12).
func TestDashboard_UpgradeBadgeOnlyWhenNotifying(t *testing.T) {
	m := newTestModel(withItems(3), withSize(100, 24))
	m.updateLatestVersion, m.updateNotify = "99.0.0", true
	view := stripAnsi(m.renderDashboard())
	lines := strings.Split(view, "\n")
	if !strings.Contains(lines[0], "⭡99.0.0") {
		t.Errorf("header should show the badge: %q", lines[0])
	}
	if strings.Contains(view, "press u") || strings.Count(view, "99.0.0") != 1 {
		t.Errorf("the footer badge should be gone:\n%s", view)
	}

	m.updateNotify = false
	if view := stripAnsi(m.renderDashboard()); strings.Contains(view, "99.0.0") {
		t.Errorf("with notices off no badge should show:\n%s", view)
	}
}
