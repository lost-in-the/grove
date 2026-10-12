package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/exp/golden"

	"github.com/lost-in-the/grove/internal/version"
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

// With nothing newer cached, and no overlay built yet, g u still opens the
// overlay and says grove is up to date (spec 12).
func TestGU_UpToDateOpensOverlay(t *testing.T) {
	m := newTestModel(withItems(3), withSize(100, 24))
	m = sendKey(sendKey(m, "g"), "u")
	if m.activeView() != ViewUpdate || m.toast.Current != nil {
		t.Fatalf("g u: view %v, toast %v; want the overlay and no toast", m.activeView(), m.toast.Current)
	}
	view := stripAnsi(m.viewString())
	if !strings.Contains(view, "grove is up to date") || strings.Contains(view, "Brew") {
		t.Errorf("want the up-to-date state without install commands:\n%s", view)
	}
}

func TestGolden_Upgrade(t *testing.T) {
	// The overlay shows the running version; pin it so a release does not
	// change the goldens.
	was := version.Version
	version.Version = "1.2.3"
	t.Cleanup(func() { version.Version = was })
	for name, opts := range map[string][]testOpt{
		"available":  {withItems(3), withUpdateCached()},
		"up_to_date": {withItems(3)},
	} {
		t.Run(name, func(t *testing.T) {
			m := goldenModel(t, sizeStandard, opts...)
			m = sendKey(sendKey(m, "g"), "u")
			golden.RequireEqual(t, []byte(m.viewString()))
		})
	}
}

// Find reaches the overlay by "update" as well as "upgrade": the overlay,
// the cache, and the docs all say update.
func TestOmni_UpdateFindsUpgrade(t *testing.T) {
	groups := omniResults("update", omniScopeActions, omniSource{Actions: omniActionsFrom(dashboardActions())})
	found := false
	for _, g := range groups {
		for _, it := range g.Items {
			found = found || it.Ref == "goto.upgrade"
		}
	}
	if !found {
		t.Errorf("Find \"update\" should offer upgrade grove: %+v", groups)
	}
}
