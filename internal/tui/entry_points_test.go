package tui

import (
	"testing"
)

func TestNewModelForPRs(t *testing.T) {
	m := newTestModel(withItems(3))
	m = m.ConfigureForPRs()

	if m.activeView != ViewPRs {
		t.Errorf("expected activeView=ViewPRs, got %d", m.activeView)
	}
	if m.prState == nil {
		t.Fatal("expected prState to be initialized")
	}
	if !m.prState.Loading {
		t.Error("expected prState.Loading to be true")
	}
}

func TestNewModelForIssues(t *testing.T) {
	m := newTestModel(withItems(3))
	m = m.ConfigureForIssues()

	if m.activeView != ViewIssues {
		t.Errorf("expected activeView=ViewIssues, got %d", m.activeView)
	}
	if m.issueState == nil {
		t.Fatal("expected issueState to be initialized")
	}
	if !m.issueState.Loading {
		t.Error("expected issueState.Loading to be true")
	}
}

func TestConfigureForPRs_WorktreeBranches(t *testing.T) {
	m := newTestModel(withItems(3))
	m = m.ConfigureForPRs()

	if m.prState.WorktreeBranches == nil {
		t.Error("expected WorktreeBranches to be initialized")
	}
}
