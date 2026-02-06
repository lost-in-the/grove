package tui

import (
	"strings"
	"testing"
)

func TestNewSyncState(t *testing.T) {
	items := makeTestItems(3)
	s := NewSyncState(items)

	if s.Step != SyncStepSource {
		t.Errorf("expected SyncStepSource, got %d", s.Step)
	}
	if s.Stepper == nil {
		t.Fatal("expected stepper to be initialized")
	}
	if len(s.Stepper.Steps) != 3 {
		t.Errorf("expected 3 stepper steps, got %d", len(s.Stepper.Steps))
	}
	// Target should be the current worktree (item 0 is current in test data)
	if s.Target.ShortName != "main" {
		t.Errorf("expected target 'main', got %q", s.Target.ShortName)
	}
}

func TestSyncOverlay_OpenClose(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "s")
	if m.activeView != ViewSync {
		t.Errorf("expected ViewSync, got %d", m.activeView)
	}
	if m.syncState == nil {
		t.Fatal("expected syncState to be set")
	}

	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after esc, got %d", m.activeView)
	}
	if m.syncState != nil {
		t.Error("expected syncState to be nil after close")
	}
}

func TestSyncOverlay_NilState(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewSync
	m.syncState = nil
	m = sendKey(m, "enter")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard for nil state, got %d", m.activeView)
	}
}

func TestSyncOverlay_SourceNavigation(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 30))
	m = sendKey(m, "s")

	// Simulate WIP info arriving
	sources := []WorktreeWIPInfo{
		{Item: WorktreeItem{ShortName: "feature-auth"}, HasWIP: true, Files: []string{"file.go"}},
		{Item: WorktreeItem{ShortName: "fix-bug"}, HasWIP: false},
		{Item: WorktreeItem{ShortName: "testing"}, HasWIP: true, Files: []string{"test.go", "other.go"}},
	}
	m = sendMsg(m, syncWIPInfoMsg{sources: sources})

	if m.syncState.Selected != 0 {
		t.Errorf("expected cursor at 0, got %d", m.syncState.Selected)
	}

	m = sendKey(m, "j")
	if m.syncState.Selected != 1 {
		t.Errorf("expected cursor at 1, got %d", m.syncState.Selected)
	}

	m = sendKey(m, "k")
	if m.syncState.Selected != 0 {
		t.Errorf("expected cursor at 0, got %d", m.syncState.Selected)
	}

	// Can't go above 0
	m = sendKey(m, "k")
	if m.syncState.Selected != 0 {
		t.Errorf("expected cursor still at 0, got %d", m.syncState.Selected)
	}
}

func TestSyncOverlay_SelectSourceWithWIP(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 30))
	m = sendKey(m, "s")

	sources := []WorktreeWIPInfo{
		{Item: WorktreeItem{ShortName: "feature-auth"}, HasWIP: true, Files: []string{"file.go"}},
	}
	m = sendMsg(m, syncWIPInfoMsg{sources: sources})

	m = sendKey(m, "enter")
	if m.syncState.Step != SyncStepPreview {
		t.Errorf("expected SyncStepPreview, got %d", m.syncState.Step)
	}
}

func TestSyncOverlay_SelectSourceWithoutWIP(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 30))
	m = sendKey(m, "s")

	sources := []WorktreeWIPInfo{
		{Item: WorktreeItem{ShortName: "fix-bug"}, HasWIP: false},
	}
	m = sendMsg(m, syncWIPInfoMsg{sources: sources})

	m = sendKey(m, "enter")
	// Should NOT advance since source has no WIP
	if m.syncState.Step != SyncStepSource {
		t.Errorf("expected SyncStepSource (no WIP to sync), got %d", m.syncState.Step)
	}
}

func TestSyncOverlay_PreviewBackAndForward(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 30))
	m.activeView = ViewSync
	m.syncState = &SyncState{
		Step:    SyncStepPreview,
		Target:  WorktreeItem{ShortName: "main"},
		Sources: []WorktreeWIPInfo{
			{Item: WorktreeItem{ShortName: "feature-auth"}, HasWIP: true, Files: []string{"file.go"}},
		},
		Stepper: NewStepper("Source", "Preview", "Confirm"),
	}
	m.syncState.Stepper.Current = 1

	// Back should go to source
	m = sendKey(m, "backspace")
	if m.syncState.Step != SyncStepSource {
		t.Errorf("expected SyncStepSource, got %d", m.syncState.Step)
	}
}

func TestSyncOverlay_ConfirmBack(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 30))
	m.activeView = ViewSync
	m.syncState = &SyncState{
		Step:    SyncStepConfirm,
		Target:  WorktreeItem{ShortName: "main"},
		Sources: []WorktreeWIPInfo{
			{Item: WorktreeItem{ShortName: "feature-auth"}, HasWIP: true, Files: []string{"file.go"}},
		},
		Stepper: NewStepper("Source", "Preview", "Confirm"),
	}
	m.syncState.Stepper.Current = 2

	m = sendKey(m, "backspace")
	if m.syncState.Step != SyncStepPreview {
		t.Errorf("expected SyncStepPreview, got %d", m.syncState.Step)
	}
}

func TestSyncOverlay_SyncingIgnoresInput(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 30))
	m.activeView = ViewSync
	m.syncState = &SyncState{
		Step:    SyncStepConfirm,
		Syncing: true,
		Stepper: NewStepper("Source", "Preview", "Confirm"),
	}

	m = sendKey(m, "esc")
	if m.activeView != ViewSync {
		t.Errorf("expected ViewSync while syncing, got %d", m.activeView)
	}
}

func TestSyncCompleteMsg_Success(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewSync
	m.syncState = &SyncState{
		Step:    SyncStepConfirm,
		Syncing: true,
		Stepper: NewStepper("Source", "Preview", "Confirm"),
	}

	m = sendMsg(m, syncCompleteMsg{filesApplied: 3})
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after sync, got %d", m.activeView)
	}
	if m.syncState != nil {
		t.Error("expected syncState nil after success")
	}
}

func TestSyncCompleteMsg_Error(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewSync
	m.syncState = &SyncState{
		Step:    SyncStepConfirm,
		Syncing: true,
		Stepper: NewStepper("Source", "Preview", "Confirm"),
	}

	m = sendMsg(m, syncCompleteMsg{err: errTest})
	if m.activeView != ViewSync {
		t.Errorf("expected ViewSync after error, got %d", m.activeView)
	}
	if m.syncState.Err == nil {
		t.Error("expected error on syncState")
	}
	if m.syncState.Syncing {
		t.Error("expected Syncing=false after error")
	}
}

func TestSyncWIPInfoMsg(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewSync
	m.syncState = &SyncState{
		Step:    SyncStepSource,
		Stepper: NewStepper("Source", "Preview", "Confirm"),
	}

	sources := []WorktreeWIPInfo{
		{Item: WorktreeItem{ShortName: "test"}, HasWIP: true, Files: []string{"file.go"}},
	}
	m = sendMsg(m, syncWIPInfoMsg{sources: sources})
	if len(m.syncState.Sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(m.syncState.Sources))
	}
}

func TestRenderSync_AllSteps(t *testing.T) {
	t.Run("source step empty", func(t *testing.T) {
		s := &SyncState{
			Step:    SyncStepSource,
			Target:  WorktreeItem{ShortName: "main"},
			Stepper: NewStepper("Source", "Preview", "Confirm"),
		}
		v := renderSync(s, 80)
		if v == "" {
			t.Fatal("expected non-empty render")
		}
		if !strings.Contains(v, "Sync Changes") {
			t.Error("expected 'Sync Changes' title")
		}
	})

	t.Run("source step with sources", func(t *testing.T) {
		s := &SyncState{
			Step:   SyncStepSource,
			Target: WorktreeItem{ShortName: "main"},
			Sources: []WorktreeWIPInfo{
				{Item: WorktreeItem{ShortName: "feature-auth"}, HasWIP: true, Files: []string{"file.go"}},
				{Item: WorktreeItem{ShortName: "fix-bug"}, HasWIP: false},
			},
			Stepper: NewStepper("Source", "Preview", "Confirm"),
		}
		v := renderSync(s, 80)
		if !strings.Contains(v, "feature-auth") {
			t.Error("expected 'feature-auth' in render")
		}
		if !strings.Contains(v, "clean") {
			t.Error("expected 'clean' status for fix-bug")
		}
	})

	t.Run("preview step", func(t *testing.T) {
		s := &SyncState{
			Step:   SyncStepPreview,
			Target: WorktreeItem{ShortName: "main"},
			Sources: []WorktreeWIPInfo{
				{Item: WorktreeItem{ShortName: "feature-auth"}, HasWIP: true, Files: []string{"M file.go", "A new.go"}},
			},
			Stepper: NewStepper("Source", "Preview", "Confirm"),
		}
		s.Stepper.Current = 1
		v := renderSync(s, 80)
		if !strings.Contains(v, "Modified") {
			t.Error("expected 'Modified' section in preview")
		}
	})

	t.Run("confirm step", func(t *testing.T) {
		s := &SyncState{
			Step:   SyncStepConfirm,
			Target: WorktreeItem{ShortName: "main"},
			Sources: []WorktreeWIPInfo{
				{Item: WorktreeItem{ShortName: "feature-auth"}, HasWIP: true, Files: []string{"file.go"}},
			},
			Stepper: NewStepper("Source", "Preview", "Confirm"),
		}
		s.Stepper.Current = 2
		v := renderSync(s, 80)
		if !strings.Contains(v, "Ready to sync") {
			t.Error("expected 'Ready to sync' in confirm")
		}
	})

	t.Run("syncing state", func(t *testing.T) {
		s := &SyncState{
			Step:    SyncStepConfirm,
			Syncing: true,
			Target:  WorktreeItem{ShortName: "main"},
			Stepper: NewStepper("Source", "Preview", "Confirm"),
		}
		v := renderSync(s, 80)
		if !strings.Contains(v, "Syncing") {
			t.Error("expected syncing message")
		}
	})

	t.Run("error state", func(t *testing.T) {
		s := &SyncState{
			Step:    SyncStepSource,
			Target:  WorktreeItem{ShortName: "main"},
			Err:     errTest,
			Stepper: NewStepper("Source", "Preview", "Confirm"),
		}
		v := renderSync(s, 80)
		if !strings.Contains(v, "test error") {
			t.Error("expected error text")
		}
	})
}

func TestSyncOverlay_SelectSourceWithError(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 30))
	m = sendKey(m, "s")

	sources := []WorktreeWIPInfo{
		{Item: WorktreeItem{ShortName: "broken"}, CheckErr: errTest},
	}
	m = sendMsg(m, syncWIPInfoMsg{sources: sources})

	m = sendKey(m, "enter")
	if m.syncState.Step != SyncStepSource {
		t.Errorf("expected SyncStepSource for errored source, got %d", m.syncState.Step)
	}
	if m.syncState.Err == nil {
		t.Error("expected error feedback for errored source")
	}
}

func TestSyncOverlay_SelectCleanShowsFeedback(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 30))
	m = sendKey(m, "s")

	sources := []WorktreeWIPInfo{
		{Item: WorktreeItem{ShortName: "clean-wt"}, HasWIP: false},
	}
	m = sendMsg(m, syncWIPInfoMsg{sources: sources})

	m = sendKey(m, "enter")
	if m.syncState.Step != SyncStepSource {
		t.Errorf("expected SyncStepSource, got %d", m.syncState.Step)
	}
	if m.syncState.Err == nil {
		t.Error("expected error feedback for clean source")
	}
}

func TestRenderSync_SourceWithCheckError(t *testing.T) {
	s := &SyncState{
		Step:   SyncStepSource,
		Target: WorktreeItem{ShortName: "main"},
		Sources: []WorktreeWIPInfo{
			{Item: WorktreeItem{ShortName: "broken"}, CheckErr: errTest},
		},
		Stepper: NewStepper("Source", "Preview", "Confirm"),
	}
	v := renderSync(s, 80)
	if !strings.Contains(v, "error") {
		t.Error("expected 'error' status for broken source")
	}
}

func TestSelectedSource(t *testing.T) {
	s := &SyncState{
		Sources: []WorktreeWIPInfo{
			{Item: WorktreeItem{ShortName: "a"}},
			{Item: WorktreeItem{ShortName: "b"}},
		},
		Selected: 1,
	}
	src := s.selectedSource()
	if src == nil {
		t.Fatal("expected non-nil source")
	}
	if src.Item.ShortName != "b" {
		t.Errorf("expected 'b', got %q", src.Item.ShortName)
	}

	// Out of range
	s.Selected = 10
	if s.selectedSource() != nil {
		t.Error("expected nil for out-of-range selected")
	}
}
