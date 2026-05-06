package tui

import (
	"strings"
	"testing"
)

func TestNewCheckoutState(t *testing.T) {
	item := WorktreeItem{
		ShortName: "feature-auth",
		Branch:    "feature/auth",
		Path:      "/tmp/test-project-feature-auth",
	}
	s := NewCheckoutState(item)

	if s.Step != CheckoutStepBranch {
		t.Errorf("expected CheckoutStepBranch, got %d", s.Step)
	}
	if s.Item.ShortName != "feature-auth" {
		t.Errorf("expected item 'feature-auth', got %q", s.Item.ShortName)
	}
	if s.Stepper == nil {
		t.Fatal("expected stepper to be initialized")
	}
	if len(s.Stepper.Steps) != 3 {
		t.Errorf("expected 3 stepper steps, got %d", len(s.Stepper.Steps))
	}
	if s.Switching {
		t.Error("expected Switching=false initially")
	}
}

func TestCheckoutOverlay_OpenClose(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	// Move to non-main item (index 1)
	m = sendKey(m, "j")
	m = sendKey(m, "b")
	if m.activeView != ViewCheckout {
		t.Errorf("expected ViewCheckout, got %d", m.activeView)
	}
	if m.checkoutState == nil {
		t.Fatal("expected checkoutState to be set")
	}
	if m.checkoutState.Step != CheckoutStepBranch {
		t.Errorf("expected CheckoutStepBranch, got %d", m.checkoutState.Step)
	}

	// Esc should close
	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after esc, got %d", m.activeView)
	}
	if m.checkoutState != nil {
		t.Error("expected checkoutState to be nil after close")
	}
}

func TestCheckoutOverlay_MainWorktreeBlocked(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	// Item 0 is main — b should not open checkout
	m = sendKey(m, "b")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard for main worktree, got %d", m.activeView)
	}
	if m.checkoutState != nil {
		t.Error("expected checkoutState nil for main worktree")
	}
}

func TestCheckoutOverlay_NilState(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = nil
	m = sendKey(m, "enter")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard for nil state, got %d", m.activeView)
	}
}

func TestCheckoutOverlay_BranchesMsg(t *testing.T) {
	item := WorktreeItem{
		ShortName: "feature-auth",
		Branch:    "feature/auth",
		Path:      "/tmp/test-project-feature-auth",
	}
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = NewCheckoutState(item)

	branches := []string{"main", "develop", "feature/auth", "fix/bug"}
	usedBranches := map[string]bool{"feature/auth": true}
	m = sendMsg(m, checkoutBranchesMsg{branches: branches, usedBranches: usedBranches})

	if m.checkoutState == nil {
		t.Fatal("expected checkoutState to be set")
	}
	// feature/auth is used by the current worktree, so available branches
	// should not include it
	for _, b := range m.checkoutState.Branches {
		if b == "feature/auth" {
			t.Error("expected feature/auth to be filtered out (used by another worktree)")
		}
	}
	if len(m.checkoutState.Branches) != 3 {
		t.Errorf("expected 3 available branches, got %d", len(m.checkoutState.Branches))
	}
}

func TestCheckoutOverlay_BranchesMsgError(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = NewCheckoutState(WorktreeItem{ShortName: "test"})

	m = sendMsg(m, checkoutBranchesMsg{err: errTest})
	if m.checkoutState.Err == nil {
		t.Error("expected error to be set on branch listing failure")
	}
}

func TestCheckoutOverlay_WIPCheckMsg(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = NewCheckoutState(WorktreeItem{ShortName: "test", Branch: "test"})

	m = sendMsg(m, wipCheckMsg{hasWIP: true, files: []string{"file.go", "other.go"}})
	if !m.checkoutState.HasWIP {
		t.Error("expected HasWIP=true")
	}
	if len(m.checkoutState.WIPFiles) != 2 {
		t.Errorf("expected 2 WIP files, got %d", len(m.checkoutState.WIPFiles))
	}
}

func TestCheckoutOverlay_WIPCheckMsgError(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = NewCheckoutState(WorktreeItem{ShortName: "test"})

	m = sendMsg(m, wipCheckMsg{err: errTest})
	if m.checkoutState.Err == nil {
		t.Error("expected error to be set on WIP check failure")
	}
}

func TestCheckoutOverlay_SelectBranchCleanSkipsWIP(t *testing.T) {
	item := WorktreeItem{
		ShortName: "feature-auth",
		Branch:    "feature/auth",
		Path:      "/tmp/test-project-feature-auth",
	}
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	s := NewCheckoutState(item)
	s.Branches = []string{"main", "develop", "fix/bug"}
	s.HasWIP = false
	s.WIPCheckDone = true
	s.BranchFilterInput.Focus()
	m.checkoutState = s

	// Select first branch (main) via enter
	m = sendKey(m, "enter")
	if m.checkoutState.SelectedBranch != "main" {
		t.Errorf("expected 'main' selected, got %q", m.checkoutState.SelectedBranch)
	}
	// Clean worktree should skip WIP step and go to confirm
	if m.checkoutState.Step != CheckoutStepConfirm {
		t.Errorf("expected CheckoutStepConfirm (clean worktree skips WIP), got %d", m.checkoutState.Step)
	}
	if m.checkoutState.Stepper.Current != 2 {
		t.Errorf("expected stepper on step 2, got %d", m.checkoutState.Stepper.Current)
	}
}

func TestCheckoutOverlay_SelectBranchDirtyShowsWIP(t *testing.T) {
	item := WorktreeItem{
		ShortName: "feature-auth",
		Branch:    "feature/auth",
		Path:      "/tmp/test-project-feature-auth",
	}
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	s := NewCheckoutState(item)
	s.Branches = []string{"main", "develop", "fix/bug"}
	s.HasWIP = true
	s.WIPCheckDone = true
	s.WIPFiles = []string{"file.go"}
	s.BranchFilterInput.Focus()
	m.checkoutState = s

	// Select first branch
	m = sendKey(m, "enter")
	if m.checkoutState.Step != CheckoutStepWIP {
		t.Errorf("expected CheckoutStepWIP (dirty worktree), got %d", m.checkoutState.Step)
	}
	if m.checkoutState.Stepper.Current != 1 {
		t.Errorf("expected stepper on step 1, got %d", m.checkoutState.Stepper.Current)
	}
}

func TestCheckoutOverlay_NoBranches(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	s := NewCheckoutState(WorktreeItem{ShortName: "test", Branch: "test"})
	s.Branches = []string{}
	s.WIPCheckDone = true
	s.BranchFilterInput.Focus()
	m.checkoutState = s

	// Enter with no branches should not crash
	m = sendKey(m, "enter")
	// Should stay on branch step since there's nothing to select
	if m.checkoutState.Step != CheckoutStepBranch {
		t.Errorf("expected CheckoutStepBranch with empty list, got %d", m.checkoutState.Step)
	}
}

func TestCheckoutOverlay_EnterBlockedBeforeWIPCheck(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	s := NewCheckoutState(WorktreeItem{ShortName: "test", Branch: "test"})
	s.Branches = []string{"main", "develop"}
	s.WIPCheckDone = false // WIP check not yet received
	s.BranchFilterInput.Focus()
	m.checkoutState = s

	// Enter should be blocked — stays on branch step
	m = sendKey(m, "enter")
	if m.checkoutState.Step != CheckoutStepBranch {
		t.Errorf("expected Enter blocked before WIP check, got step %d", m.checkoutState.Step)
	}
	if m.checkoutState.SelectedBranch != "" {
		t.Errorf("expected no branch selected, got %q", m.checkoutState.SelectedBranch)
	}

	// After WIP check arrives, Enter should work
	m = sendMsg(m, wipCheckMsg{hasWIP: false})
	if !m.checkoutState.WIPCheckDone {
		t.Error("expected WIPCheckDone=true after msg")
	}
	m = sendKey(m, "enter")
	if m.checkoutState.Step != CheckoutStepConfirm {
		t.Errorf("expected CheckoutStepConfirm after WIP check done, got %d", m.checkoutState.Step)
	}
}

func TestCheckoutOverlay_ConfirmBackResetsStash(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = &CheckoutState{
		Step:           CheckoutStepConfirm,
		Item:           WorktreeItem{ShortName: "test", Branch: "test"},
		SelectedBranch: "develop",
		HasWIP:         true,
		Stash:          true, // previously selected stash
		Stepper:        NewStepper("Branch", "WIP", "Confirm"),
	}
	m.checkoutState.Stepper.Current = 2

	m = sendKey(m, "backspace")
	if m.checkoutState.Stash {
		t.Error("expected Stash=false after back-navigation from confirm")
	}
}

func TestCheckoutOverlay_BranchAlreadyUsedShown(t *testing.T) {
	item := WorktreeItem{
		ShortName: "feature-auth",
		Branch:    "feature/auth",
		Path:      "/tmp/test-project-feature-auth",
	}
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	s := NewCheckoutState(item)
	// Simulate receiving branches where feature/auth is filtered out
	s.Branches = []string{"main", "develop"}
	m.checkoutState = s

	// Verify the branches are filtered via the checkoutBranchesMsg handler
	m = sendMsg(m, checkoutBranchesMsg{
		branches:     []string{"main", "develop", "feature/auth"},
		usedBranches: map[string]bool{"feature/auth": true},
	})
	for _, b := range m.checkoutState.Branches {
		if b == "feature/auth" {
			t.Error("current branch should be filtered out")
		}
	}
}

func TestCheckoutOverlay_WIPStepStash(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = &CheckoutState{
		Step:           CheckoutStepWIP,
		Item:           WorktreeItem{ShortName: "test", Branch: "test"},
		SelectedBranch: "develop",
		HasWIP:         true,
		WIPFiles:       []string{"file.go"},
		WIPCursor:      0, // stash option
		Stepper:        NewStepper("Branch", "WIP", "Confirm"),
	}
	m.checkoutState.Stepper.Current = 1

	m = sendKey(m, "enter")
	if !m.checkoutState.Stash {
		t.Error("expected Stash=true when selecting stash option")
	}
	if m.checkoutState.Step != CheckoutStepConfirm {
		t.Errorf("expected CheckoutStepConfirm, got %d", m.checkoutState.Step)
	}
}

func TestCheckoutOverlay_WIPStepCancel(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = &CheckoutState{
		Step:           CheckoutStepWIP,
		Item:           WorktreeItem{ShortName: "test", Branch: "test"},
		SelectedBranch: "develop",
		HasWIP:         true,
		WIPFiles:       []string{"file.go"},
		WIPCursor:      1, // cancel option
		Stepper:        NewStepper("Branch", "WIP", "Confirm"),
	}
	m.checkoutState.Stepper.Current = 1

	m = sendKey(m, "enter")
	// Cancel should close the overlay
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after WIP cancel, got %d", m.activeView)
	}
	if m.checkoutState != nil {
		t.Error("expected checkoutState nil after WIP cancel")
	}
}

func TestCheckoutOverlay_WIPStepBackGoesToBranch(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = &CheckoutState{
		Step:           CheckoutStepWIP,
		Item:           WorktreeItem{ShortName: "test", Branch: "test"},
		SelectedBranch: "develop",
		HasWIP:         true,
		Stepper:        NewStepper("Branch", "WIP", "Confirm"),
	}
	m.checkoutState.Stepper.Current = 1

	m = sendKey(m, "backspace")
	if m.checkoutState.Step != CheckoutStepBranch {
		t.Errorf("expected CheckoutStepBranch after back, got %d", m.checkoutState.Step)
	}
	if m.checkoutState.Stepper.Current != 0 {
		t.Errorf("expected stepper on step 0, got %d", m.checkoutState.Stepper.Current)
	}
}

func TestCheckoutOverlay_ConfirmEsc(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = &CheckoutState{
		Step:           CheckoutStepConfirm,
		Item:           WorktreeItem{ShortName: "test", Branch: "test"},
		SelectedBranch: "develop",
		Stepper:        NewStepper("Branch", "WIP", "Confirm"),
	}
	m.checkoutState.Stepper.Current = 2

	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func TestCheckoutOverlay_ConfirmBackNoWIP(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = &CheckoutState{
		Step:           CheckoutStepConfirm,
		Item:           WorktreeItem{ShortName: "test", Branch: "test"},
		SelectedBranch: "develop",
		HasWIP:         false,
		Stepper:        NewStepper("Branch", "WIP", "Confirm"),
	}
	m.checkoutState.Stepper.Current = 2

	m = sendKey(m, "backspace")
	if m.checkoutState.Step != CheckoutStepBranch {
		t.Errorf("expected CheckoutStepBranch after back with no WIP, got %d", m.checkoutState.Step)
	}
}

func TestCheckoutOverlay_ConfirmBackWithWIP(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = &CheckoutState{
		Step:           CheckoutStepConfirm,
		Item:           WorktreeItem{ShortName: "test", Branch: "test"},
		SelectedBranch: "develop",
		HasWIP:         true,
		Stepper:        NewStepper("Branch", "WIP", "Confirm"),
	}
	m.checkoutState.Stepper.Current = 2

	m = sendKey(m, "backspace")
	if m.checkoutState.Step != CheckoutStepWIP {
		t.Errorf("expected CheckoutStepWIP after back with WIP, got %d", m.checkoutState.Step)
	}
}

func TestCheckoutOverlay_SwitchingIgnoresInput(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = &CheckoutState{
		Step:           CheckoutStepConfirm,
		Item:           WorktreeItem{ShortName: "test", Branch: "test"},
		SelectedBranch: "develop",
		Switching:      true,
		Stepper:        NewStepper("Branch", "WIP", "Confirm"),
	}
	m.checkoutState.Stepper.Current = 2

	m = sendKey(m, "esc")
	if m.activeView != ViewCheckout {
		t.Errorf("expected ViewCheckout while switching, got %d", m.activeView)
	}
}

func TestCheckoutOverlay_ConfirmWithNilManagers(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = &CheckoutState{
		Step:           CheckoutStepConfirm,
		Item:           WorktreeItem{ShortName: "test", Branch: "test", Path: "/tmp/test"},
		SelectedBranch: "develop",
		Stepper:        NewStepper("Branch", "WIP", "Confirm"),
	}
	m.checkoutState.Stepper.Current = 2

	// worktreeMgr is nil on test model — confirm should still set Switching
	// because checkoutBranchCmd only needs the path, not a manager
	m = sendKey(m, "enter")
	if !m.checkoutState.Switching {
		t.Error("expected Switching=true after confirm")
	}
}

func TestCheckoutCompleteMsg_Success(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = &CheckoutState{
		Step:           CheckoutStepConfirm,
		Item:           WorktreeItem{ShortName: "test", Branch: "test"},
		SelectedBranch: "develop",
		Switching:      true,
		Stepper:        NewStepper("Branch", "WIP", "Confirm"),
	}

	m = sendMsg(m, checkoutCompleteMsg{branch: "develop"})
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after checkout complete, got %d", m.activeView)
	}
	if m.checkoutState != nil {
		t.Error("expected checkoutState nil after success")
	}
}

func TestCheckoutCompleteMsg_Error(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = &CheckoutState{
		Step:           CheckoutStepConfirm,
		Item:           WorktreeItem{ShortName: "test", Branch: "test"},
		SelectedBranch: "develop",
		Switching:      true,
		Stepper:        NewStepper("Branch", "WIP", "Confirm"),
	}

	m = sendMsg(m, checkoutCompleteMsg{err: errTest})
	if m.activeView != ViewCheckout {
		t.Errorf("expected ViewCheckout after error, got %d", m.activeView)
	}
	if m.checkoutState.Err == nil {
		t.Error("expected error on checkoutState")
	}
	if m.checkoutState.Switching {
		t.Error("expected Switching=false after error")
	}
}

func TestCheckoutOverlay_NavigateBranchList(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	s := NewCheckoutState(WorktreeItem{ShortName: "test", Branch: "test"})
	s.Branches = []string{"main", "develop", "fix/bug"}
	s.BranchFilterInput.Focus()
	m.checkoutState = s

	// Down should move cursor
	m = sendKey(m, "down")
	if m.checkoutState.BranchCursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.checkoutState.BranchCursor)
	}

	// Up should move back
	m = sendKey(m, "up")
	if m.checkoutState.BranchCursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.checkoutState.BranchCursor)
	}

	// Up at top stays at 0
	m = sendKey(m, "up")
	if m.checkoutState.BranchCursor != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", m.checkoutState.BranchCursor)
	}
}

// Render tests

func TestRenderCheckout_AllSteps(t *testing.T) {
	t.Run("branch step", func(t *testing.T) {
		s := &CheckoutState{
			Step:              CheckoutStepBranch,
			Item:              WorktreeItem{ShortName: "feature-auth", Branch: "feature/auth"},
			Branches:          []string{"main", "develop", "fix/bug"},
			BranchFilterInput: NewCheckoutState(WorktreeItem{}).BranchFilterInput,
			Stepper:           NewStepper("Branch", "WIP", "Confirm"),
		}
		v := renderCheckout(s, 80)
		if v == "" {
			t.Fatal("expected non-empty render")
		}
		if !strings.Contains(v, "Switch Branch") {
			t.Error("expected 'Switch Branch' title")
		}
		if !strings.Contains(v, "main") {
			t.Error("expected branch 'main' in render")
		}
	})

	t.Run("branch step loading", func(t *testing.T) {
		s := &CheckoutState{
			Step:              CheckoutStepBranch,
			Item:              WorktreeItem{ShortName: "feature-auth", Branch: "feature/auth"},
			BranchFilterInput: NewCheckoutState(WorktreeItem{}).BranchFilterInput,
			Stepper:           NewStepper("Branch", "WIP", "Confirm"),
		}
		v := renderCheckout(s, 80)
		if !strings.Contains(v, "Loading branches") {
			t.Error("expected loading message when branches are nil")
		}
	})

	t.Run("WIP step", func(t *testing.T) {
		s := &CheckoutState{
			Step:           CheckoutStepWIP,
			Item:           WorktreeItem{ShortName: "feature-auth", Branch: "feature/auth"},
			SelectedBranch: "develop",
			HasWIP:         true,
			WIPFiles:       []string{"file.go", "other.go"},
			Stepper:        NewStepper("Branch", "WIP", "Confirm"),
		}
		s.Stepper.Current = 1
		v := renderCheckout(s, 80)
		if !strings.Contains(v, "2 files") {
			t.Error("expected WIP file count in render")
		}
		if !strings.Contains(v, "Stash") {
			t.Error("expected stash option in WIP step")
		}
	})

	t.Run("confirm step", func(t *testing.T) {
		s := &CheckoutState{
			Step:           CheckoutStepConfirm,
			Item:           WorktreeItem{ShortName: "feature-auth", Branch: "feature/auth"},
			SelectedBranch: "develop",
			Stepper:        NewStepper("Branch", "WIP", "Confirm"),
		}
		s.Stepper.Current = 2
		v := renderCheckout(s, 80)
		if !strings.Contains(v, "feature/auth") {
			t.Error("expected current branch in confirm step")
		}
		if !strings.Contains(v, "develop") {
			t.Error("expected target branch in confirm step")
		}
	})

	t.Run("switching state", func(t *testing.T) {
		s := &CheckoutState{
			Step:           CheckoutStepConfirm,
			Item:           WorktreeItem{ShortName: "test"},
			SelectedBranch: "develop",
			Switching:      true,
			Stepper:        NewStepper("Branch", "WIP", "Confirm"),
		}
		v := renderCheckout(s, 80)
		if !strings.Contains(v, "Switching") {
			t.Error("expected switching message")
		}
	})

	t.Run("error state", func(t *testing.T) {
		s := &CheckoutState{
			Step:              CheckoutStepBranch,
			Item:              WorktreeItem{ShortName: "test"},
			Err:               errTest,
			BranchFilterInput: NewCheckoutState(WorktreeItem{}).BranchFilterInput,
			Stepper:           NewStepper("Branch", "WIP", "Confirm"),
		}
		v := renderCheckout(s, 80)
		if !strings.Contains(v, "test error") {
			t.Error("expected error text in render")
		}
	})
}

func TestRenderCheckout_BoundaryWidths(t *testing.T) {
	s := &CheckoutState{
		Step:              CheckoutStepBranch,
		Item:              WorktreeItem{ShortName: "test", Branch: "test"},
		Branches:          []string{"main"},
		BranchFilterInput: NewCheckoutState(WorktreeItem{}).BranchFilterInput,
		Stepper:           NewStepper("Branch", "WIP", "Confirm"),
	}
	// Narrow width
	v := renderCheckout(s, 40)
	if v == "" {
		t.Fatal("expected non-empty render at narrow width")
	}
	// Wide width
	v = renderCheckout(s, 200)
	if v == "" {
		t.Fatal("expected non-empty render at wide width")
	}
}

func TestRenderCheckout_ConfirmWithStash(t *testing.T) {
	s := &CheckoutState{
		Step:           CheckoutStepConfirm,
		Item:           WorktreeItem{ShortName: "test", Branch: "test"},
		SelectedBranch: "develop",
		HasWIP:         true,
		Stash:          true,
		WIPFiles:       []string{"file.go"},
		Stepper:        NewStepper("Branch", "WIP", "Confirm"),
	}
	s.Stepper.Current = 2
	v := renderCheckout(s, 80)
	if !strings.Contains(v, "stash") {
		t.Error("expected stash info in confirm step")
	}
}

func TestCheckoutOverlay_FilterBranches(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	s := NewCheckoutState(WorktreeItem{ShortName: "test", Branch: "test"})
	s.Branches = []string{"main", "develop", "fix/bug", "feature/auth"}
	s.BranchFilterInput.Focus()
	m.checkoutState = s

	// Type 'f' to filter
	m = sendKey(m, "f")
	filter := m.checkoutState.BranchFilterInput.Value()
	if filter != "f" {
		t.Errorf("expected filter 'f', got %q", filter)
	}
	// Cursor should reset to 0 on filter change
	if m.checkoutState.BranchCursor != 0 {
		t.Errorf("expected cursor reset to 0, got %d", m.checkoutState.BranchCursor)
	}
}

func TestCheckoutOverlay_WIPNavigateUpDown(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewCheckout
	m.checkoutState = &CheckoutState{
		Step:           CheckoutStepWIP,
		Item:           WorktreeItem{ShortName: "test", Branch: "test"},
		SelectedBranch: "develop",
		HasWIP:         true,
		WIPCursor:      0,
		Stepper:        NewStepper("Branch", "WIP", "Confirm"),
	}
	m.checkoutState.Stepper.Current = 1

	m = sendKey(m, "down")
	if m.checkoutState.WIPCursor != 1 {
		t.Errorf("expected WIP cursor at 1, got %d", m.checkoutState.WIPCursor)
	}

	// Down at max stays at 1 (only 2 options: stash, cancel)
	m = sendKey(m, "down")
	if m.checkoutState.WIPCursor != 1 {
		t.Errorf("expected WIP cursor clamped at 1, got %d", m.checkoutState.WIPCursor)
	}

	m = sendKey(m, "up")
	if m.checkoutState.WIPCursor != 0 {
		t.Errorf("expected WIP cursor at 0, got %d", m.checkoutState.WIPCursor)
	}
}
