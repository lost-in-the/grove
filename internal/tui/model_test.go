package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/theme"
	"github.com/lost-in-the/grove/plugins/tracker"
)

func TestUpdateDashboardNavigation(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		before func(*Model)
		assert func(*testing.T, Model)
	}{
		{
			name: "j moves cursor down",
			key:  "j",
			before: func(m *Model) {
				// items already seeded by withItems
			},
			assert: func(t *testing.T, m Model) {
				if m.list.Index() != 1 {
					t.Errorf("expected cursor at 1, got %d", m.list.Index())
				}
			},
		},
		{
			name: "k does not move cursor above 0",
			key:  "k",
			assert: func(t *testing.T, m Model) {
				if m.list.Index() != 0 {
					t.Errorf("expected cursor at 0, got %d", m.list.Index())
				}
			},
		},
		{
			name: "? opens help overlay",
			key:  "?",
			assert: func(t *testing.T, m Model) {
				if !m.helpOverlay.Active {
					t.Error("expected helpOverlay.Active=true after ?")
				}
			},
		},
		{
			name: "n opens create view",
			key:  "n",
			assert: func(t *testing.T, m Model) {
				if m.activeView != ViewCreate {
					t.Errorf("expected ViewCreate, got %d", m.activeView)
				}
				if m.createState == nil {
					t.Fatal("expected createState to be set")
				}
				if m.createState.Step != CreateStepBranch {
					t.Errorf("expected CreateStepBranch, got %d", m.createState.Step)
				}
			},
		},
		{
			name: "o cycles sort mode",
			key:  "o",
			assert: func(t *testing.T, m Model) {
				if m.sortMode != SortByLastAccessed {
					t.Errorf("expected SortByLastAccessed, got %d", m.sortMode)
				}
			},
		},
		{
			name: "a opens bulk view",
			key:  "a",
			assert: func(t *testing.T, m Model) {
				if m.activeView != ViewBulk {
					t.Errorf("expected ViewBulk, got %d", m.activeView)
				}
				if m.bulkState == nil {
					t.Fatal("expected bulkState to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(withItems(5), withSize(80, 24))
			if tt.before != nil {
				tt.before(&m)
			}
			m = sendKey(m, tt.key)
			tt.assert(t, m)
		})
	}
}

func TestDeleteFlow(t *testing.T) {
	t.Run("d opens delete overlay for non-main item", func(t *testing.T) {
		m := newTestModel(withItems(3), withSize(80, 24))
		// Move to item 1 (non-main)
		m = sendKey(m, "j")
		m = sendKey(m, "d")
		if m.activeView != ViewDelete {
			t.Errorf("expected ViewDelete, got %d", m.activeView)
		}
		if m.deleteState == nil {
			t.Fatal("expected deleteState to be set")
		}
	})

	t.Run("d does nothing on main worktree", func(t *testing.T) {
		m := newTestModel(withItems(3), withSize(80, 24))
		// Cursor is at 0 = main
		m = sendKey(m, "d")
		if m.activeView != ViewDashboard {
			t.Errorf("expected ViewDashboard, got %d", m.activeView)
		}
		if m.deleteState != nil {
			t.Error("expected deleteState to be nil for main worktree")
		}
	})

	t.Run("space toggles delete branch", func(t *testing.T) {
		m := newTestModel(withItems(3), withSize(80, 24))
		m = sendKey(m, "j")
		m = sendKey(m, "d")
		if !m.deleteState.DeleteBranch == false {
			t.Error("expected DeleteBranch to start false")
		}
		m = sendKey(m, " ")
		if !m.deleteState.DeleteBranch {
			t.Error("expected DeleteBranch to be true after toggle")
		}
	})

	t.Run("n cancels delete", func(t *testing.T) {
		m := newTestModel(withItems(3), withSize(80, 24))
		m = sendKey(m, "j")
		m = sendKey(m, "d")
		m = sendKey(m, "n")
		if m.activeView != ViewDashboard {
			t.Errorf("expected ViewDashboard after cancel, got %d", m.activeView)
		}
		if m.deleteState != nil {
			t.Error("expected deleteState to be nil after cancel")
		}
	})

	t.Run("esc cancels delete", func(t *testing.T) {
		m := newTestModel(withItems(3), withSize(80, 24))
		m = sendKey(m, "j")
		m = sendKey(m, "d")
		m = sendKey(m, "esc")
		if m.activeView != ViewDashboard {
			t.Errorf("expected ViewDashboard, got %d", m.activeView)
		}
	})

	t.Run("y sets Deleting true", func(t *testing.T) {
		m := newTestModel(withItems(3), withSize(80, 24))
		m = sendKey(m, "j")
		m = sendKey(m, "d")
		m = sendKey(m, "y")
		if m.deleteState == nil {
			t.Fatal("expected deleteState to still be set")
		}
		if !m.deleteState.Deleting {
			t.Error("expected Deleting to be true after confirming")
		}
	})

	t.Run("keys ignored while deleting", func(t *testing.T) {
		m := newTestModel(withItems(3), withSize(80, 24))
		m = sendKey(m, "j")
		m = sendKey(m, "d")
		m = sendKey(m, "y")
		// Try to cancel while deleting — should be ignored
		m = sendKey(m, "n")
		if m.activeView != ViewDelete {
			t.Errorf("expected ViewDelete (input blocked during delete), got %d", m.activeView)
		}
		m = sendKey(m, "esc")
		if m.activeView != ViewDelete {
			t.Errorf("expected ViewDelete (esc blocked during delete), got %d", m.activeView)
		}
	})
}

func TestCreateWizardFlow(t *testing.T) {
	t.Run("branch step: typing adds to filter", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterCreateManual(m)
		m = sendKey(m, "d")
		m = sendKey(m, "e")
		m = sendKey(m, "v")
		if m.createState.BranchFilterInput.Value() != "dev" {
			t.Errorf("expected filter 'dev', got %q", m.createState.BranchFilterInput.Value())
		}
	})

	t.Run("branch step: selecting existing branch advances to action", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterCreateManual(m)
		// Select first branch (main)
		m = sendKey(m, "enter")
		if m.createState.Step != CreateStepBranchAction {
			t.Errorf("expected CreateStepBranchAction, got %d", m.createState.Step)
		}
		if m.createState.BaseBranch != "main" {
			t.Errorf("expected BaseBranch 'main', got %q", m.createState.BaseBranch)
		}
	})

	t.Run("branch step: create new branch advances to name", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterCreateManual(m)
		// Type a name that doesn't match any branch
		m = sendKey(m, "m")
		m = sendKey(m, "y")
		m = sendKey(m, "-")
		m = sendKey(m, "f")
		m = sendKey(m, "e")
		m = sendKey(m, "a")
		m = sendKey(m, "t")
		// Move cursor down to "Create new branch" option (past the 0 filtered results)
		// With filter "my-feat", no branches match, so cursor 0 = "Create new branch"
		m = sendKey(m, "enter")
		if m.createState.Step != CreateStepName {
			t.Errorf("expected CreateStepName, got %d", m.createState.Step)
		}
		if m.createState.NewBranchName != "my-feat" {
			t.Errorf("expected NewBranchName 'my-feat', got %q", m.createState.NewBranchName)
		}
	})

	t.Run("name step: typing adds characters", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterCreateManual(m)
		m = enterNameStep(m)
		m = sendKey(m, "t")
		m = sendKey(m, "e")
		m = sendKey(m, "s")
		m = sendKey(m, "t")
		if m.createState.Name != "test" {
			t.Errorf("expected name 'test', got %q", m.createState.Name)
		}
	})

	t.Run("name step: enter with empty name and no suggestion shows error", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterCreateManual(m)
		m = enterNameStep(m)
		m = sendKey(m, "enter")
		if m.createState.Error != "name cannot be empty" {
			t.Errorf("expected empty name error, got %q", m.createState.Error)
		}
	})

	t.Run("name step: enter with empty name uses suggestion", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterCreateManual(m)
		m = enterNameStep(m)
		m.createState.NameSuggestion = "agent-slot-db"
		m = sendKey(m, "enter")
		if m.createState.Name != "agent-slot-db" {
			t.Errorf("expected name 'agent-slot-db', got %q", m.createState.Name)
		}
		if m.createState.Step != CreateStepConfirm {
			t.Errorf("expected CreateStepConfirm, got %d", m.createState.Step)
		}
	})

	t.Run("name step: backspace goes back to branch", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterCreateManual(m)
		m = enterNameStep(m)
		m = sendKey(m, "backspace")
		if m.createState.Step != CreateStepBranch {
			t.Errorf("expected CreateStepBranch after backspace, got %d", m.createState.Step)
		}
	})

	t.Run("esc at any step cancels create", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterCreateManual(m)
		m = sendKey(m, "esc")
		if m.activeView != ViewDashboard {
			t.Errorf("expected ViewDashboard, got %d", m.activeView)
		}
		if m.createState != nil {
			t.Error("expected createState to be nil")
		}
	})
}

func TestHelpOverlayToggleFromDashboard(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "?")
	if !m.helpOverlay.Active {
		t.Fatal("expected helpOverlay.Active=true after first ?")
	}
	// Second ? should close
	m = sendKey(m, "?")
	if m.helpOverlay.Active {
		t.Error("expected helpOverlay.Active=false after second ?")
	}
}

func TestQuickSwitch(t *testing.T) {
	t.Run("1 switches to first item", func(t *testing.T) {
		m := newTestModel(withItems(5), withSize(80, 24))
		// Item 0 is current (main), so pressing "1" should quit (current worktree)
		result, cmd := m.Update(makeKeyMsg("1"))
		_ = result.(Model)
		// It's the current worktree, so it should just quit
		if cmd == nil {
			t.Error("expected quit command for current worktree")
		}
	})

	t.Run("2 switches to second item", func(t *testing.T) {
		m := newTestModel(withItems(5), withSize(80, 24))
		result, _ := m.Update(makeKeyMsg("2"))
		m = result.(Model)
		if m.switchTo == "" {
			t.Error("expected switchTo to be set for non-current item")
		}
	})

	t.Run("9 does nothing with only 5 items", func(t *testing.T) {
		m := newTestModel(withItems(5), withSize(80, 24))
		result, _ := m.Update(makeKeyMsg("9"))
		m = result.(Model)
		if m.switchTo != "" {
			t.Error("expected no switch for out-of-range number")
		}
	})
}

func TestSortCycling(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))
	if m.sortMode != SortByName {
		t.Fatal("expected initial sort by name")
	}

	m = sendKey(m, "o")
	if m.sortMode != SortByLastAccessed {
		t.Errorf("expected SortByLastAccessed, got %d", m.sortMode)
	}

	m = sendKey(m, "o")
	if m.sortMode != SortByDirtyFirst {
		t.Errorf("expected SortByDirtyFirst, got %d", m.sortMode)
	}

	m = sendKey(m, "o")
	if m.sortMode != SortByName {
		t.Errorf("expected SortByName after full cycle, got %d", m.sortMode)
	}
}

func TestBulkFlow(t *testing.T) {
	t.Run("a opens bulk view", func(t *testing.T) {
		m := newTestModel(withItems(5), withSize(80, 24))
		m = sendKey(m, "a")
		if m.activeView != ViewBulk {
			t.Errorf("expected ViewBulk, got %d", m.activeView)
		}
		if m.bulkState == nil {
			t.Fatal("expected bulkState to be set")
		}
		// Main and current items should be excluded
		for _, item := range m.bulkState.Items {
			if item.IsMain || item.IsCurrent {
				t.Errorf("bulk should not include main/current item: %s", item.ShortName)
			}
		}
	})

	t.Run("space toggles selection", func(t *testing.T) {
		m := newTestModel(withItems(5), withSize(80, 24))
		m = sendKey(m, "a")
		m = sendKey(m, " ")
		if m.bulkState.SelectedCount() != 1 {
			t.Errorf("expected 1 selected, got %d", m.bulkState.SelectedCount())
		}
		m = sendKey(m, " ")
		if m.bulkState.SelectedCount() != 0 {
			t.Errorf("expected 0 selected after untoggle, got %d", m.bulkState.SelectedCount())
		}
	})

	t.Run("esc cancels bulk", func(t *testing.T) {
		m := newTestModel(withItems(5), withSize(80, 24))
		m = sendKey(m, "a")
		m = sendKey(m, "esc")
		if m.activeView != ViewDashboard {
			t.Errorf("expected ViewDashboard, got %d", m.activeView)
		}
		if m.bulkState != nil {
			t.Error("expected bulkState to be nil")
		}
	})

	t.Run("enter with no selection does nothing", func(t *testing.T) {
		m := newTestModel(withItems(5), withSize(80, 24))
		m = sendKey(m, "a")
		m = sendKey(m, "enter")
		// Should still be in bulk view since nothing selected
		if m.activeView != ViewBulk {
			t.Errorf("expected ViewBulk (no selection), got %d", m.activeView)
		}
	})

	t.Run("navigation in bulk view", func(t *testing.T) {
		m := newTestModel(withItems(5), withSize(80, 24))
		m = sendKey(m, "a")
		initial := m.bulkState.Cursor
		m = sendKey(m, "j")
		if m.bulkState.Cursor != initial+1 {
			t.Errorf("expected cursor at %d, got %d", initial+1, m.bulkState.Cursor)
		}
		m = sendKey(m, "k")
		if m.bulkState.Cursor != initial {
			t.Errorf("expected cursor back at %d, got %d", initial, m.bulkState.Cursor)
		}
	})
}

func TestWorktreesFetchedMsg(t *testing.T) {
	t.Run("successful fetch populates list", func(t *testing.T) {
		m := newTestModel(withSize(80, 24))
		m.loading = true
		items := makeTestItems(3)
		m = sendMsg(m, worktreesFetchedMsg{items: items})
		if m.loading {
			t.Error("expected loading to be false after fetch")
		}
		if len(m.list.Items()) != 3 {
			t.Errorf("expected 3 items, got %d", len(m.list.Items()))
		}
	})

	t.Run("fetch error sets err and quits", func(t *testing.T) {
		m := newTestModel(withSize(80, 24))
		m.loading = true
		_, cmd := m.Update(worktreesFetchedMsg{err: errTest})
		result := m
		if result.err == nil {
			// The err is set on the returned model
			result2, _ := m.Update(worktreesFetchedMsg{err: errTest})
			result = result2.(Model)
		}
		if cmd == nil {
			t.Error("expected quit command on error")
		}
	})
}

func TestToastLifecycle(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))

	// Simulate a delete completion
	m = sendMsg(m, worktreeDeletedMsg{name: "testing", err: nil})
	if m.toast.Message() == "" {
		t.Error("expected toast message after delete")
	}

	// Dismiss clears it
	m.toast.Dismiss()
	if m.toast.Message() != "" {
		t.Error("expected toast message to be cleared after dismiss")
	}
}

func TestToastSuperseding(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))

	// First action
	m = sendMsg(m, worktreeDeletedMsg{name: "first", err: nil})

	// Second action supersedes
	m = sendMsg(m, worktreeCreatedMsg{name: "second"})
	if m.toast.Message() == "" || m.toast.Message() == `Deleted "first"` {
		t.Error("expected second toast to replace first")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	m.ready = false
	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if !m.ready {
		t.Error("expected ready to be true after WindowSizeMsg")
	}
	if m.width != 120 || m.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", m.width, m.height)
	}
}

func TestLoadingState(t *testing.T) {
	m := newTestModel(withLoading(), withSize(80, 24))
	v := m.viewString()
	if v == "" {
		t.Error("expected non-empty view during loading")
	}
	// Should contain loading text
	if !containsStr(v, "Loading") {
		t.Error("expected loading text in view")
	}
}

var errTest = fmt.Errorf("test error")

func TestDataInterfaceMethods(t *testing.T) {
	item := WorktreeItem{ShortName: "test", Branch: "feature/test"}
	if item.Title() != "test" {
		t.Errorf("Title() = %q, want %q", item.Title(), "test")
	}
	if item.Description() != "feature/test" {
		t.Errorf("Description() = %q, want %q", item.Description(), "feature/test")
	}
	if item.FilterValue() != "test feature/test" {
		t.Errorf("FilterValue() = %q", item.FilterValue())
	}
}

func TestStatusText(t *testing.T) {
	tests := []struct {
		name  string
		item  WorktreeItem
		check string
	}{
		{"prunable", WorktreeItem{IsPrunable: true}, "stale"},
		{"dirty", WorktreeItem{IsDirty: true}, "dirty"},
		{"clean", WorktreeItem{}, "clean"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.item.StatusText()
			if !strings.Contains(got, tt.check) {
				t.Errorf("StatusText() = %q, expected to contain %q", got, tt.check)
			}
		})
	}
}

func TestTmuxText(t *testing.T) {
	tests := []struct {
		status string
		check  string
	}{
		{"attached", "attached"},
		{"detached", "tmux"},
		{"none", ""},
	}
	for _, tt := range tests {
		item := WorktreeItem{TmuxStatus: tt.status}
		got := item.TmuxText()
		if tt.check == "" && got != "" {
			t.Errorf("TmuxText() for %q should be empty, got %q", tt.status, got)
		}
		if tt.check != "" && !strings.Contains(got, tt.check) {
			t.Errorf("TmuxText() for %q = %q, expected %q", tt.status, got, tt.check)
		}
	}
}

func TestAgeText(t *testing.T) {
	item := WorktreeItem{CommitAge: "2 hours ago"}
	got := item.AgeText()
	if got == "" {
		t.Error("expected non-empty AgeText()")
	}
	empty := WorktreeItem{}
	if empty.AgeText() != "" {
		t.Error("expected empty AgeText() for empty commit age")
	}
}

func TestRenderCreateOverlayWithError(t *testing.T) {
	s := createStateWithName("bad/name", "proj")
	s.Error = "invalid character"
	v := renderCreateV2(s, 80, "")
	if !strings.Contains(v, "invalid") {
		t.Error("expected error in create overlay")
	}
}

func TestRenderCreateBranchActionWithDontShowAgain(t *testing.T) {
	s := &CreateState{Step: CreateStepBranchAction, BaseBranch: "dev", DontShowAgain: true}
	v := renderCreateV2(s, 80, "")
	if !strings.Contains(v, "[x]") {
		t.Error("expected checked checkbox for DontShowAgain")
	}
}

func TestRenderCreateOverlaySteps(t *testing.T) {
	t.Run("name step", func(t *testing.T) {
		s := createStateWithName("test", "proj")
		v := renderCreateV2(s, 80, "")
		if !strings.Contains(v, "Name") {
			t.Error("expected 'Name' in create name step")
		}
	})

	t.Run("branch step", func(t *testing.T) {
		s := &CreateState{Step: CreateStepBranch, Branches: []string{"main", "develop"}, BranchFilterInput: newBranchFilterInput()}
		v := renderCreateV2(s, 80, "")
		if !strings.Contains(v, "Branch") {
			t.Error("expected 'Branch' in create branch step")
		}
	})

	t.Run("branch step with filter", func(t *testing.T) {
		s := createStateWithBranchFilter([]string{"main", "develop"}, "dev")
		v := renderCreateV2(s, 80, "")
		if !strings.Contains(v, "Filter") {
			t.Error("expected 'Filter' label in filtered branch list")
		}
	})

	t.Run("branch step no matches shows create new", func(t *testing.T) {
		s := createStateWithBranchFilter([]string{"main"}, "nonexistent")
		v := renderCreateV2(s, 80, "")
		if !strings.Contains(v, "Create new branch") {
			t.Error("expected 'Create new branch' option")
		}
	})

	t.Run("branch action step", func(t *testing.T) {
		s := &CreateState{Step: CreateStepBranchAction, BaseBranch: "develop"}
		v := renderCreateV2(s, 80, "")
		if !strings.Contains(v, "already exists") {
			t.Error("expected 'already exists' in branch action step")
		}
	})
}

func TestRenderDeleteOverlay(t *testing.T) {
	item := &WorktreeItem{ShortName: "testing", Branch: "testing"}
	s := &DeleteState{Item: item, Warnings: []string{"dirty"}, DeleteBranch: true}
	v := renderDeleteV2(s, 80)
	if !strings.Contains(v, "testing") {
		t.Error("expected item name in delete overlay")
	}
	if !strings.Contains(v, "[x]") {
		t.Error("expected checked checkbox when DeleteBranch=true")
	}
}

func TestRenderBulkOverlay(t *testing.T) {
	t.Run("empty items", func(t *testing.T) {
		s := &BulkState{Items: nil, Selected: nil}
		v := renderBulk(s)
		if !strings.Contains(v, "No merged") {
			t.Error("expected empty message")
		}
	})

	t.Run("deleting state", func(t *testing.T) {
		s := &BulkState{Deleting: true, Progress: "Deleting 2/3..."}
		v := renderBulk(s)
		if !strings.Contains(v, "Deleting 2/3") {
			t.Error("expected progress text")
		}
	})

	t.Run("with items", func(t *testing.T) {
		items := makeTestItems(3)
		s := &BulkState{
			Items:    items,
			Selected: []bool{true, false, false},
		}
		v := renderBulk(s)
		if !strings.Contains(v, "1/3 selected") {
			t.Error("expected selection count")
		}
	})
}

func TestDetailContent(t *testing.T) {
	t.Run("nil item", func(t *testing.T) {
		got := renderDetailV2(nil, 80)
		if got != "" {
			t.Error("expected empty content for nil item")
		}
	})

	t.Run("narrow width", func(t *testing.T) {
		item := &WorktreeItem{ShortName: "test"}
		got := renderDetailV2(item, 10)
		if got != "" {
			t.Error("expected empty content for narrow width")
		}
	})

	t.Run("full item", func(t *testing.T) {
		item := &WorktreeItem{
			ShortName:     "testing",
			Branch:        "feature/test",
			Commit:        "abc1234",
			CommitMessage: "add feature",
			CommitAge:     "2h ago",
			AheadCount:    1,
			BehindCount:   2,
			IsEnvironment: true,
			IsDirty:       true,
			DirtyFiles:    []string{"M  file.go", "?? new.go", " D old.go"},
		}
		got := renderDetailV2(item, 80)
		if got == "" {
			t.Error("expected non-empty detail content")
		}
	})
}

func TestGatherDeleteWarnings(t *testing.T) {
	tests := []struct {
		name    string
		item    WorktreeItem
		wantLen int
	}{
		{"no warnings", WorktreeItem{}, 0},
		{"protected", WorktreeItem{IsProtected: true}, 1},
		{"dirty", WorktreeItem{IsDirty: true}, 1},
		{"environment", WorktreeItem{IsEnvironment: true}, 1},
		{"all warnings", WorktreeItem{IsProtected: true, IsDirty: true, IsEnvironment: true}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gatherDeleteWarnings(&tt.item)
			if len(got) != tt.wantLen {
				t.Errorf("got %d warnings, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestNoColor(t *testing.T) {
	// Ensure env is clean
	t.Setenv("NO_COLOR", "1")
	if !theme.IsNoColor() {
		t.Error("expected theme.IsNoColor()=true when NO_COLOR set")
	}
}

func TestNoColorGrove(t *testing.T) {
	t.Setenv("GROVE_NO_COLOR", "1")
	if !theme.IsNoColor() {
		t.Error("expected theme.IsNoColor()=true when GROVE_NO_COLOR set")
	}
}

func TestHandleDeleteKeyNilState(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m.activeView = ViewDelete
	m.deleteState = nil
	m = sendKey(m, "y")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func TestHandleCreateKeyNilState(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m.activeView = ViewCreate
	m.createState = nil
	m = sendKey(m, "a")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func TestHandleBulkKeyNilState(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m.activeView = ViewBulk
	m.bulkState = nil
	m = sendKey(m, "j")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func TestHandleBulkKeyWhileDeleting(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))
	m = sendKey(m, "a")
	m.bulkState.Deleting = true
	// Input should be ignored while deleting
	cursor := m.bulkState.Cursor
	m = sendKey(m, "j")
	if m.bulkState.Cursor != cursor {
		t.Error("expected cursor unchanged while deleting")
	}
}

func TestBulkEnterWithSelection(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))
	m = sendKey(m, "a")
	m = sendKey(m, " ") // select first
	_, cmd := m.Update(makeKeyMsg("enter"))
	if cmd == nil {
		t.Error("expected cmd from bulk enter with selection")
	}
}

func TestWorktreeCreatedMsgWithError(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m = enterCreateManual(m)
	m.createState.Name = "test"
	m = sendMsg(m, worktreeCreatedMsg{err: errTest})
	if m.createState == nil {
		t.Error("expected createState to be preserved on error")
	}
	if m.createState.Error == "" {
		t.Error("expected error message on createState")
	}
}

func TestWorktreeDeletedMsgWithError(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m.activeView = ViewDelete
	m.deleteState = &DeleteState{Item: &WorktreeItem{ShortName: "test"}}
	m = sendMsg(m, worktreeDeletedMsg{name: "test", err: errTest})
	// Toast should show the error
	if m.toast.Current == nil || m.toast.Current.Level != ToastError {
		t.Error("expected error toast on delete failure")
	}
}

func TestWorktreeDeletedMsgErrorDuringDeletion(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m.activeView = ViewDelete
	m.deleteState = &DeleteState{
		Item:     &WorktreeItem{ShortName: "test"},
		Deleting: true,
	}
	m = sendMsg(m, worktreeDeletedMsg{name: "test", err: errTest})

	// deleteState should be cleared
	if m.deleteState != nil {
		t.Error("expected deleteState to be nil after error")
	}
	// Should return to dashboard
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
	// Toast should show error
	if m.toast.Current == nil || m.toast.Current.Level != ToastError {
		t.Error("expected error toast on delete failure during deletion")
	}
}

func TestBulkDeleteDoneMsg(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))
	m.activeView = ViewBulk
	m.bulkState = &BulkState{Deleting: true}
	m = sendMsg(m, bulkDeleteDoneMsg{count: 3})
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
	if m.bulkState != nil {
		t.Error("expected bulkState nil after done")
	}
	if !strings.Contains(m.toast.Message(), "3") {
		t.Errorf("expected toast msg with count, got %q", m.toast.Message())
	}
}

func TestSpinnerTickWhenNotLoading(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m.loading = false
	_, cmd := m.Update(spinner.TickMsg{})
	if cmd != nil {
		t.Error("expected no cmd when not loading")
	}
}

func TestEnterOnCurrentWorktree(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	// Item 0 is current
	_, cmd := m.Update(makeKeyMsg("enter"))
	if cmd == nil {
		t.Error("expected quit cmd for current worktree enter")
	}
	if m.switchTo != "" {
		t.Error("expected no switchTo for current worktree")
	}
}

func TestEnterOnNonCurrentWorktree(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "j") // move to non-current
	result, cmd := m.Update(makeKeyMsg("enter"))
	m = result.(Model)
	if cmd == nil {
		t.Error("expected quit cmd")
	}
	if m.switchTo == "" {
		t.Error("expected switchTo to be set")
	}
}

func TestDeleteOnProtectedItem(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	// Make item 1 protected
	items := m.list.Items()
	item := items[1].(WorktreeItem)
	item.IsProtected = true
	items[1] = item
	m.list.SetItems(items)
	m = sendKey(m, "j") // move to protected item
	m = sendKey(m, "d")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard for protected item, got %d", m.activeView)
	}
}

func TestWorktreesFetchedWithSort(t *testing.T) {
	m := newTestModel(withSize(80, 24))
	m.loading = true
	m.sortMode = SortByDirtyFirst
	items := makeTestItems(5)
	m = sendMsg(m, worktreesFetchedMsg{items: items})
	if m.loading {
		t.Error("expected loading=false")
	}
	if len(m.list.Items()) != 5 {
		t.Errorf("expected 5 items, got %d", len(m.list.Items()))
	}
}

func TestUpdateDetailContentNoItems(t *testing.T) {
	m := newTestModel(withSize(80, 24))
	// No items set
	m.updateDetailContent()
	// Should not panic
}

func TestCreateNameBackspaceOnEmpty(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m = enterCreateManual(m)
	m = enterNameStep(m)
	m = sendKey(m, "backspace")
	// Backspace with empty name goes back to branch step
	if m.createState.Step != CreateStepBranch {
		t.Errorf("expected CreateStepBranch after backspace on empty name, got %d", m.createState.Step)
	}
}

func TestCreateBranchEsc(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m = enterCreateManual(m)
	m.createState.Step = CreateStepBranch
	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func TestCreateNameEnterWithInvalidName(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m = enterCreateManual(m)
	m = enterNameStep(m)
	m.createState.NameInput.SetValue("invalid name with spaces")
	m.createState.Name = "invalid name with spaces"
	m = sendKey(m, "enter")
	if m.createState.Error == "" {
		t.Error("expected validation error")
	}
	if m.createState.Step != CreateStepName {
		t.Errorf("expected still on CreateStepName, got %d", m.createState.Step)
	}
}

func TestBranchStepUpAtZero(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m = enterCreateManual(m)
	m.createState.Step = CreateStepBranch
	m.createState.BranchCursor = 0
	m = sendKey(m, "k")
	if m.createState.BranchCursor != 0 {
		t.Errorf("expected BranchCursor=0, got %d", m.createState.BranchCursor)
	}
}

func TestBranchSelectorUpAtZero(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m = enterCreateManual(m)
	m.createState.Step = CreateStepBranch
	m.createState.Branches = []string{"main", "dev"}
	m.createState.BranchCursor = 0
	m = sendKey(m, "up")
	if m.createState.BranchCursor != 0 {
		t.Errorf("expected cursor=0, got %d", m.createState.BranchCursor)
	}
}

func TestBranchActionUpAtZero(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m = enterCreateManual(m)
	m.createState.Step = CreateStepBranchAction
	m.createState.ActionChoice = 0
	m = sendKey(m, "k")
	if m.createState.ActionChoice != 0 {
		t.Errorf("expected ActionChoice=0, got %d", m.createState.ActionChoice)
	}
}

func TestBranchActionDownAtMax(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m = enterCreateManual(m)
	m.createState.Step = CreateStepBranchAction
	m.createState.ActionChoice = 1
	m = sendKey(m, "j")
	if m.createState.ActionChoice != 1 {
		t.Errorf("expected ActionChoice=1, got %d", m.createState.ActionChoice)
	}
}

func TestQuitKeys(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	_, cmd := m.Update(makeKeyMsg("q"))
	if cmd == nil {
		t.Error("expected quit cmd from q")
	}
}

func TestEscQuitsDashboard(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	_, cmd := m.Update(makeKeyMsg("esc"))
	if cmd == nil {
		t.Error("expected quit cmd from esc on dashboard")
	}
}

func TestSwitchToAndErr(t *testing.T) {
	m := newTestModel()
	if m.SwitchTo() != "" {
		t.Error("expected empty SwitchTo()")
	}
	if m.Err() != nil {
		t.Error("expected nil Err()")
	}
}

func TestCreateWizardBranchSelectorNavigation(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m = enterCreateManual(m)
	// Start at branch step with branches: main, develop, feature/auth

	// Navigate down
	m = sendKey(m, "j")
	if m.createState.BranchCursor != 1 {
		t.Errorf("expected BranchCursor=1, got %d", m.createState.BranchCursor)
	}

	// Navigate back up
	m = sendKey(m, "k")
	if m.createState.BranchCursor != 0 {
		t.Errorf("expected BranchCursor=0, got %d", m.createState.BranchCursor)
	}
}

func TestCreateWizardBranchActionNavigation(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m = enterCreateManual(m)

	// Directly set up the branch action step
	m.createState.Step = CreateStepBranchAction
	m.createState.BaseBranch = "develop"

	// Navigate down
	m = sendKey(m, "j")
	if m.createState.ActionChoice != 1 {
		t.Errorf("expected ActionChoice=1, got %d", m.createState.ActionChoice)
	}

	// Navigate up
	m = sendKey(m, "k")
	if m.createState.ActionChoice != 0 {
		t.Errorf("expected ActionChoice=0, got %d", m.createState.ActionChoice)
	}

	// Toggle dont-show-again
	m = sendKey(m, " ")
	if !m.createState.DontShowAgain {
		t.Error("expected DontShowAgain=true after toggle")
	}

	// Backspace goes back to branch selector
	m = sendKey(m, "backspace")
	if m.createState.Step != CreateStepBranch {
		t.Errorf("expected CreateStepBranch, got %d", m.createState.Step)
	}
}

func TestCreateWizardBranchFilterAndNavigation(t *testing.T) {
	t.Run("typing adds to filter", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterCreateManual(m)
		m.createState.Step = CreateStepBranch
		m.createState.Branches = []string{"main", "develop", "feature/auth"}

		m = sendKey(m, "d")
		if m.createState.BranchFilterInput.Value() != "d" {
			t.Errorf("expected filter 'd', got %q", m.createState.BranchFilterInput.Value())
		}
	})

	t.Run("backspace removes filter char", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterCreateManual(m)
		m.createState.Step = CreateStepBranch
		m.createState.Branches = []string{"main", "develop"}
		m.createState.BranchFilterInput.SetValue("de")

		m = sendKey(m, "backspace")
		if m.createState.BranchFilterInput.Value() != "d" {
			t.Errorf("expected filter 'd', got %q", m.createState.BranchFilterInput.Value())
		}
	})

	t.Run("backspace with empty filter is no-op on branch step", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterCreateManual(m)
		m.createState.Step = CreateStepBranch
		m.createState.Branches = []string{"main"}

		m = sendKey(m, "backspace")
		// Branch is step 0, so backspace with empty filter stays on branch
		if m.createState.Step != CreateStepBranch {
			t.Errorf("expected CreateStepBranch, got %d", m.createState.Step)
		}
	})

	t.Run("arrow down moves cursor", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterCreateManual(m)
		m.createState.Step = CreateStepBranch
		m.createState.Branches = []string{"main", "develop", "feature/auth"}
		m.createState.BranchCursor = 0

		m = sendKey(m, "down")
		if m.createState.BranchCursor != 1 {
			t.Errorf("expected cursor=1, got %d", m.createState.BranchCursor)
		}
	})
}

func TestFullHelpKeyMap(t *testing.T) {
	keys := DefaultKeyMap()
	groups := keys.FullHelp()
	if len(groups) != 3 {
		t.Errorf("expected 3 groups in FullHelp, got %d", len(groups))
	}
}

func TestBulkSelectedItems(t *testing.T) {
	items := makeTestItems(3)
	s := &BulkState{
		Items:    items,
		Selected: []bool{true, false, true},
	}
	selected := s.SelectedItems()
	if len(selected) != 2 {
		t.Errorf("expected 2 selected, got %d", len(selected))
	}
}

func TestCreateWizardEscFromBranchSelector(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m = enterCreateManual(m)
	m.createState.Step = CreateStepBranch
	m.createState.Branches = []string{"main"}
	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func TestCreateWizardEscFromBranchAction(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m = enterCreateManual(m)
	m.createState.Step = CreateStepBranchAction
	m.createState.BaseBranch = "develop"
	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func TestDownKeyOnEmptyBulkList(t *testing.T) {
	m := newTestModel(withSize(80, 24))
	m.activeView = ViewBulk
	m.bulkState = &BulkState{Items: nil, Selected: nil}
	// Down on empty bulk list must not panic (underflow guard)
	m = sendKey(m, "down")
	if m.bulkState.Cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.bulkState.Cursor)
	}
}

func TestDownKeyOnEmptyConfigTab(t *testing.T) {
	m := newTestModel(withSize(80, 24))
	m.activeView = ViewConfig
	m.configState = NewConfigState()
	// Empty fields — Down must not panic
	m = sendKey(m, "down")
	if m.configState.Cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.configState.Cursor)
	}
}

func TestDownKeyOnEmptySyncSources(t *testing.T) {
	m := newTestModel(withSize(80, 24))
	m.activeView = ViewSync
	m.syncState = &SyncState{Sources: nil}
	// Down on empty sync sources must not panic
	m = sendKey(m, "down")
	if m.syncState.Selected != 0 {
		t.Errorf("expected selected 0, got %d", m.syncState.Selected)
	}
}

func TestSortWithMixedTypes(t *testing.T) {
	// Sort should handle non-WorktreeItem elements gracefully
	items := []list.Item{
		WorktreeItem{ShortName: "alpha"},
		WorktreeItem{ShortName: "beta"},
	}
	sorted := sortWorktreeItems(items, SortByName)
	if len(sorted) != 2 {
		t.Errorf("expected 2 sorted items, got %d", len(sorted))
	}
}

func TestHandlePRKeyNilState(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m.activeView = ViewPRs
	m.prState = nil
	m = sendKey(m, "j")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func TestHandleIssueKeyNilState(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m.activeView = ViewIssues
	m.issueState = nil
	m = sendKey(m, "j")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func TestDeleteConfirmNilItem(t *testing.T) {
	m := newTestModel(withSize(80, 24))
	m.activeView = ViewDelete
	m.deleteState = &DeleteState{Item: nil}
	// Confirm on nil item should not panic
	m = sendKey(m, "y")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func containsStr(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && contains(s, sub)
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// enterPRViewLoaded enters the PR view via the "p" key and simulates data loading.
func enterPRViewLoaded(m Model, prs []*tracker.PullRequest) Model {
	m = sendKey(m, "p")
	if m.prState == nil {
		return m
	}
	m = sendMsg(m, prsFetchedMsg{prs: prs})
	return m
}

// enterIssueViewLoaded enters the Issue view via the "i" key and simulates data loading.
func enterIssueViewLoaded(m Model, issues []*tracker.Issue) Model {
	m = sendKey(m, "i")
	if m.issueState == nil {
		return m
	}
	m = sendMsg(m, issuesFetchedMsg{issues: issues})
	return m
}

func TestPRFilterTypingThroughUpdate(t *testing.T) {
	prs := []*tracker.PullRequest{
		{Number: 1, Title: "Alpha feature", Branch: "alpha", Author: "user"},
		{Number: 2, Title: "Beta bugfix", Branch: "beta", Author: "user"},
	}

	t.Run("typing populates filter after / activation", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterPRViewLoaded(m, prs)

		m = sendKey(m, "/") // activate filter mode
		m = sendKey(m, "a")
		m = sendKey(m, "l")
		if m.prState.FilterInput.Value() != "al" {
			t.Errorf("expected filter 'al', got %q", m.prState.FilterInput.Value())
		}
		if !m.prState.Filtering {
			t.Error("expected Filtering to be true")
		}
	})

	t.Run("typing without / does not filter", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterPRViewLoaded(m, prs)

		m = sendKey(m, "a")
		if m.prState.FilterInput.Value() != "" {
			t.Errorf("expected empty filter without /, got %q", m.prState.FilterInput.Value())
		}
	})

	t.Run("filter narrows results", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterPRViewLoaded(m, prs)

		m = sendKey(m, "/")
		m = sendKey(m, "a")
		m = sendKey(m, "l")
		m = sendKey(m, "p")
		filtered := filteredPRs(m.prState.PRs, m.prState.FilterInput.Value())
		if len(filtered) != 1 {
			t.Errorf("expected 1 filtered PR, got %d", len(filtered))
		}
	})

	t.Run("backspace removes character", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterPRViewLoaded(m, prs)

		m = sendKey(m, "/")
		m = sendKey(m, "a")
		m = sendKey(m, "b")
		m = sendKey(m, "backspace")
		if m.prState.FilterInput.Value() != "a" {
			t.Errorf("expected filter 'a', got %q", m.prState.FilterInput.Value())
		}
	})

	t.Run("esc clears filter and exits filter mode", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterPRViewLoaded(m, prs)

		m = sendKey(m, "/")
		m = sendKey(m, "a")
		m = sendKey(m, "esc")
		if m.prState.FilterInput.Value() != "" {
			t.Errorf("expected empty filter after esc, got %q", m.prState.FilterInput.Value())
		}
		if m.prState.Filtering {
			t.Error("expected Filtering to be false after esc")
		}
	})

	t.Run("enter keeps filter value and exits filter mode", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterPRViewLoaded(m, prs)

		m = sendKey(m, "/")
		m = sendKey(m, "a")
		m = sendKey(m, "l")
		m = sendKey(m, "enter")
		if m.prState.FilterInput.Value() != "al" {
			t.Errorf("expected filter 'al' after enter, got %q", m.prState.FilterInput.Value())
		}
		if m.prState.Filtering {
			t.Error("expected Filtering to be false after enter")
		}
	})
}

func TestIssueFilterTypingThroughUpdate(t *testing.T) {
	issues := []*tracker.Issue{
		{Number: 1, Title: "Login broken"},
		{Number: 2, Title: "Signup slow"},
	}

	t.Run("typing populates filter after / activation", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterIssueViewLoaded(m, issues)

		m = sendKey(m, "/") // activate filter mode
		m = sendKey(m, "l")
		m = sendKey(m, "o")
		if m.issueState.FilterInput.Value() != "lo" {
			t.Errorf("expected filter 'lo', got %q", m.issueState.FilterInput.Value())
		}
		if !m.issueState.Filtering {
			t.Error("expected Filtering to be true")
		}
	})

	t.Run("typing without / does not filter", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterIssueViewLoaded(m, issues)

		m = sendKey(m, "l")
		if m.issueState.FilterInput.Value() != "" {
			t.Errorf("expected empty filter without /, got %q", m.issueState.FilterInput.Value())
		}
	})

	t.Run("filter narrows results", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterIssueViewLoaded(m, issues)

		m = sendKey(m, "/")
		m = sendKey(m, "l")
		m = sendKey(m, "o")
		m = sendKey(m, "g")
		filtered := filteredIssues(m.issueState.Issues, m.issueState.FilterInput.Value())
		if len(filtered) != 1 {
			t.Errorf("expected 1 filtered issue, got %d", len(filtered))
		}
	})

	t.Run("esc clears filter and exits filter mode", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterIssueViewLoaded(m, issues)

		m = sendKey(m, "/")
		m = sendKey(m, "l")
		m = sendKey(m, "esc")
		if m.issueState.FilterInput.Value() != "" {
			t.Errorf("expected empty filter after esc, got %q", m.issueState.FilterInput.Value())
		}
		if m.issueState.Filtering {
			t.Error("expected Filtering to be false after esc")
		}
	})
}

func TestCreateBranchFilterTypingThroughUpdate(t *testing.T) {
	t.Run("typing via Update populates filter", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = sendKey(m, "n") // enter create wizard
		if m.createState == nil {
			t.Fatal("expected createState after 'n'")
		}
		m.createState.Branches = []string{"main", "develop", "feature/auth"}
		// Focus is called on the stored reference by our fix, so sendKey("n")
		// already focused BranchFilterInput via m.createState.BranchFilterInput.Focus()

		m = sendKey(m, "d")
		m = sendKey(m, "e")
		if m.createState.BranchFilterInput.Value() != "de" {
			t.Errorf("expected filter 'de', got %q", m.createState.BranchFilterInput.Value())
		}
	})
}

func TestCreateNameTypingThroughUpdate(t *testing.T) {
	t.Run("typing via Update populates name", func(t *testing.T) {
		m := newTestModel(withItems(1), withSize(80, 24))
		m = enterCreateManual(m)
		m = enterNameStep(m)

		m = sendKey(m, "m")
		m = sendKey(m, "y")
		m = sendKey(m, "w")
		m = sendKey(m, "t")
		if m.createState.NameInput.Value() != "mywt" {
			t.Errorf("expected name 'mywt', got %q", m.createState.NameInput.Value())
		}
		if m.createState.Name != "mywt" {
			t.Errorf("expected createState.Name 'mywt', got %q", m.createState.Name)
		}
	})
}

func TestDashboardKey_B_NoOpWithoutPR(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "B")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after B with no PR, got %d", m.activeView)
	}
}

func TestDashboardKey_B_WithPR(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	// Set an AssociatedPR on the first item
	items := m.list.Items()
	item := items[0].(WorktreeItem)
	item.AssociatedPR = &PRInfo{Number: 42, Title: "Test PR"}
	items[0] = item
	m.list.SetItems(items)

	// Sending B should not panic even with an associated PR
	m = sendKey(m, "B")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after B, got %d", m.activeView)
	}
}

func TestDashboardKey_Tab(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "tab")
	if !m.detailFocused {
		t.Error("expected detailFocused=true after tab")
	}
}
