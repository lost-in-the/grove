package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
)

func TestSortModeString(t *testing.T) {
	tests := []struct {
		mode SortMode
		want string
	}{
		{SortByName, "name"},
		{SortByLastAccessed, "recent"},
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
	if mode != SortByLastAccessed {
		t.Errorf("expected SortByLastAccessed, got %d", mode)
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
	sorted := sortWorktreeItems(items, SortByName)
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

func TestSortByLastAccessed(t *testing.T) {
	now := time.Now()
	items := []list.Item{
		WorktreeItem{ShortName: "old", LastAccessed: now.Add(-2 * time.Hour)},
		WorktreeItem{ShortName: "new", LastAccessed: now},
		WorktreeItem{ShortName: "root", IsMain: true, LastAccessed: now.Add(-1 * time.Hour)},
	}
	sorted := sortWorktreeItems(items, SortByLastAccessed)
	names := make([]string, len(sorted))
	for i, item := range sorted {
		names[i] = item.(WorktreeItem).ShortName
	}
	if names[0] != "root" {
		t.Errorf("expected root first, got %q", names[0])
	}
	if names[1] != "new" {
		t.Errorf("expected new second (most recent), got %q", names[1])
	}
	if names[2] != "old" {
		t.Errorf("expected old third, got %q", names[2])
	}
}

func TestSortByDirtyFirst(t *testing.T) {
	items := []list.Item{
		WorktreeItem{ShortName: "clean-b", IsDirty: false},
		WorktreeItem{ShortName: "dirty-a", IsDirty: true},
		WorktreeItem{ShortName: "clean-a", IsDirty: false},
		WorktreeItem{ShortName: "root", IsMain: true, IsDirty: false},
	}
	sorted := sortWorktreeItems(items, SortByDirtyFirst)
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
	sorted := sortWorktreeItems(items, SortByName)
	// Original should be unchanged
	if items[0].(WorktreeItem).ShortName != "b" {
		t.Error("original slice was modified")
	}
	if sorted[0].(WorktreeItem).ShortName != "a" {
		t.Error("sorted slice not in expected order")
	}
}
