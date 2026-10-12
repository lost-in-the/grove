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

func withUpdateCached() testOpt {
	return func(m *Model) {
		m.updateOverlay = NewUpdateOverlay()
		m.updateLatestVersion, m.updateLatestURL, m.updateNotify = "99.0.0", "https://example.com/release", true
	}
}

// Bare u means nothing (spec N9): g u and Find reach the overlay.
func TestDashboard_BareUOpensNothing(t *testing.T) {
	m := newTestModel(withItems(3), withSize(100, 24), withUpdateCached())
	m = sendKey(m, "u")
	if m.activeView() != ViewDashboard || m.toast.Current != nil {
		t.Errorf("u: view %v, toast %v; want the dashboard unchanged", m.activeView(), m.toast.Current)
	}
}

func TestUpgradeOverlay_UDoesNotClose(t *testing.T) {
	m := newTestModel(withItems(3), withSize(100, 24), withUpdateCached())
	m = sendKey(sendKey(m, "g"), "u")
	if m.activeView() != ViewUpdate {
		t.Fatalf("g u: view %v, want the upgrade overlay", m.activeView())
	}
	if m = sendKey(m, "u"); m.activeView() != ViewUpdate {
		t.Error("u closed the overlay; only esc should")
	}
	if m = sendKey(m, "esc"); m.activeView() != ViewDashboard {
		t.Errorf("esc: view %v, want the dashboard", m.activeView())
	}
	if view := stripAnsi(m.updateOverlay.View(100, 24)); !strings.Contains(view, "esc close") || strings.Contains(view, "esc/u") {
		t.Errorf("the key line should read esc close:\n%s", view)
	}
}
