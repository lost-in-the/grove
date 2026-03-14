package tui

import (
	"fmt"
	"testing"
)

func TestReadCreationLog_Line(t *testing.T) {
	ch := make(chan creationEvent, 5)
	ch <- creationEvent{line: "step 1"}

	cmd := readCreationLog(ch, "test")
	msg := cmd()

	logMsg, ok := msg.(creationLogMsg)
	if !ok {
		t.Fatalf("expected creationLogMsg, got %T", msg)
	}
	if logMsg.line != "step 1" {
		t.Errorf("line = %q, want %q", logMsg.line, "step 1")
	}
	if logMsg.ch == nil {
		t.Error("ch should be populated for chaining")
	}
}

func TestReadCreationLog_Done(t *testing.T) {
	ch := make(chan creationEvent, 5)
	ch <- creationEvent{done: true, name: "wt", path: "/tmp/wt"}

	cmd := readCreationLog(ch, "test")
	msg := cmd()

	doneMsg, ok := msg.(creationDoneMsg)
	if !ok {
		t.Fatalf("expected creationDoneMsg, got %T", msg)
	}
	if doneMsg.name != "wt" {
		t.Errorf("name = %q, want %q", doneMsg.name, "wt")
	}
	if doneMsg.source != "test" {
		t.Errorf("source = %q, want %q", doneMsg.source, "test")
	}
}

func TestReadCreationLog_ChannelClose(t *testing.T) {
	ch := make(chan creationEvent, 5)
	close(ch)

	cmd := readCreationLog(ch, "test")
	msg := cmd()

	doneMsg, ok := msg.(creationDoneMsg)
	if !ok {
		t.Fatalf("expected creationDoneMsg, got %T", msg)
	}
	if doneMsg.name != "" {
		t.Errorf("name should be empty on channel close, got %q", doneMsg.name)
	}
	if doneMsg.source != "test" {
		t.Errorf("source = %q, want %q", doneMsg.source, "test")
	}
}

func TestCreationLogMsg_ChainsNext(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	// Set up create state with ActivityLog
	m.activeView = ViewCreate
	m.createState = &CreateState{
		Creating:    true,
		ActivityLog: NewActivityLog(60, 10),
	}

	ch := make(chan creationEvent, 5)
	msg := creationLogMsg{line: "building...", ch: ch}

	result, cmd := m.Update(msg)
	m = result.(Model)

	// Verify the log line was added
	if len(m.createState.ActivityLog.lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(m.createState.ActivityLog.lines))
	}
	if m.createState.ActivityLog.lines[0] != "building..." {
		t.Errorf("log line = %q, want %q", m.createState.ActivityLog.lines[0], "building...")
	}

	// Verify a chained read cmd was returned
	if cmd == nil {
		t.Error("expected non-nil cmd for chained read")
	}
}

func TestCreationDoneMsg_Create(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m.activeView = ViewCreate
	m.createState = &CreateState{
		Creating:    true,
		ActivityLog: NewActivityLog(60, 10),
	}

	msg := creationDoneMsg{source: "create", name: "wt"}
	result, _ := m.Update(msg)
	m = result.(Model)

	if m.activeView != ViewDashboard {
		t.Errorf("activeView should be ViewDashboard after create done, got %d", m.activeView)
	}
}

func TestCreationDoneMsg_PR(t *testing.T) {
	m := newTestModel(withPRData(), withSize(80, 24))
	m.activeView = ViewPRs
	m.prState.Creating = true
	m.prState.ActivityLog = NewActivityLog(60, 10)

	msg := creationDoneMsg{source: "pr", name: "wt"}
	result, _ := m.Update(msg)
	m = result.(Model)

	// Successful PR creation transitions to dashboard
	if m.activeView != ViewDashboard {
		t.Errorf("activeView should be ViewDashboard after PR create done, got %d", m.activeView)
	}
}

func TestCreationDoneMsg_Error(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m.activeView = ViewCreate
	m.createState = &CreateState{
		Creating:    true,
		ActivityLog: NewActivityLog(60, 10),
	}

	msg := creationDoneMsg{source: "create", err: fmt.Errorf("fail")}
	result, _ := m.Update(msg)
	m = result.(Model)

	// On error, should stay in create view with error set
	if m.createState == nil {
		t.Fatal("createState should not be nil on error")
	}
	if m.createState.Error != "fail" {
		t.Errorf("createState.Error = %q, want %q", m.createState.Error, "fail")
	}
	if m.createState.Creating {
		t.Error("Creating should be false after error")
	}
}
