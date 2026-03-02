package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
)

func BenchmarkViewDashboard(b *testing.B) {
	m := newTestModel(withItems(20), withSize(120, 40))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkViewDashboardWide(b *testing.B) {
	m := newTestModel(withItems(20), withSize(200, 50))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkViewHelp(b *testing.B) {
	m := newTestModel(withItems(5), withSize(120, 40))
	m = sendKey(m, "?")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkViewDeleteOverlay(b *testing.B) {
	m := newTestModel(withItems(5), withSize(120, 40))
	m = sendKey(m, "j")
	m = sendKey(m, "d")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkViewBulkOverlay(b *testing.B) {
	m := newTestModel(withItems(20), withSize(120, 40))
	m = sendKey(m, "a")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkSortWorktreeItems(b *testing.B) {
	items := makeTestItems(50)
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sortWorktreeItems(listItems, SortByDirtyFirst)
	}
}
