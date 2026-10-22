package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/lost-in-the/grove/internal/git"
)

// A branch with one committed change and one uncommitted edit, in a repo
// whose config would change git diff's prefixes if grove did not set them.
func localFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null") // no hooks or diff settings from the user
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	for _, kv := range [][2]string{{"user.name", "t"}, {"user.email", "t@t"}, {"commit.gpgsign", "false"},
		{"diff.mnemonicPrefix", "true"}, {"diff.noprefix", "true"}} {
		run("config", kv[0], kv[1])
	}
	write("a.txt", "one\n")
	write("b.txt", "two\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	run("checkout", "-q", "-b", "feature")
	write("a.txt", "one\nmore\n")
	run("commit", "-q", "-am", "change a")
	write("b.txt", "two\nedited\n") // uncommitted
	return dir
}

func TestLocalDiff_AgainstMergeBase(t *testing.T) {
	dir := localFixture(t)
	msg := localDiffCmd(7, dir, "main")().(prDiffMsg)
	if msg.err != nil || msg.gen != 7 {
		t.Fatalf("gen %d err %v", msg.gen, msg.err)
	}
	var paths []string
	for _, f := range msg.files {
		paths = append(paths, f.Path())
	}
	sort.Strings(paths)
	if strings.Join(paths, ",") != "a.txt,b.txt" {
		t.Errorf("paths %v, want the committed and the uncommitted change with plain names", paths)
	}
}

func TestGStrip_DiffOpensLocalReview(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(sendKey(sendKey(m, "j"), "g"), "d")
	if m.activeView() != ViewPRReview || m.prReview.Local == nil || m.prReview.Local.Path != "/tmp/test-project-feature-auth" {
		t.Fatalf("view %v review %+v", m.activeView(), m.prReview)
	}
}

func TestDetailFocused_DOpensLocalReview(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(sendKey(m, "j"), "tab") // focus the docked detail pane
	if !m.detailFocused {
		t.Fatal("tab did not focus the detail pane")
	}
	if m = sendKey(m, "d"); m.activeView() != ViewPRReview || m.prReview.Local == nil {
		t.Errorf("d in the focused detail pane: view %v", m.activeView())
	}
}

func TestDetailSheet_DOpensLocalReview(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(sendKey(m, "j"), "tab") // the sheet below 110 columns
	if m.activeView() != ViewDetailSheet {
		t.Fatalf("tab at 80 columns: view %v", m.activeView())
	}
	if m = sendKey(m, "d"); m.activeView() != ViewPRReview || m.prReview.Local == nil {
		t.Errorf("d in the detail sheet: view %v", m.activeView())
	}
}

func TestDashboard_DStillDeletes(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	if m = sendKey(sendKey(m, "j"), "d"); m.activeView() != ViewDelete {
		t.Errorf("d with the list focused: view %v, want delete", m.activeView())
	}
}

func localReview(t *testing.T, files []git.FileDiff) Model {
	t.Helper()
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(sendKey(sendKey(m, "j"), "g"), "d")
	return sendMsg(m, prDiffMsg{gen: m.prReview.Gen, files: files})
}

func TestLocalReview_OnlyFilesTab(t *testing.T) {
	m := localReview(t, railFiles)
	lines := strings.Split(stripAnsi(m.viewString()), "\n")
	if !strings.Contains(lines[0], "feature-auth vs main") {
		t.Errorf("header %q", lines[0])
	}
	if !strings.Contains(lines[1], "[4] files 4") || strings.Contains(lines[1], "conversation") {
		t.Errorf("tab row %q", lines[1])
	}
	for _, k := range []string{"1", "2", "3"} {
		if m = sendKey(m, k); m.prReview.Tab != 4 {
			t.Errorf("%s moved a local review to tab %d", k, m.prReview.Tab)
		}
	}
	if m = sendKey(m, "v"); !strings.Contains(m.toast.Message(), "pull requests") {
		t.Errorf("v: toast %q", m.toast.Message())
	}
}

func TestLocalReview_EveryKeySafe(t *testing.T) {
	m := localReview(t, railFiles)
	captureEditor(t)
	for _, k := range []string{"1", "2", "3", "4", "B", "v", "a", "enter", "J", "K", "t", "z", "j", "G", "g"} {
		m = sendKey(m, k)
	}
	if m.activeView() != ViewPRReview {
		t.Errorf("view %v", m.activeView())
	}
}

func TestLocalReview_NotCached(t *testing.T) {
	m := localReview(t, railFiles)
	if len(m.prReviewCache) != 0 {
		t.Errorf("a local review was cached: %v", m.prReviewCache)
	}
}

func TestLocalReview_EmptyDiff(t *testing.T) {
	if text := stripAnsi(localReview(t, nil).viewString()); !strings.Contains(text, "no files changed") {
		t.Errorf("empty local diff:\n%s", text)
	}
}

func TestGolden_LocalReview(t *testing.T) {
	m := goldenModel(t, sizeStandard, withItems(3))
	m = sendKey(sendKey(sendKey(m, "j"), "g"), "d")
	m = sendMsg(m, prDiffMsg{gen: m.prReview.Gen, files: railFiles[:2]})
	golden.RequireEqual(t, []byte(m.viewString()))
}

// A checkout with no local default branch (a fresh clone of a feature
// branch) diffs against origin's.
func TestLocalDiff_FallsBackToOrigin(t *testing.T) {
	dir := localFixture(t)
	for _, args := range [][]string{
		{"update-ref", "refs/remotes/origin/main", "main"},
		{"branch", "-D", "main"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	msg := localDiffCmd(1, dir, "main")().(prDiffMsg)
	if msg.err != nil || len(msg.files) != 2 {
		t.Errorf("err %v, %d files", msg.err, len(msg.files))
	}
}

// A local review's editor opens in that worktree, whatever its branch.
func TestLocalReview_EnterOpensItsOwnWorktree(t *testing.T) {
	got := captureEditor(t)
	m := localReview(t, []git.FileDiff{jumpFile()})
	m.prReview.PR.Branch = "fix-bug" // another worktree's branch, as two detached ones share one
	next, _ := m.handlePRReviewKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	_ = next
	if want := "/tmp/test-project-feature-auth/main.go"; len(*got) == 0 || (*got)[len(*got)-1] != want {
		t.Errorf("editor %q, want the file in %s", *got, want)
	}
}

// A default branch that reads as a git option is refused, not passed on.
func TestLocalDiff_RejectsOptionLikeBase(t *testing.T) {
	dir := localFixture(t)
	if msg := localDiffCmd(1, dir, "--independent")().(prDiffMsg); msg.err == nil {
		t.Errorf("a base of --independent diffed %d files", len(msg.files))
	}
}

// The local review's footer offers only keys it has.
func TestLocalReview_FooterHasNoPRKeys(t *testing.T) {
	m := localReview(t, railFiles)
	footer := stripAnsi(m.prReviewFooter())
	if strings.Contains(footer, "viewed") || strings.Contains(footer, "1-4") || !strings.Contains(footer, "editor") {
		t.Errorf("footer %q", footer)
	}
}
