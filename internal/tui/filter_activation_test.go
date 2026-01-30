package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func TestListFilterActivation(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))

	if m.list.FilterState() != list.Unfiltered {
		t.Fatalf("expected Unfiltered, got %d", m.list.FilterState())
	}

	// Press / to activate filter
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = result.(Model)

	if m.list.FilterState() != list.Filtering {
		t.Fatalf("expected Filtering after /, got %d", m.list.FilterState())
	}

	// Cmd should be non-nil (cursor blink, etc.)
	if cmd == nil {
		t.Log("warning: no cmd returned after / (expected cursor blink)")
	}

	// Type 'f' — should stay in filtering mode, not trigger any dashboard handler
	result, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m = result.(Model)

	if m.list.FilterState() != list.Filtering {
		t.Fatalf("expected Filtering after typing 'f', got %d", m.list.FilterState())
	}
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}

	// The filter cmd from list should be returned for the runtime to execute
	if cmd == nil {
		t.Error("expected non-nil cmd from filter input (filter match computation)")
	}
}

func TestDefaultCaseForwardsToList(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))

	// Send an arbitrary non-key message — should be forwarded to the list
	// via the default case without panicking
	type customMsg struct{}
	result, _ := m.Update(customMsg{})
	m = result.(Model)

	// Should still be in dashboard, no crash
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after unknown msg, got %d", m.activeView)
	}
}

func TestListFilterRendersInStackedLayout(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))

	// List should have enough height for items + filter bar
	listH := m.list.Height()
	t.Logf("List height: %d, items: %d", listH, len(m.list.Items()))
	if listH < 5 {
		t.Errorf("list height %d too small for filter UI", listH)
	}
}
