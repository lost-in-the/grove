package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestGStrip_ShowsActions(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "g")
	out := ansi.Strip(m.viewString())
	for _, want := range []string{" g goto", "g top of list", "p pull requests", "i issues", "c config", "u upgrade grove", "esc close"} {
		if !strings.Contains(out, want) {
			t.Errorf("g strip missing %q:\n%s", want, out)
		}
	}
}

func TestGStrip_RunsAliases(t *testing.T) {
	for k, want := range map[string]ActiveView{"c": ViewConfig, "i": ViewIssues, "p": ViewPRs} {
		m := newTestModel(withItems(3), withSize(120, 40))
		m = sendKey(m, "g")
		m = sendKey(m, k)
		if m.activeView() != want {
			t.Errorf("g %s: view %v, want %v", k, m.activeView(), want)
		}
	}
}

func TestGStrip_GGStillTopAndUnmappedCloses(t *testing.T) {
	m := newTestModel(withItems(5), withSize(120, 40))
	m.list.Select(3)
	m = sendKey(m, "g")
	m = sendKey(m, "g")
	if m.list.Index() != 0 || m.pendingG {
		t.Errorf("gg: index %d pendingG %v", m.list.Index(), m.pendingG)
	}
	m.list.Select(3)
	m = sendKey(m, "g")
	m = sendKey(m, "x")
	if m.list.Index() != 3 || m.pendingG || m.stripHeight() != 0 {
		t.Errorf("g x should only close the strip (index %d)", m.list.Index())
	}
}

func TestGStrip_ListHeightRecovers(t *testing.T) {
	m := newTestModel(withItems(5), withSize(120, 40))
	h := m.list.Height()
	m = sendKey(m, "g")
	if m.list.Height() >= h {
		t.Errorf("list height %d with the g strip open, want less than %d", m.list.Height(), h)
	}
	m = sendKey(m, "esc")
	if m.list.Height() != h {
		t.Errorf("list height %d after closing, want %d", m.list.Height(), h)
	}
}

func TestGStrip_UpgradeWithoutUpdateFlashes(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "g")
	m = sendKey(m, "u")
	if m.activeView() != ViewDashboard || m.toast.Message() != "grove is up to date" {
		t.Errorf("g u: view %v toast %q", m.activeView(), m.toast.Message())
	}
}

func TestStrip_FitsWidth(t *testing.T) {
	for _, k := range []string{"g", "m"} {
		m := newTestModel(withItems(3), withSize(44, 24))
		m.projectRoot = prefsRoot(t)
		m = sendKey(m, k)
		for i, line := range strings.Split(m.viewString(), "\n") {
			if w := lipgloss.Width(line); w > 44 {
				t.Errorf("%s strip: line %d is %d cells", k, i, w)
			}
		}
	}
}

func TestGStrip_UpgradeWithUpdateOpensModal(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.updateOverlay = NewUpdateOverlay()
	m.updateLatestVersion, m.updateLatestURL = "99.0.0", "https://example.com/release"
	m = sendKey(m, "g")
	m = sendKey(m, "u")
	if m.activeView() != ViewUpdate {
		t.Errorf("g u with an update cached: view %v, want the update modal", m.activeView())
	}
}

// Opening a strip shrinks the list; the selected row stays on screen.
func TestGStrip_SelectedRowStaysVisible(t *testing.T) {
	m := newTestModel(withItems(10), withSize(80, 16))
	m.updateLayout()
	m.moveCursorTo(9)
	m = sendKey(m, "g")
	if !strings.Contains(ansi.Strip(m.viewString()), "❯ ● dev") && !strings.Contains(ansi.Strip(m.viewString()), "❯ ○ dev") {
		t.Errorf("selected row dev scrolled out of view with the strip open:\n%s", ansi.Strip(m.viewString()))
	}
}
