package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
)

func TestStatusBar_SortModeAlwaysVisible(t *testing.T) {
	tests := []struct {
		name     string
		sortMode SortMode
		wantText string
	}{
		{"default name sort", SortByName, "name"},
		{"recent sort", SortByLastAccessed, "recent"},
		{"dirty sort", SortByDirtyFirst, "dirty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(withItems(3), withSize(100, 24))
			m.sortMode = tt.sortMode
			bar := stripAnsi(m.renderStatusBar())
			if !strings.Contains(bar, tt.wantText) {
				t.Errorf("status bar missing sort mode %q in:\n%s", tt.wantText, bar)
			}
			if !strings.Contains(bar, "\u2195") { // ↕
				t.Errorf("status bar missing sort icon in:\n%s", bar)
			}
		})
	}
}

func TestStatusBar_ViewModeAlwaysVisible(t *testing.T) {
	tests := []struct {
		name        string
		compactMode bool
		wantText    string
	}{
		{"detailed mode", false, "detailed"},
		{"compact mode", true, "compact"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(withItems(3), withSize(100, 24))
			m.compactMode = tt.compactMode
			bar := stripAnsi(m.renderStatusBar())
			if !strings.Contains(bar, tt.wantText) {
				t.Errorf("status bar missing view mode %q in:\n%s", tt.wantText, bar)
			}
			if !strings.Contains(bar, "\u2630") { // ☰
				t.Errorf("status bar missing view icon in:\n%s", bar)
			}
		})
	}
}

func TestStatusBar_FilterBadgeWhenActive(t *testing.T) {
	m := newTestModel(withItems(5), withSize(100, 24))

	// Before filtering, no filter badge
	bar := stripAnsi(m.renderStatusBar())
	if strings.Contains(bar, "\U0001f50d") { // 🔍
		t.Errorf("filter badge should not appear when unfiltered:\n%s", bar)
	}

	// Simulate filter applied
	m.list.SetFilterState(list.FilterApplied)
	m.list.SetFilterText("feat")

	bar = stripAnsi(m.renderStatusBar())
	if !strings.Contains(bar, "feat") {
		t.Errorf("status bar missing filter text in:\n%s", bar)
	}
	if !strings.Contains(bar, "\U0001f50d") { // 🔍
		t.Errorf("status bar missing filter icon in:\n%s", bar)
	}
}

func TestStatusBar_FilterBadgeShowsMatchCount(t *testing.T) {
	m := newTestModel(withItems(5), withSize(100, 24))
	m.list.SetFilterState(list.FilterApplied)
	m.list.SetFilterText("root")

	bar := stripAnsi(m.renderStatusBar())

	// Should contain the total count at minimum
	// The exact visible count depends on the filter engine, but the format
	// should include [N/5] where 5 is the total item count.
	if !strings.Contains(bar, "/5]") {
		t.Errorf("status bar missing total count in filter badge:\n%s", bar)
	}
}

func TestStatusBar_FilterBadgeDuringFiltering(t *testing.T) {
	m := newTestModel(withItems(5), withSize(100, 24))
	m.list.SetFilterText("te")
	m.list.SetFilterState(list.Filtering)

	bar := stripAnsi(m.renderStatusBar())
	// During active filtering, badge should also appear
	if !strings.Contains(bar, "\U0001f50d") { // 🔍
		t.Errorf("filter badge should appear during filtering:\n%s", bar)
	}
}

func TestStatusBar_NarrowWidth_AbbreviatedLabels(t *testing.T) {
	m := newTestModel(withItems(3), withSize(60, 24))
	m.sortMode = SortByLastAccessed
	m.compactMode = true

	bar := stripAnsi(m.renderStatusBar())

	// At narrow widths (< 80), should show icons without full text labels
	// Sort icon should be present
	if !strings.Contains(bar, "\u2195") { // ↕
		t.Errorf("narrow bar missing sort icon:\n%s", bar)
	}
	// View icon should be present
	if !strings.Contains(bar, "\u2630") { // ☰
		t.Errorf("narrow bar missing view icon:\n%s", bar)
	}
	// Full labels should be abbreviated (no "recent", no "compact")
	if strings.Contains(bar, "recent") {
		t.Errorf("narrow bar should abbreviate sort label:\n%s", bar)
	}
	if strings.Contains(bar, "compact") {
		t.Errorf("narrow bar should abbreviate view label:\n%s", bar)
	}
}

func TestStatusBar_WideWidth_FullLabels(t *testing.T) {
	m := newTestModel(withItems(3), withSize(100, 24))
	m.sortMode = SortByLastAccessed
	m.compactMode = false

	bar := stripAnsi(m.renderStatusBar())
	if !strings.Contains(bar, "recent") {
		t.Errorf("wide bar should show full sort label:\n%s", bar)
	}
	if !strings.Contains(bar, "detailed") {
		t.Errorf("wide bar should show full view label:\n%s", bar)
	}
}

func TestStatusBar_ToastStillShows(t *testing.T) {
	m := newTestModel(withItems(3), withSize(100, 24))
	m.toast.Show(NewToast("Worktree deleted", ToastSuccess))

	bar := stripAnsi(m.renderStatusBar())
	if !strings.Contains(bar, "Worktree deleted") {
		t.Errorf("status bar should still show toast:\n%s", bar)
	}
}

func TestStatusBar_ProjectNameAndCount(t *testing.T) {
	m := newTestModel(withItems(3), withSize(100, 24))
	bar := stripAnsi(m.renderStatusBar())

	if !strings.Contains(bar, "test-project") {
		t.Errorf("status bar missing project name:\n%s", bar)
	}
	if !strings.Contains(bar, "3 worktrees") {
		t.Errorf("status bar missing worktree count:\n%s", bar)
	}
}
