package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
)

func TestSortModeString(t *testing.T) {
	tests := []struct {
		mode SortMode
		want string
	}{
		{SortByName, "name"},
		{SortByAge, "age"},
		{SortByDirtyFirst, "dirty"},
		{SortMode(99), "name"}, // default case
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("SortMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestSortModeNext(t *testing.T) {
	mode := SortByName
	mode = mode.Next()
	if mode != SortByAge {
		t.Errorf("expected SortByAge, got %d", mode)
	}
	mode = mode.Next()
	if mode != SortByDirtyFirst {
		t.Errorf("expected SortByDirtyFirst, got %d", mode)
	}
	mode = mode.Next()
	if mode != SortByName {
		t.Errorf("expected SortByName after full cycle, got %d", mode)
	}
}

func TestSortByName(t *testing.T) {
	items := []list.Item{
		WorktreeItem{ShortName: "zebra"},
		WorktreeItem{ShortName: "alpha"},
		WorktreeItem{ShortName: "root", IsMain: true},
	}
	sorted := sortWorktreeItems(items, SortByName, false)
	names := make([]string, len(sorted))
	for i, item := range sorted {
		names[i] = item.(WorktreeItem).ShortName
	}
	// Root worktree should be first, then alphabetical
	if names[0] != "root" {
		t.Errorf("expected root first, got %q", names[0])
	}
	if names[1] != "alpha" {
		t.Errorf("expected alpha second, got %q", names[1])
	}
	if names[2] != "zebra" {
		t.Errorf("expected zebra third, got %q", names[2])
	}
}

func TestSortByAge(t *testing.T) {
	items := []list.Item{
		WorktreeItem{ShortName: "old", CommitTime: testNow.Add(-2 * time.Hour)},
		WorktreeItem{ShortName: "new", CommitTime: testNow},
		WorktreeItem{ShortName: "root", IsMain: true, CommitTime: testNow.Add(-1 * time.Hour)},
		WorktreeItem{ShortName: "also-new", CommitTime: testNow},
	}
	got := names(sortWorktreeItems(items, SortByAge, false))
	want := []string{"root", "also-new", "new", "old"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("age order = %v, want %v (main first, newest commit next, ties by alias)", got, want)
	}
}

// O reverses every mode; the main worktree stays first.
func TestSortReverse(t *testing.T) {
	items := []list.Item{
		WorktreeItem{ShortName: "b", IsDirty: true, CommitTime: testNow.Add(-time.Hour)},
		WorktreeItem{ShortName: "root", IsMain: true},
		WorktreeItem{ShortName: "a", CommitTime: testNow},
		WorktreeItem{ShortName: "c", CommitTime: testNow.Add(-2 * time.Hour)},
	}
	for mode, want := range map[SortMode][]string{
		SortByName:       {"root", "c", "b", "a"},
		SortByAge:        {"root", "c", "b", "a"},
		SortByDirtyFirst: {"root", "c", "a", "b"},
	} {
		got := names(sortWorktreeItems(items, mode, true))
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s reversed = %v, want %v", mode, got, want)
		}
	}
}

func names(items []list.Item) []string {
	out := make([]string, len(items))
	for i, li := range items {
		out[i] = li.(WorktreeItem).ShortName
	}
	return out
}

func TestSortCycleIsNameAgeDirty(t *testing.T) {
	if SortByName.Next() != SortByAge || SortByAge.Next() != SortByDirtyFirst || SortByDirtyFirst.Next() != SortByName {
		t.Error("o should cycle name, age, dirty")
	}
}

func TestDashboard_OReversesSort(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "O")
	if !m.sortReversed || !strings.Contains(stripAnsi(m.buildDashboardHeader().View(120)), "⬍ name ↑") {
		t.Errorf("O did not reverse the sort (reversed=%v)", m.sortReversed)
	}
	if got := names(m.list.Items())[0]; got != "root" {
		t.Errorf("main worktree should stay first, got %q", got)
	}
	m = sendKey(m, "O")
	if m.sortReversed {
		t.Error("second O should restore the direction")
	}
}

func TestSortByDirtyFirst(t *testing.T) {
	items := []list.Item{
		WorktreeItem{ShortName: "clean-b", IsDirty: false},
		WorktreeItem{ShortName: "dirty-a", IsDirty: true},
		WorktreeItem{ShortName: "clean-a", IsDirty: false},
		WorktreeItem{ShortName: "root", IsMain: true, IsDirty: false},
	}
	sorted := sortWorktreeItems(items, SortByDirtyFirst, false)
	names := make([]string, len(sorted))
	for i, item := range sorted {
		names[i] = item.(WorktreeItem).ShortName
	}
	if names[0] != "root" {
		t.Errorf("expected root first, got %q", names[0])
	}
	if names[1] != "dirty-a" {
		t.Errorf("expected dirty-a second, got %q", names[1])
	}
}

func TestSortPreservesOriginal(t *testing.T) {
	items := []list.Item{
		WorktreeItem{ShortName: "b"},
		WorktreeItem{ShortName: "a"},
	}
	sorted := sortWorktreeItems(items, SortByName, false)
	// Original should be unchanged
	if items[0].(WorktreeItem).ShortName != "b" {
		t.Error("original slice was modified")
	}
	if sorted[0].(WorktreeItem).ShortName != "a" {
		t.Error("sorted slice not in expected order")
	}
}
