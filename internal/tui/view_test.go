package tui

import (
	"strings"
	"testing"
)

func TestViewLoadingState(t *testing.T) {
	m := newTestModel(withLoading(), withSize(80, 24))
	v := m.View()
	if !strings.Contains(v, "Loading") {
		t.Error("expected loading text in view")
	}
}

func TestViewNotReady(t *testing.T) {
	m := newTestModel()
	m.ready = false
	v := m.View()
	if v != "loading..." {
		t.Errorf("expected 'loading...' when not ready, got %q", v)
	}
}

func TestViewDashboardWithItems(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	v := m.View()
	if v == "" {
		t.Error("expected non-empty dashboard view")
	}
	// Should contain project name
	if !strings.Contains(v, "test-project") {
		t.Error("expected project name in status bar")
	}
	// Should contain worktree count
	if !strings.Contains(v, "3 worktrees") {
		t.Error("expected worktree count in status bar")
	}
}

func TestViewDashboardEmpty(t *testing.T) {
	m := newTestModel(withItems(0), withSize(80, 24))
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view even with no items")
	}
}

func TestViewHelpOverlay(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "?")
	v := m.View()
	if !strings.Contains(v, "Keybindings") {
		t.Error("expected 'Keybindings' in help view")
	}
	if !strings.Contains(v, "Navigation") {
		t.Error("expected 'Navigation' section in help view")
	}
}

func TestViewDeleteOverlay(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "j") // move to non-main
	m = sendKey(m, "d")
	v := m.View()
	if !strings.Contains(v, "Delete Worktree") {
		t.Error("expected 'Delete Worktree' in delete overlay")
	}
}

func TestViewCreateOverlay(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "n")
	v := m.View()
	if !strings.Contains(v, "New Worktree") {
		t.Error("expected 'New Worktree' in create overlay")
	}
}

func TestViewBulkOverlay(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))
	m = sendKey(m, "a")
	v := m.View()
	if !strings.Contains(v, "Bulk Delete") {
		t.Error("expected 'Bulk Delete' in bulk overlay")
	}
}

func TestViewSideBySideLayout(t *testing.T) {
	m := newTestModel(withItems(3), withSize(140, 40))
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view for wide terminal")
	}
}

func TestViewStackedLayout(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	v := m.View()
	// Stacked layout should contain a separator
	if !strings.Contains(v, "─") {
		t.Error("expected separator in stacked layout")
	}
}

func TestViewStatusBarWithSort(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "o") // cycle to "recent"
	v := m.renderStatusBar()
	if !strings.Contains(v, "recent") {
		t.Error("expected sort mode 'recent' in status bar")
	}
}

func TestViewStatusBarWithToast(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendMsg(m, worktreeDeletedMsg{name: "testing", err: nil})
	v := m.renderStatusBar()
	if !strings.Contains(v, "testing") {
		t.Error("expected toast with deleted name in status bar")
	}
}
