package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestActivityLog_NewMinLines(t *testing.T) {
	log := NewActivityLog(60, 1)
	// maxLines should be clamped to 3
	if log.maxLines != 3 {
		t.Errorf("maxLines = %d, want 3 (clamped from 1)", log.maxLines)
	}
}

func TestActivityLog_EmptyView(t *testing.T) {
	log := NewActivityLog(60, 5)
	out := log.View("⠋")
	if !strings.Contains(out, "Starting...") {
		t.Errorf("empty log should contain 'Starting...', got %q", out)
	}
}

func TestActivityLog_AddAndView(t *testing.T) {
	log := NewActivityLog(60, 5)
	log.AddLine("step 1")
	log.AddLine("step 2")
	out := ansi.Strip(log.View("⠋"))
	if !strings.Contains(out, "step 1") {
		t.Errorf("output should contain 'step 1', got %q", out)
	}
	if !strings.Contains(out, "step 2") {
		t.Errorf("output should contain 'step 2', got %q", out)
	}
}

func TestActivityLog_TailBehavior(t *testing.T) {
	log := NewActivityLog(60, 5)
	for i := 0; i < 10; i++ {
		log.AddLine(fmt.Sprintf("line-%d", i))
	}
	out := ansi.Strip(log.View("⠋"))
	// First 5 lines should be gone (tail shows last 5)
	for i := 0; i < 5; i++ {
		needle := fmt.Sprintf("line-%d", i)
		if strings.Contains(out, needle) {
			t.Errorf("output should NOT contain %q (tail should have dropped it), got %q", needle, out)
		}
	}
	// Last 5 lines should be present
	for i := 5; i < 10; i++ {
		needle := fmt.Sprintf("line-%d", i)
		if !strings.Contains(out, needle) {
			t.Errorf("output should contain %q, got %q", needle, out)
		}
	}
}

func TestActivityLog_SetDoneSuccess(t *testing.T) {
	log := NewActivityLog(60, 5)
	log.AddLine("working...")
	log.SetDone(nil)

	out := ansi.Strip(log.View("⠋"))
	if !strings.Contains(out, "Done") {
		t.Errorf("done log should contain 'Done', got %q", out)
	}
	if !log.IsDone() {
		t.Error("IsDone() should return true after SetDone(nil)")
	}
}

func TestActivityLog_SetDoneError(t *testing.T) {
	log := NewActivityLog(60, 5)
	log.AddLine("working...")
	log.SetDone(fmt.Errorf("boom"))

	out := ansi.Strip(log.View("⠋"))
	if !strings.Contains(out, "Failed") {
		t.Errorf("error log should contain 'Failed', got %q", out)
	}
	if !strings.Contains(out, "boom") {
		t.Errorf("error log should contain 'boom', got %q", out)
	}
}

func TestActivityLog_DoneAllLinesDim(t *testing.T) {
	log := NewActivityLog(60, 5)
	log.AddLine("step 1")
	log.SetDone(nil)

	out := log.View("⠋")
	// After done, the spinner prefix should NOT appear (all lines are dim bullets)
	if strings.Contains(out, "⠋") {
		t.Errorf("after SetDone, spinner prefix should not appear in output, got %q", out)
	}
}

func TestRenderCreatingDetail_WithLog(t *testing.T) {
	log := NewActivityLog(60, 5)
	log.AddLine("fetching PR...")
	out := ansi.Strip(renderCreatingDetail(log, "⠋", "fallback"))
	if !strings.Contains(out, "fetching PR...") {
		t.Errorf("renderCreatingDetail with log should contain log line, got %q", out)
	}
}

func TestRenderCreatingDetail_NilLog(t *testing.T) {
	out := renderCreatingDetail(nil, "⠋", "fallback")
	if !strings.Contains(out, "fallback") {
		t.Errorf("renderCreatingDetail with nil log should contain 'fallback', got %q", out)
	}
	if !strings.Contains(out, "⠋") {
		t.Errorf("renderCreatingDetail with nil log should contain spinner, got %q", out)
	}
}
