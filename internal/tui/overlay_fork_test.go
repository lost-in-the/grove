package tui

import (
	"strings"
	"testing"
)

func TestNewForkState(t *testing.T) {
	source := WorktreeItem{
		ShortName: "feature-auth",
		Branch:    "feature-auth",
		Path:      "/tmp/test-project-feature-auth",
	}
	s := NewForkState(source)

	if s.Step != ForkStepName {
		t.Errorf("expected ForkStepName, got %d", s.Step)
	}
	if s.Source.ShortName != "feature-auth" {
		t.Errorf("expected source 'feature-auth', got %q", s.Source.ShortName)
	}
	if s.Stepper == nil {
		t.Fatal("expected stepper to be initialized")
	}
	if len(s.Stepper.Steps) != 3 {
		t.Errorf("expected 3 stepper steps, got %d", len(s.Stepper.Steps))
	}
}

func TestForkOverlay_OpenClose(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	// Move to non-main item
	m = sendKey(m, "j")
	m = sendKey(m, "f")
	if m.activeView != ViewFork {
		t.Errorf("expected ViewFork, got %d", m.activeView)
	}
	if m.forkState == nil {
		t.Fatal("expected forkState to be set")
	}
	if m.forkState.Step != ForkStepName {
		t.Errorf("expected ForkStepName, got %d", m.forkState.Step)
	}

	// Esc should close (once form is initialized it handles esc as abort)
	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after esc, got %d", m.activeView)
	}
	if m.forkState != nil {
		t.Error("expected forkState to be nil after close")
	}
}

func TestForkOverlay_NilState(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewFork
	m.forkState = nil
	m = sendKey(m, "enter")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard for nil state, got %d", m.activeView)
	}
}

func TestForkOverlay_WIPCheckSkipsStep(t *testing.T) {
	source := WorktreeItem{
		ShortName: "feature-auth",
		Branch:    "feature-auth",
		Path:      "/tmp/test-project-feature-auth",
	}
	s := NewForkState(source)

	// Simulate WIP check with no WIP — should skip to confirm
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewFork
	m.forkState = s
	m = sendMsg(m, forkWIPCheckMsg{hasWIP: false, files: nil})
	if m.forkState.Step != ForkStepConfirm {
		t.Errorf("expected ForkStepConfirm when no WIP, got %d", m.forkState.Step)
	}
}

func TestForkOverlay_WIPCheckKeepsStep(t *testing.T) {
	source := WorktreeItem{
		ShortName: "feature-auth",
		Branch:    "feature-auth",
		Path:      "/tmp/test-project-feature-auth",
	}
	s := NewForkState(source)

	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewFork
	m.forkState = s
	m = sendMsg(m, forkWIPCheckMsg{hasWIP: true, files: []string{"file.go", "other.go"}})
	if m.forkState.Step != ForkStepName {
		t.Errorf("expected ForkStepName when WIP present, got %d", m.forkState.Step)
	}
	if !m.forkState.HasWIP {
		t.Error("expected HasWIP=true")
	}
	if len(m.forkState.WIPFiles) != 2 {
		t.Errorf("expected 2 WIP files, got %d", len(m.forkState.WIPFiles))
	}
}

func TestForkOverlay_ConfirmEsc(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewFork
	m.forkState = &ForkState{
		Step:    ForkStepConfirm,
		Source:  WorktreeItem{ShortName: "test"},
		Name:    "new-feature",
		Stepper: NewStepper("Name", "WIP", "Confirm"),
	}
	m.forkState.Stepper.Current = 2

	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func TestForkOverlay_ConfirmBackNoWIP(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewFork
	m.forkState = &ForkState{
		Step:    ForkStepConfirm,
		Source:  WorktreeItem{ShortName: "test"},
		Name:    "new-feature",
		HasWIP:  false,
		Stepper: NewStepper("Name", "WIP", "Confirm"),
	}
	m.forkState.Stepper.Current = 2

	m = sendKey(m, "backspace")
	if m.forkState.Step != ForkStepName {
		t.Errorf("expected ForkStepName after back with no WIP, got %d", m.forkState.Step)
	}
}

func TestForkOverlay_ConfirmBackWithWIP(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewFork
	m.forkState = &ForkState{
		Step:    ForkStepConfirm,
		Source:  WorktreeItem{ShortName: "test"},
		Name:    "new-feature",
		HasWIP:  true,
		Stepper: NewStepper("Name", "WIP", "Confirm"),
	}
	m.forkState.Stepper.Current = 2

	m = sendKey(m, "backspace")
	if m.forkState.Step != ForkStepWIP {
		t.Errorf("expected ForkStepWIP after back with WIP, got %d", m.forkState.Step)
	}
}

func TestForkOverlay_ForkingIgnoresInput(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewFork
	m.forkState = &ForkState{
		Step:    ForkStepConfirm,
		Source:  WorktreeItem{ShortName: "test"},
		Name:    "new-feature",
		Forking: true,
		Stepper: NewStepper("Name", "WIP", "Confirm"),
	}
	m.forkState.Stepper.Current = 2

	m = sendKey(m, "esc")
	// Should still be in fork view since forking is in progress
	if m.activeView != ViewFork {
		t.Errorf("expected ViewFork while forking, got %d", m.activeView)
	}
}

func TestForkCompleteMsg_Success(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewFork
	m.forkState = &ForkState{
		Step:    ForkStepConfirm,
		Source:  WorktreeItem{ShortName: "test"},
		Name:    "new-feature",
		Forking: true,
		Stepper: NewStepper("Name", "WIP", "Confirm"),
	}

	m = sendMsg(m, forkCompleteMsg{name: "new-feature", path: "/tmp/new"})
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after fork complete, got %d", m.activeView)
	}
	if m.forkState != nil {
		t.Error("expected forkState nil after success")
	}
	if m.pendingSelect != "new-feature" {
		t.Errorf("expected pendingSelect='new-feature', got %q", m.pendingSelect)
	}
}

func TestForkCompleteMsg_Error(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewFork
	m.forkState = &ForkState{
		Step:    ForkStepConfirm,
		Source:  WorktreeItem{ShortName: "test"},
		Name:    "new-feature",
		Forking: true,
		Stepper: NewStepper("Name", "WIP", "Confirm"),
	}

	m = sendMsg(m, forkCompleteMsg{err: errTest})
	if m.activeView != ViewFork {
		t.Errorf("expected ViewFork after fork error, got %d", m.activeView)
	}
	if m.forkState.Err == nil {
		t.Error("expected error on forkState")
	}
	if m.forkState.Forking {
		t.Error("expected Forking=false after error")
	}
}

func TestRenderFork_AllSteps(t *testing.T) {
	t.Run("name step", func(t *testing.T) {
		s := &ForkState{
			Step:    ForkStepName,
			Source:  WorktreeItem{ShortName: "feature-auth", Branch: "feature-auth"},
			Stepper: NewStepper("Name", "WIP", "Confirm"),
		}
		v := renderFork(s, 80)
		if v == "" {
			t.Fatal("expected non-empty render")
		}
		if !strings.Contains(v, "Fork Worktree") {
			t.Error("expected 'Fork Worktree' title")
		}
		if !strings.Contains(v, "feature-auth") {
			t.Error("expected source name in render")
		}
	})

	t.Run("WIP step", func(t *testing.T) {
		s := &ForkState{
			Step:     ForkStepWIP,
			Source:   WorktreeItem{ShortName: "feature-auth", Branch: "feature-auth"},
			Name:     "new-fork",
			HasWIP:   true,
			WIPFiles: []string{"file.go", "other.go"},
			Stepper:  NewStepper("Name", "WIP", "Confirm"),
		}
		s.Stepper.Current = 1
		v := renderFork(s, 80)
		if !strings.Contains(v, "2 files changed") {
			t.Error("expected WIP file count in render")
		}
	})

	t.Run("confirm step", func(t *testing.T) {
		s := &ForkState{
			Step:        ForkStepConfirm,
			Source:      WorktreeItem{ShortName: "feature-auth", Branch: "feature-auth"},
			Name:        "new-fork",
			WIPStrategy: WIPMove,
			HasWIP:      true,
			WIPFiles:    []string{"file.go"},
			Stepper:     NewStepper("Name", "WIP", "Confirm"),
		}
		s.Stepper.Current = 2
		v := renderFork(s, 80)
		if !strings.Contains(v, "Ready to fork") {
			t.Error("expected 'Ready to fork' in confirm step")
		}
		if !strings.Contains(v, "move to new worktree") {
			t.Error("expected WIP move strategy in render")
		}
	})

	t.Run("forking state", func(t *testing.T) {
		s := &ForkState{
			Step:    ForkStepConfirm,
			Source:  WorktreeItem{ShortName: "test"},
			Name:    "new-fork",
			Forking: true,
			Stepper: NewStepper("Name", "WIP", "Confirm"),
		}
		v := renderFork(s, 80)
		if !strings.Contains(v, "Forking worktree") {
			t.Error("expected forking message")
		}
	})

	t.Run("error state", func(t *testing.T) {
		s := &ForkState{
			Step:    ForkStepName,
			Source:  WorktreeItem{ShortName: "test"},
			Err:     errTest,
			Stepper: NewStepper("Name", "WIP", "Confirm"),
		}
		v := renderFork(s, 80)
		if !strings.Contains(v, "test error") {
			t.Error("expected error text in render")
		}
	})
}

func TestForkNameForm(t *testing.T) {
	var name string
	form := NewForkNameForm(&name, "test-project", nil)
	if form == nil {
		t.Fatal("expected non-nil form")
	}
}

func TestForkWIPForm(t *testing.T) {
	var choice string
	form := NewForkWIPForm(&choice)
	if form == nil {
		t.Fatal("expected non-nil form")
	}
}

func TestWIPStrategyConstants(t *testing.T) {
	if WIPMove != 0 || WIPCopy != 1 || WIPLeave != 2 {
		t.Error("unexpected WIP strategy constant values")
	}
}
