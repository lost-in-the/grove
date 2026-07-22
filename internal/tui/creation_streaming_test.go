package tui

import (
	"fmt"
	"os"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
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
	if logMsg.source != "test" {
		t.Errorf("source = %q, want %q", logMsg.source, "test")
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
	msg := creationLogMsg{source: "create", line: "building...", ch: ch}

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

// collectCreationEvents drains ch (which fn must close by returning) into a
// slice of log lines.
func collectDockerAutoUpLines(t *testing.T, cfg *config.Config, name, path string) []string {
	t.Helper()
	ch := make(chan creationEvent, 20)
	done := make(chan struct{})
	var lines []string
	go func() {
		defer close(done)
		for ev := range ch {
			lines = append(lines, ev.line)
		}
	}()
	runDockerAutoUp(ch, cfg, name, path)
	close(ch)
	<-done
	return lines
}

func TestRunDockerAutoUp_OffByDefault(t *testing.T) {
	called := false
	restore := stubDockerAutoUp(t, func(cfg *config.Config, wtPath string) (bool, error) {
		called = true
		return true, nil
	})
	defer restore()

	lines := collectDockerAutoUpLines(t, &config.Config{}, "wt", "/tmp/wt")
	if called {
		t.Error("docker.AutoUp should not run when auto_up is unset")
	}
	if len(lines) != 0 {
		t.Errorf("expected no log lines, got %v", lines)
	}
}

func TestRunDockerAutoUp_StartsWhenOptedIn(t *testing.T) {
	restore := stubDockerAutoUp(t, func(cfg *config.Config, wtPath string) (bool, error) {
		// Simulate compose streaming to stderr — must be captured, not leaked.
		fmt.Fprintln(os.Stderr, "container app-1 started")
		return true, nil
	})
	defer restore()

	autoUp := true
	cfg := &config.Config{}
	cfg.Plugins.Docker.AutoUp = &autoUp

	lines := collectDockerAutoUpLines(t, cfg, "wt", "/tmp/wt")
	want := []string{"Starting Docker stack...", "container app-1 started", "Docker stack started"}
	if len(lines) != len(want) {
		t.Fatalf("lines = %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("lines[%d] = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestRunDockerAutoUp_SurfacesFailure(t *testing.T) {
	restore := stubDockerAutoUp(t, func(cfg *config.Config, wtPath string) (bool, error) {
		return false, fmt.Errorf("compose exploded")
	})
	defer restore()

	autoUp := true
	cfg := &config.Config{}
	cfg.Plugins.Docker.AutoUp = &autoUp

	lines := collectDockerAutoUpLines(t, cfg, "wt", "/tmp/wt")
	if len(lines) != 2 {
		t.Fatalf("expected start + failure lines, got %v", lines)
	}
	if lines[1] != "Docker auto-start failed: compose exploded" {
		t.Errorf("failure line = %q", lines[1])
	}
}

// stubDockerAutoUp swaps the package seam for docker.AutoUp and returns a
// restore func.
func stubDockerAutoUp(t *testing.T, fn func(*config.Config, string) (bool, error)) func() {
	t.Helper()
	orig := dockerAutoUp
	dockerAutoUp = fn
	return func() { dockerAutoUp = orig }
}
