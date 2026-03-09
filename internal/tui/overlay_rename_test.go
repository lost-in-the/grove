package tui

import (
	"strings"
	"testing"
)

func TestRenameOverlay_OpenClose(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	// Move to non-main item (index 1)
	m = sendKey(m, "j")
	m = sendKey(m, "R")
	if m.activeView != ViewRename {
		t.Errorf("expected ViewRename, got %d", m.activeView)
	}
	if m.renameState == nil {
		t.Fatal("expected renameState to be set")
	}

	// Esc should close
	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after esc, got %d", m.activeView)
	}
	if m.renameState != nil {
		t.Error("expected renameState nil after close")
	}
}

func TestRenameOverlay_MainWorktreeBlocked(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	// Item 0 is main — R should not open rename
	m = sendKey(m, "R")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard for main worktree, got %d", m.activeView)
	}
	if m.renameState != nil {
		t.Error("expected renameState nil for main worktree")
	}
}

func TestRenameOverlay_NilState(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewRename
	m.renameState = nil
	m = sendKey(m, "enter")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard for nil state, got %d", m.activeView)
	}
}

func TestRenameOverlay_EmptyNameBlocked(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "j")
	m = sendKey(m, "R")
	if m.renameState == nil {
		t.Fatal("expected renameState to be set")
	}

	// Press enter without typing a name
	m = sendKey(m, "enter")
	if m.renameState.Error == "" {
		t.Error("expected error for empty name")
	}
	if m.activeView != ViewRename {
		t.Errorf("expected ViewRename still active, got %d", m.activeView)
	}
}

func TestRenameOverlay_SameNameBlocked(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "j")
	m = sendKey(m, "R")
	if m.renameState == nil {
		t.Fatal("expected renameState to be set")
	}

	// Type the same name as the current item
	currentName := m.renameState.Item.ShortName
	for _, ch := range currentName {
		m = sendKey(m, string(ch))
	}
	m = sendKey(m, "enter")
	if m.renameState.Error == "" {
		t.Errorf("expected error for same name %q", currentName)
	}
}

func TestRenameOverlay_DuplicateNameBlocked(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "j")
	m = sendKey(m, "R")
	if m.renameState == nil {
		t.Fatal("expected renameState to be set")
	}

	// Try to rename to "root" (item 0's name — the main worktree)
	for _, ch := range "root" {
		m = sendKey(m, string(ch))
	}
	m = sendKey(m, "enter")
	if m.renameState.Error == "" {
		t.Error("expected error for duplicate name 'root'")
	}
}

func TestRenameOverlay_RenamingIgnoresInput(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewRename
	item := WorktreeItem{ShortName: "test"}
	m.renameState = &RenameState{
		Item:     &item,
		Renaming: true,
	}

	m = sendKey(m, "esc")
	if m.activeView != ViewRename {
		t.Errorf("expected ViewRename while renaming, got %d", m.activeView)
	}
}

func TestRenameCompleteMsg_Success(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewRename
	item := WorktreeItem{ShortName: "old-name"}
	m.renameState = &RenameState{
		Item:     &item,
		Renaming: true,
	}

	m = sendMsg(m, renameCompleteMsg{oldName: "old-name", newName: "new-name"})
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after rename complete, got %d", m.activeView)
	}
	if m.renameState != nil {
		t.Error("expected renameState nil after success")
	}
	if m.pendingSelect != "new-name" {
		t.Errorf("expected pendingSelect='new-name', got %q", m.pendingSelect)
	}
}

func TestRenameCompleteMsg_Error(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewRename
	item := WorktreeItem{ShortName: "old-name"}
	m.renameState = &RenameState{
		Item:     &item,
		Renaming: true,
	}

	m = sendMsg(m, renameCompleteMsg{oldName: "old-name", newName: "new-name", err: errTest})
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after rename error, got %d", m.activeView)
	}
	// Toast should show error — can't easily test toast content without
	// accessing unexported fields, but we verify the state cleanup is correct
	if m.renameState != nil {
		t.Error("expected renameState nil after error")
	}
}

func TestRenderRename_AllStates(t *testing.T) {
	t.Run("initial", func(t *testing.T) {
		item := WorktreeItem{ShortName: "test-feature"}
		s := NewRenameState(&item)
		v := renderRename(s, 80)
		if v == "" {
			t.Fatal("expected non-empty render")
		}
		if !strings.Contains(v, "Rename Worktree") {
			t.Error("expected 'Rename Worktree' title")
		}
		if !strings.Contains(v, "test-feature") {
			t.Error("expected current name in render")
		}
	})

	t.Run("renaming", func(t *testing.T) {
		item := WorktreeItem{ShortName: "test-feature"}
		s := &RenameState{Item: &item, Renaming: true}
		v := renderRename(s, 80)
		if !strings.Contains(v, "Renaming") {
			t.Error("expected 'Renaming' in progress render")
		}
	})

	t.Run("error", func(t *testing.T) {
		item := WorktreeItem{ShortName: "test-feature"}
		s := &RenameState{Item: &item, Error: "name already taken"}
		v := renderRename(s, 80)
		if !strings.Contains(v, "name already taken") {
			t.Error("expected error text in render")
		}
	})

	t.Run("nil", func(t *testing.T) {
		v := renderRename(nil, 80)
		if v != "" {
			t.Error("expected empty render for nil state")
		}
	})
}
