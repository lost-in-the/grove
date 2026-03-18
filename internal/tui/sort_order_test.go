package tui

import (
	"testing"
)

// TestSortOrderActuallyChanges verifies that toggling sort mode
// actually changes the order of items in the list, not just the label.
func TestSortOrderActuallyChanges(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))

	getOrder := func() []string {
		items := m.list.Items()
		names := make([]string, len(items))
		for i, li := range items {
			names[i] = li.(WorktreeItem).ShortName
		}
		return names
	}

	getVisibleOrder := func() []string {
		items := m.list.VisibleItems()
		names := make([]string, len(items))
		for i, li := range items {
			names[i] = li.(WorktreeItem).ShortName
		}
		return names
	}

	nameOrder := getOrder()
	visibleNameOrder := getVisibleOrder()
	t.Logf("initial items: %v", nameOrder)
	t.Logf("initial visible: %v", visibleNameOrder)

	// Toggle to recent
	m = sendKey(m, "o")
	if m.sortMode != SortByLastAccessed {
		t.Fatalf("expected SortByLastAccessed, got %d", m.sortMode)
	}
	recentOrder := getOrder()
	visibleRecentOrder := getVisibleOrder()
	t.Logf("after 1st toggle items: %v", recentOrder)
	t.Logf("after 1st toggle visible: %v", visibleRecentOrder)

	// Toggle to dirty
	m = sendKey(m, "o")
	if m.sortMode != SortByDirtyFirst {
		t.Fatalf("expected SortByDirtyFirst, got %d", m.sortMode)
	}
	dirtyOrder := getOrder()
	visibleDirtyOrder := getVisibleOrder()
	t.Logf("after 2nd toggle items: %v", dirtyOrder)
	t.Logf("after 2nd toggle visible: %v", visibleDirtyOrder)

	// Toggle back to name
	m = sendKey(m, "o")
	if m.sortMode != SortByName {
		t.Fatalf("expected SortByName, got %d", m.sortMode)
	}
	nameOrder2 := getOrder()
	visibleNameOrder2 := getVisibleOrder()
	t.Logf("after 3rd toggle items: %v", nameOrder2)
	t.Logf("after 3rd toggle visible: %v", visibleNameOrder2)

	// Verify that dirty order differs from recent order
	if dirtyOrder[1] == recentOrder[1] {
		t.Errorf("dirty and recent orders should differ at index 1: dirty=%v, recent=%v", dirtyOrder, recentOrder)
	}

	// Verify items and visible items match (no stale filter)
	for i, n := range dirtyOrder {
		if visibleDirtyOrder[i] != n {
			t.Errorf("visible items don't match items at index %d: items=%v, visible=%v", i, dirtyOrder, visibleDirtyOrder)
			break
		}
	}

	// Verify name order is alphabetical (after main)
	expectedNameOrder := []string{"root", "feature-auth", "fix-bug", "refactor", "testing"}
	for i, n := range expectedNameOrder {
		if nameOrder2[i] != n {
			t.Errorf("name order wrong at index %d: expected=%v, got=%v", i, expectedNameOrder, nameOrder2)
			break
		}
	}
}
