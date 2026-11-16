package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestToastHistory_KeepsWarningsAndErrors(t *testing.T) {
	tm := NewToastModel()
	tm.Show(NewToast("saved", ToastSuccess))
	tm.Show(NewToast("loading", ToastInfo))
	tm.Show(NewToast("hook failed", ToastWarning))
	tm.Show(NewToast("delete failed", ToastError))
	var got []string
	for _, t := range tm.History() {
		got = append(got, t.Message)
	}
	if strings.Join(got, ",") != "delete failed,hook failed" {
		t.Errorf("history = %v, want errors and warnings, newest first", got)
	}
}

func TestToastHistory_CappedAt50(t *testing.T) {
	tm := NewToastModel()
	for i := 0; i < 60; i++ {
		tm.Show(NewToast(fmt.Sprintf("w%d", i), ToastWarning))
	}
	h := tm.History()
	if len(h) != 50 || h[0].Message != "w59" || h[49].Message != "w10" {
		t.Errorf("len %d first %q last %q", len(h), h[0].Message, h[len(h)-1].Message)
	}
}

func TestMessages_ActionOpensOverlay(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.toast.Show(&Toast{Message: "Deleted \"x\" but branch is checked out", Level: ToastWarning, Duration: time.Hour, CreatedAt: testNow.Add(-5 * time.Minute)})
	m = sendKey(m, "space")
	m = typeOmni(m, "warnings")
	m = sendKey(m, "enter")
	if m.activeView() != ViewMessages {
		t.Fatalf("view %v, want messages", m.activeView())
	}
	out := ansi.Strip(m.viewString())
	if !strings.Contains(out, "branch is checked out") || !strings.Contains(out, "5m") {
		t.Errorf("messages overlay:\n%s", out)
	}
	m = sendKey(m, "esc")
	if m.activeView() != ViewDashboard {
		t.Errorf("esc: view %v", m.activeView())
	}
}

func TestMessages_EmptySaysSo(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.pushView(ViewMessages)
	if out := ansi.Strip(m.viewString()); !strings.Contains(out, "no warnings or errors this session") {
		t.Errorf("empty history:\n%s", out)
	}
}

// The app's model has no injected clock; ages still show.
func TestMessages_AgeWithoutInjectedClock(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.now = nil
	m.toast.Show(&Toast{Message: "hook failed", Level: ToastWarning, Duration: time.Hour, CreatedAt: time.Now().Add(-3 * time.Minute)})
	m.pushView(ViewMessages)
	if out := ansi.Strip(m.viewString()); !strings.Contains(out, "3m") {
		t.Errorf("no age without an injected clock:\n%s", out)
	}
}
