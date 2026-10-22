package tui

import (
	"os/exec"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/plugins/tracker"
)

func TestEditorArgs(t *testing.T) {
	cases := []struct {
		editor, want string
	}{
		{"nvim", "nvim +12 /w/a b.go"},
		{"vi", "vi +12 /w/a b.go"},
		{"/usr/local/bin/hx", "/usr/local/bin/hx +12 /w/a b.go"},
		{"code -w", "code -w -g /w/a b.go:12"},
		{"cursor", "cursor -g /w/a b.go:12"},
		{"subl", "subl /w/a b.go:12"},
		{"zed", "zed /w/a b.go:12"},
		{"goland", "goland --line 12 /w/a b.go"},
		{"", "vi +12 /w/a b.go"},
	}
	for _, c := range cases {
		name, args := editorArgs(c.editor, "/w/a b.go", 12)
		if got := strings.Join(append([]string{name}, args...), " "); got != c.want {
			t.Errorf("%q: %s, want %s", c.editor, got, c.want)
		}
	}
	// A path with a space stays one argument.
	if _, args := editorArgs("nvim", "/w/a b.go", 1); args[len(args)-1] != "/w/a b.go" {
		t.Errorf("path split: %q", args)
	}
}

// The line is the new-side number of the first line in view that has one.
func TestEditorLine(t *testing.T) {
	f := jumpFile() // header row 0, context 1..40 at rows 1..40, removed row 41, added row 42
	for _, c := range []struct{ scroll, want int }{{0, 1}, {1, 1}, {10, 10}, {41, 41}, {42, 41}, {99, 1}} {
		if got := editorLine(f, c.scroll, false); got != c.want {
			t.Errorf("scroll %d: line %d, want %d", c.scroll, got, c.want)
		}
	}
}

// editorReview opens the files tab of a PR on fix-bug, a branch the test
// model has a worktree for.
func editorReview(t *testing.T, branch string, files []git.FileDiff) Model {
	t.Helper()
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.openPRReview(&tracker.PullRequest{Number: 42, Branch: branch})
	m = next.(Model)
	m = sendMsg(m, prReviewRepoMsg{gen: m.prReview.Gen, repo: "o/r"})
	return sendKey(sendMsg(m, prDiffMsg{gen: m.prReview.Gen, files: files}), "4")
}

func captureEditor(t *testing.T) *[]string {
	t.Helper()
	var got []string
	orig := execEditor
	execEditor = func(c *exec.Cmd) tea.Cmd {
		got = c.Args
		return func() tea.Msg { return editorClosedMsg{} }
	}
	t.Cleanup(func() { execEditor = orig })
	return &got
}

func TestPRReview_EnterOpensEditor(t *testing.T) {
	t.Setenv("VISUAL", "nvim")
	got := captureEditor(t)
	m := editorReview(t, "fix-bug", []git.FileDiff{jumpFile()})
	before := m.viewString()
	next, cmd := m.handlePRReviewKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter returned no command")
	}
	if want := "nvim +1 /tmp/test-project-fix-bug/main.go"; strings.Join(*got, " ") != want {
		t.Errorf("editor %q, want %q", *got, want)
	}
	m = sendMsg(next.(Model), cmd())
	if m.viewString() != before {
		t.Error("the frame changed across the editor round trip")
	}
}

func TestPRReview_EditorWithoutWorktree(t *testing.T) {
	got := captureEditor(t)
	m := editorReview(t, "nobody-has-this", []git.FileDiff{jumpFile()})
	next, cmd := m.handlePRReviewKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	if cmd != nil || *got != nil || !strings.Contains(m.toast.Message(), "no worktree on nobody-has-this") {
		t.Errorf("cmd %v, editor %q, toast %q", cmd != nil, *got, m.toast.Message())
	}
}

func TestPRReview_EditorDeletedFile(t *testing.T) {
	got := captureEditor(t)
	f := diffFile("gone.go", 0, 2)
	f.Status, f.NewPath = git.FileDeleted, ""
	m := editorReview(t, "fix-bug", []git.FileDiff{f})
	next, cmd := m.handlePRReviewKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	if cmd != nil || *got != nil || !strings.Contains(m.toast.Message(), "deleted") {
		t.Errorf("cmd %v, editor %q, toast %q", cmd != nil, *got, m.toast.Message())
	}
}

func TestPRReview_EditorClosedWithError(t *testing.T) {
	m := editorReview(t, "fix-bug", []git.FileDiff{jumpFile()})
	m = sendMsg(m, editorClosedMsg{err: errTest})
	if m.activeView() != ViewPRReview || !strings.Contains(m.toast.Message(), "editor: test error") {
		t.Errorf("view %v toast %q", m.activeView(), m.toast.Message())
	}
}
