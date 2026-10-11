package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
)

func TestViewStack_EmptyTopIsDashboard(t *testing.T) {
	if got := (viewStack{}).top(); got != ViewDashboard {
		t.Errorf("top of empty stack = %v, want ViewDashboard", got)
	}
}

func TestViewStack_PushPopOrder(t *testing.T) {
	s := viewStack{}.push(viewFrame{view: ViewPRs}).push(viewFrame{view: ViewCreate})
	if s.top() != ViewCreate {
		t.Fatalf("top = %v, want ViewCreate", s.top())
	}
	s, f, ok := s.pop()
	if !ok || f.view != ViewCreate || s.top() != ViewPRs {
		t.Errorf("pop = (%v, %v), top now %v; want (ViewCreate, true), top ViewPRs", f.view, ok, s.top())
	}
}

func TestViewStack_PopAtRootIsNoOp(t *testing.T) {
	s, _, ok := viewStack{}.pop()
	if ok || len(s) != 0 {
		t.Errorf("pop at root: ok=%v len=%d, want false and 0", ok, len(s))
	}
}

func TestViewStack_ToRootReturnsBottomFrame(t *testing.T) {
	s := viewStack{}.
		push(viewFrame{view: ViewPRs, returnPath: "/a"}).
		push(viewFrame{view: ViewCreate, returnPath: "/b"})
	s, f, ok := s.toRoot()
	if !ok || len(s) != 0 || f.returnPath != "/a" {
		t.Errorf("toRoot: ok=%v len=%d path=%q; want true, 0, /a", ok, len(s), f.returnPath)
	}
}

func TestViewStack_CopiesDoNotShareFrames(t *testing.T) {
	base := viewStack{}.push(viewFrame{view: ViewPRs}).push(viewFrame{view: ViewCreate})
	popped, _, _ := base.pop()
	branched := popped.push(viewFrame{view: ViewFork})
	if base.top() != ViewCreate {
		t.Errorf("original copy changed: top = %v, want ViewCreate", base.top())
	}
	if branched.top() != ViewFork {
		t.Errorf("branched top = %v, want ViewFork", branched.top())
	}
}

func TestRestoreCursor_FindsWorktreeByPath(t *testing.T) {
	m := newTestModel(withItems(5), withSize(120, 40))
	m.restoreCursor(viewFrame{returnPath: "/tmp/test-project-testing", returnIndex: 0})
	if item, _ := m.selectedItem(); item.ShortName != "testing" {
		t.Errorf("selected %q, want testing", item.ShortName)
	}
}

func TestRestoreCursor_FallsBackToClampedIndex(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.restoreCursor(viewFrame{returnPath: "/tmp/gone", returnIndex: 9})
	if got := m.list.Index(); got != 2 {
		t.Errorf("index = %d, want 2 (last row)", got)
	}
}

func TestRestoreCursor_EmptyListIsNoOp(t *testing.T) {
	m := newTestModel(withItems(0), withSize(120, 40))
	m.restoreCursor(viewFrame{returnPath: "/tmp/x", returnIndex: 3})
	if got := m.list.Index(); got != 0 {
		t.Errorf("index = %d, want 0", got)
	}
}

func TestRestoreCursor_UsesVisibleRowsUnderFilter(t *testing.T) {
	m := newTestModel(withItems(5), withSize(120, 40))
	m.list.SetFilterText("t")
	m.list.SetFilterState(list.FilterApplied)
	if n := len(m.list.VisibleItems()); n >= 5 {
		t.Fatalf("filter did not hide any rows (%d visible); the test would not tell visible from all rows", n)
	}
	var want string
	for _, li := range m.list.VisibleItems() {
		if it := li.(WorktreeItem); it.ShortName == "testing" {
			want = it.Path
		}
	}
	if want == "" {
		t.Fatal("filter 't' should keep testing visible")
	}
	m.restoreCursor(viewFrame{returnPath: want})
	if item, _ := m.selectedItem(); item.Path != want {
		t.Errorf("selected %q, want %q", item.Path, want)
	}
}

func TestViewStack_LayoutViewSkipsHelpAndUpdate(t *testing.T) {
	s := viewStack{}.push(viewFrame{view: ViewPRs}).push(viewFrame{view: ViewHelp})
	if got := s.layoutView(); got != ViewPRs {
		t.Errorf("layoutView with help on PRs = %v, want ViewPRs", got)
	}
	s = viewStack{}.push(viewFrame{view: ViewUpdate})
	if got := s.layoutView(); got != ViewDashboard {
		t.Errorf("layoutView with update on root = %v, want ViewDashboard", got)
	}
}
