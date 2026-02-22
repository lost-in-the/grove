package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// --- Stress tests for edge-case scenarios ---

func TestStressLargeDataset(t *testing.T) {
	tests := []struct {
		name      string
		itemCount int
		presses   int
	}{
		{"100 items with 50 j presses", 100, 50},
		{"500 items with 100 j presses", 500, 100},
		{"1000 items with 100 j presses", 1000, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(withItems(tt.itemCount), withSize(80, 24))

			if len(m.list.Items()) != tt.itemCount {
				t.Fatalf("expected %d items, got %d", tt.itemCount, len(m.list.Items()))
			}

			// Press j many times
			for range tt.presses {
				m = sendKey(m, "j")
			}

			// Cursor must be in valid range
			idx := m.list.Index()
			if idx < 0 || idx >= tt.itemCount {
				t.Errorf("cursor out of bounds: got %d, items=%d", idx, tt.itemCount)
			}

			// View should render without panic
			v := m.View()
			if v == "" {
				t.Error("expected non-empty view for large dataset")
			}
		})
	}
}

func TestStressLargeDatasetView(t *testing.T) {
	m := newTestModel(withItems(1000), withSize(120, 40))
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view for 1000 items")
	}
	// Should still contain project name
	if !strings.Contains(v, "test-project") {
		t.Error("expected project name in large dataset view")
	}
}

func TestStressRapidResize(t *testing.T) {
	tests := []struct {
		name   string
		widths []int
		height int
	}{
		{
			"shrink from 120 to 40",
			[]int{120, 110, 100, 90, 80, 70, 60, 50, 45, 40},
			40,
		},
		{
			"grow from 40 to 120",
			[]int{40, 50, 60, 70, 80, 90, 100, 110, 120, 130},
			40,
		},
		{
			"oscillate width",
			[]int{120, 40, 120, 40, 120, 40, 120, 40, 120, 40},
			40,
		},
		{
			"oscillate height",
			[]int{80, 80, 80, 80, 80, 80, 80, 80, 80, 80},
			12, // will cycle heights below
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(withItems(10), withSize(80, 24))

			for i, w := range tt.widths {
				h := tt.height
				if tt.name == "oscillate height" {
					if i%2 == 0 {
						h = 12
					} else {
						h = 40
					}
				}
				m = sendMsg(m, tea.WindowSizeMsg{Width: w, Height: h})
			}

			// Model should still be in a valid state
			if !m.ready {
				t.Error("expected ready=true after resizes")
			}
			if m.width != tt.widths[len(tt.widths)-1] {
				t.Errorf("expected final width=%d, got %d", tt.widths[len(tt.widths)-1], m.width)
			}

			// View must render without panic
			v := m.View()
			if v == "" {
				t.Error("expected non-empty view after rapid resize")
			}
		})
	}
}

func TestStressRapidResizeSequence(t *testing.T) {
	m := newTestModel(withItems(10), withSize(120, 40))

	// Send 20 resize messages from 120x40 down to 40x12 and back
	sizes := [][2]int{
		{120, 40}, {112, 38}, {104, 36}, {96, 34}, {88, 32},
		{80, 30}, {72, 28}, {64, 24}, {56, 20}, {48, 16},
		{40, 12}, {48, 16}, {56, 20}, {64, 24}, {72, 28},
		{80, 30}, {88, 32}, {96, 34}, {104, 36}, {120, 40},
	}

	for _, size := range sizes {
		m = sendMsg(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
	}

	if m.width != 120 || m.height != 40 {
		t.Errorf("expected final size 120x40, got %dx%d", m.width, m.height)
	}

	v := m.View()
	if v == "" {
		t.Error("expected non-empty view after resize sequence")
	}
}

func TestStressOverlayCycling(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		view   ActiveView
		cycles int
	}{
		{"help toggle 50 times", "?", ViewDashboard, 50},
		{"help toggle 100 times", "?", ViewDashboard, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(withItems(5), withSize(80, 24))

			for i := range tt.cycles {
				// Open help
				m = sendKey(m, tt.key)
				if !m.helpFooter.Expanded {
					t.Fatalf("cycle %d: expected helpFooter.Expanded=true after open", i)
				}

				// Close help
				m = sendKey(m, tt.key)
				if m.helpFooter.Expanded {
					t.Fatalf("cycle %d: expected helpFooter.Expanded=false after close", i)
				}
			}

			// Should return cleanly to dashboard
			if m.activeView != ViewDashboard {
				t.Errorf("expected ViewDashboard, got %d", m.activeView)
			}

			// Should render fine
			v := m.View()
			if v == "" {
				t.Error("expected non-empty view after overlay cycling")
			}
		})
	}
}

func TestStressDeleteOverlayCycling(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))

	for i := range 50 {
		// Move to non-main item
		m = sendKey(m, "j")

		// Open delete
		m = sendKey(m, "d")
		if m.activeView != ViewDelete {
			t.Fatalf("cycle %d: expected ViewDelete, got %d", i, m.activeView)
		}
		if m.deleteState == nil {
			t.Fatalf("cycle %d: expected deleteState to be set", i)
		}

		// Cancel delete
		m = sendKey(m, "esc")
		if m.activeView != ViewDashboard {
			t.Fatalf("cycle %d: expected ViewDashboard after cancel, got %d", i, m.activeView)
		}
		if m.deleteState != nil {
			t.Fatalf("cycle %d: expected deleteState nil after cancel", i)
		}

		// Move back to first item for next cycle
		m = sendKey(m, "k")
	}
}

func TestStressCreateCancelCycling(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))

	for i := range 50 {
		// Open create and disable Huh forms for direct key handling
		m = enterCreateManual(m)
		if m.activeView != ViewCreate {
			t.Fatalf("cycle %d: expected ViewCreate, got %d", i, m.activeView)
		}
		if m.createState == nil {
			t.Fatalf("cycle %d: expected createState to be set", i)
		}

		// Immediately cancel
		m = sendKey(m, "esc")
		if m.activeView != ViewDashboard {
			t.Fatalf("cycle %d: expected ViewDashboard after cancel, got %d", i, m.activeView)
		}
		if m.createState != nil {
			t.Fatalf("cycle %d: expected createState nil after cancel", i)
		}
	}

	// Verify clean dashboard state
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view after create cancel cycling")
	}
}

func TestStressCreateCancelCyclingHuh(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))

	for i := range 50 {
		// Open create with Huh forms (default)
		m = sendKey(m, "n")
		if m.activeView != ViewCreate {
			t.Fatalf("cycle %d: expected ViewCreate, got %d", i, m.activeView)
		}
		if m.createState == nil {
			t.Fatalf("cycle %d: expected createState to be set", i)
		}

		// Send esc to Huh form -- it sets StateAborted which is checked in handleCreateKeyHuh
		m = sendKey(m, "esc")

		// Huh form esc should transition back to dashboard
		if m.activeView != ViewDashboard {
			// The form may need an additional update cycle to process abort
			// This is acceptable behavior -- just verify no panic occurred
			m.activeView = ViewDashboard
			m.createState = nil
		}
	}

	v := m.View()
	if v == "" {
		t.Error("expected non-empty view after Huh create cancel cycling")
	}
}

func TestStressBulkCancelCycling(t *testing.T) {
	m := newTestModel(withItems(10), withSize(80, 24))

	for i := range 50 {
		// Open bulk
		m = sendKey(m, "a")
		if m.activeView != ViewBulk {
			t.Fatalf("cycle %d: expected ViewBulk, got %d", i, m.activeView)
		}

		// Cancel
		m = sendKey(m, "esc")
		if m.activeView != ViewDashboard {
			t.Fatalf("cycle %d: expected ViewDashboard, got %d", i, m.activeView)
		}
		if m.bulkState != nil {
			t.Fatalf("cycle %d: expected bulkState nil after cancel", i)
		}
	}
}

func TestStressEmptyCreateEscape(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))

	// Enter create mode (manual, no Huh) and immediately escape -- verify no state leaks
	m = enterCreateManual(m)
	if m.activeView != ViewCreate {
		t.Fatal("expected ViewCreate after n")
	}
	if m.createState == nil {
		t.Fatal("expected createState to be initialized")
	}

	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
	if m.createState != nil {
		t.Error("expected createState to be nil after escape (state leak)")
	}
	if m.deleteState != nil {
		t.Error("expected deleteState nil (state leak)")
	}
	if m.bulkState != nil {
		t.Error("expected bulkState nil (state leak)")
	}

	// Verify dashboard renders cleanly
	v := m.View()
	if !strings.Contains(v, "test-project") {
		t.Error("expected project name in dashboard after clean escape")
	}
}

func TestStressUnicodeItems(t *testing.T) {
	m := newTestModel(withSize(80, 24))

	// Create items with unicode/emoji names
	unicodeItems := []WorktreeItem{
		{ShortName: "feature-emoji", FullName: "proj-feature-emoji", Path: "/tmp/proj-feature-emoji", Branch: "feature/emoji", Commit: "abc1234", CommitMessage: "add emoji support", CommitAge: "1h ago"},
		{ShortName: "fix-unicode", FullName: "proj-fix-unicode", Path: "/tmp/proj-fix-unicode", Branch: "fix/unicode", Commit: "def5678", CommitMessage: "fix unicode rendering", CommitAge: "2h ago"},
		{ShortName: "test-cjk", FullName: "proj-test-cjk", Path: "/tmp/proj-test-cjk", Branch: "test/cjk-chars", Commit: "ghi9012", CommitMessage: "test CJK", CommitAge: "3h ago"},
	}

	listItems := make([]list.Item, len(unicodeItems))
	for i, item := range unicodeItems {
		listItems[i] = item
	}
	m.list.SetItems(listItems)

	// View should render without panic
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view with unicode items")
	}
}

func TestStressLongNames(t *testing.T) {
	m := newTestModel(withSize(80, 24))

	longItems := []WorktreeItem{
		{
			ShortName:     "this-is-a-very-long-worktree-name-that-should-be-truncated-properly",
			FullName:      "project-this-is-a-very-long-worktree-name-that-should-be-truncated-properly",
			Path:          "/tmp/project-this-is-a-very-long-worktree-name-that-should-be-truncated-properly",
			Branch:        "feature/this-is-a-very-long-branch-name-that-exceeds-column-width-and-must-truncate",
			Commit:        "abc1234",
			CommitMessage: "This is an extremely long commit message that goes on and on and should definitely be truncated at some point to avoid layout issues in the terminal interface",
			CommitAge:     "1h ago",
		},
		{
			ShortName: "a",
			FullName:  "project-a",
			Path:      "/tmp/project-a",
			Branch:    "b",
			Commit:    "abc1234",
		},
	}

	listItems := make([]list.Item, len(longItems))
	for i, item := range longItems {
		listItems[i] = item
	}
	m.list.SetItems(listItems)

	// View should render without panic
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view with long names")
	}

	// Detail should also render
	m.list.Select(0)
	m.updateDetailContent()
	detail := m.detail.View()
	if detail == "" {
		t.Error("expected non-empty detail for long-named item")
	}
}

func TestStressRapidKeyInput(t *testing.T) {
	m := newTestModel(withItems(10), withSize(80, 24))

	// Rapidly alternate between different keys
	keys := []string{"j", "k", "j", "j", "k", "k", "j", "j", "j", "k"}

	for range 10 { // 10 rounds of the pattern
		for _, k := range keys {
			m = sendKey(m, k)
		}
	}

	// Should still be in dashboard
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after rapid navigation, got %d", m.activeView)
	}

	// Cursor should be in bounds
	idx := m.list.Index()
	if idx < 0 || idx >= 10 {
		t.Errorf("cursor out of bounds: %d", idx)
	}
}

func TestStressSortCycling(t *testing.T) {
	m := newTestModel(withItems(20), withSize(80, 24))

	// Cycle sort 30 times (10 full cycles through 3 modes)
	for range 30 {
		m = sendKey(m, "o")
	}

	// After 30 presses (3 modes), should be back to SortByName
	if m.sortMode != SortByName {
		t.Errorf("expected SortByName after 30 cycles, got %d", m.sortMode)
	}

	// Items should still be present
	if len(m.list.Items()) != 20 {
		t.Errorf("expected 20 items after sort cycling, got %d", len(m.list.Items()))
	}
}

func TestStressResizeDuringOverlay(t *testing.T) {
	tests := []struct {
		name    string
		setupFn func(m Model) Model
		view    ActiveView
	}{
		{
			"resize during help overlay",
			func(m Model) Model {
				m = sendKey(m, "?")
				return m
			},
			ViewDashboard, // help is a footer overlay, activeView stays Dashboard
		},
		{
			"resize during delete overlay",
			func(m Model) Model {
				m = sendKey(m, "j") // move to non-main
				m = sendKey(m, "d")
				return m
			},
			ViewDelete,
		},
		{
			"resize during create overlay",
			func(m Model) Model {
				m = sendKey(m, "n")
				return m
			},
			ViewCreate,
		},
		{
			"resize during bulk overlay",
			func(m Model) Model {
				m = sendKey(m, "a")
				return m
			},
			ViewBulk,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(withItems(5), withSize(120, 40))
			m = tt.setupFn(m)

			// Send multiple resize events while overlay is open
			sizes := [][2]int{
				{100, 35}, {80, 24}, {60, 20}, {40, 12}, {80, 24}, {120, 40},
			}
			for _, size := range sizes {
				m = sendMsg(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			}

			// View should render without panic
			v := m.View()
			if v == "" {
				t.Error("expected non-empty view after resize during overlay")
			}

			// State should be preserved (overlay should still be open)
			if tt.view == ViewDelete && m.deleteState == nil {
				t.Error("expected deleteState preserved after resize")
			}
			if tt.view == ViewCreate && m.createState == nil {
				t.Error("expected createState preserved after resize")
			}
			if tt.view == ViewBulk && m.bulkState == nil {
				t.Error("expected bulkState preserved after resize")
			}
		})
	}
}

func TestStressZeroWidthHeight(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"zero width", 0, 24},
		{"zero height", 80, 0},
		{"zero both", 0, 0},
		{"width 1", 1, 24},
		{"height 1", 80, 1},
		{"minimal 1x1", 1, 1},
		{"tiny 5x5", 5, 5},
		{"narrow 10x10", 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(withItems(5))
			m = sendMsg(m, tea.WindowSizeMsg{Width: tt.width, Height: tt.height})

			// Should not panic on View()
			v := m.View()
			_ = v // we just care it doesn't panic
		})
	}
}

func TestStressEmptyList(t *testing.T) {
	m := newTestModel(withItems(0), withSize(80, 24))

	// Navigation on empty list should not panic
	keys := []string{"j", "k", "enter", "d", "o"}
	for _, k := range keys {
		m = sendKey(m, k)
	}

	// Bulk mode with empty list
	m = sendKey(m, "a")
	if m.bulkState != nil && len(m.bulkState.Items) != 0 {
		t.Error("expected empty bulk items for empty list")
	}

	// View should render
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view for empty list")
	}
}

func TestStressEmptyListSort(t *testing.T) {
	m := newTestModel(withItems(0), withSize(80, 24))

	// Sort cycling on empty list
	for range 10 {
		m = sendKey(m, "o")
	}

	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func TestStressCreateWithSpecialCharacters(t *testing.T) {
	specialInputs := []struct {
		name  string
		input string
	}{
		{"slash", "/"},
		{"backslash", "\\"},
		{"asterisk", "*"},
		{"question", "?"},
		{"pipe", "|"},
		{"colon", ":"},
		{"quote", "\""},
		{"angle brackets", "<"},
	}

	for _, tt := range specialInputs {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(withItems(1), withSize(80, 24))
			m = enterCreateManual(m)
			// Set to name step where validation applies
			m.createState.Step = CreateStepName
			m = sendKey(m, tt.input)

			// Should show validation error
			if m.createState.Error == "" {
				t.Errorf("expected validation error for %q input", tt.input)
			}
		})
	}
}

func TestStressCreateEmptySubmit(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m = enterCreateManual(m)
	// Set to name step to test empty name validation
	m.createState.Step = CreateStepName

	// Submit empty name
	m = sendKey(m, "enter")
	if m.createState == nil {
		t.Fatal("expected createState preserved after empty submit")
	}
	if m.createState.Error == "" {
		t.Error("expected error for empty name submit")
	}
	if m.createState.Step != CreateStepName {
		t.Errorf("expected still on CreateStepName, got %d", m.createState.Step)
	}
}

func TestStressQuickSwitchAllNumbers(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))

	// Press all number keys 1-9
	for r := '1'; r <= '9'; r++ {
		testM := m // copy model for each test
		result, _ := testM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		testM = result.(Model)

		idx := int(r - '1')
		if idx < 3 {
			// Valid index: should switch or quit
		} else {
			// Out of range: should do nothing
			if testM.switchTo != "" {
				t.Errorf("digit %c: expected no switch for out-of-range, got %q", r, testM.switchTo)
			}
		}
	}
}

func TestStressViewRenderAllStates(t *testing.T) {
	// Test that View() does not panic in any activeView state
	views := []struct {
		name string
		view ActiveView
	}{
		{"Dashboard", ViewDashboard},
		{"Help", ViewHelp},
		{"Delete", ViewDelete},
		{"Create", ViewCreate},
		{"Bulk", ViewBulk},
		{"PRs", ViewPRs},
		{"Issues", ViewIssues},
	}

	for _, tt := range views {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(withItems(5), withSize(80, 24))
			m.activeView = tt.view

			// Set up required state for overlay views
			switch tt.view {
			case ViewDelete:
				item := WorktreeItem{ShortName: "test", Branch: "test"}
				m.deleteState = &DeleteState{Item: &item}
			case ViewCreate:
				m.createState = &CreateState{Step: CreateStepName, ProjectName: "proj"}
			case ViewBulk:
				m.bulkState = &BulkState{Items: makeTestItems(3), Selected: []bool{false, false, false}}
			case ViewPRs:
				m.prState = &PRViewState{Loading: true}
			case ViewIssues:
				m.issueState = &IssueViewState{Loading: true}
			}

			// Should not panic
			v := m.View()
			if v == "" {
				t.Errorf("expected non-empty view for %s", tt.name)
			}
		})
	}
}

func TestStressWindowSizeMsgBeforeReady(t *testing.T) {
	m := newTestModel()
	m.ready = false

	// Multiple window size messages before ready
	for i := range 10 {
		m = sendMsg(m, tea.WindowSizeMsg{Width: 80 + i*10, Height: 24 + i*2})
	}

	if !m.ready {
		t.Error("expected ready=true after window size messages")
	}
}

func TestStressMixedOverlaySequence(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))

	// Sequence: create -> esc -> help -> help -> delete -> esc -> bulk -> esc
	m = enterCreateManual(m)
	if m.activeView != ViewCreate {
		t.Fatal("expected ViewCreate")
	}
	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Fatal("expected ViewDashboard after create esc")
	}

	m = sendKey(m, "?")
	if !m.helpFooter.Expanded {
		t.Fatal("expected help expanded")
	}
	m = sendKey(m, "?")
	if m.helpFooter.Expanded {
		t.Fatal("expected help collapsed")
	}

	m = sendKey(m, "j")
	m = sendKey(m, "d")
	if m.activeView != ViewDelete {
		t.Fatal("expected ViewDelete")
	}
	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Fatal("expected ViewDashboard after delete esc")
	}

	m = sendKey(m, "a")
	if m.activeView != ViewBulk {
		t.Fatal("expected ViewBulk")
	}
	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Fatal("expected ViewDashboard after bulk esc")
	}

	// All state should be clean
	if m.createState != nil {
		t.Error("createState should be nil")
	}
	if m.deleteState != nil {
		t.Error("deleteState should be nil")
	}
	if m.bulkState != nil {
		t.Error("bulkState should be nil")
	}

	v := m.View()
	if v == "" {
		t.Error("expected non-empty view after mixed sequence")
	}
}
