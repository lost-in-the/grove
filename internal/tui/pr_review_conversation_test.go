package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"

	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/plugins/tracker"
)

var convNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func ago(d time.Duration) time.Time { return convNow.Add(-d) }

var convDetail = &tracker.PRDetail{
	Number:   42,
	Comments: []tracker.PRComment{{Author: "octocat", Body: "Looks close.", CreatedAt: ago(48 * time.Hour)}},
	Reviews:  []tracker.PRReview{{Author: "hubot", State: "APPROVED", SubmittedAt: ago(time.Hour)}},
	Checks:   []tracker.PRCheck{{Name: "a", State: "SUCCESS"}, {Name: "b", State: "FAILURE"}, {Name: "c", State: "QUEUED"}},
}

var convThreads = []tracker.ReviewComment{
	{ID: 1, Path: "main.go", Line: 12, Side: "RIGHT", Author: "hubot", Body: "Why this?", CreatedAt: ago(24 * time.Hour), DiffHunk: "@@ -10,3 +10,4 @@ func main() {\n a"},
	{ID: 2, InReplyTo: 1, Path: "main.go", Line: 12, Side: "RIGHT", Author: "octocat", Body: "To keep it short.", CreatedAt: ago(23 * time.Hour)},
	{ID: 3, Path: "docs/README.md", OriginalLine: 4, Side: "RIGHT", Author: "hubot", Body: "Outdated now.", CreatedAt: ago(30 * time.Hour)},
}

func itemText(it convItem) string { return stripAnsi(strings.Join(it.lines, "\n")) }

// Items run oldest first after the check summary; a reply sits under its
// thread's root; an outdated thread names its original line.
func TestBuildConversation_OrderAndThreads(t *testing.T) {
	items := buildConversation(convDetail, convThreads, 80, convNow)
	if len(items) != 5 {
		t.Fatalf("got %d items, want summary + comment + 2 threads + review", len(items))
	}
	if s := itemText(items[0]); !strings.Contains(s, "checks: 1 passed · 1 failed · 1 running") {
		t.Errorf("summary %q", s)
	}
	want := []string{"@octocat · 2d", "docs/README.md (outdated, was line 4)", "main.go:12", "@hubot approved · 60m"}
	for i, w := range want {
		if s := itemText(items[i+1]); !strings.Contains(s, w) {
			t.Errorf("item %d %q lacks %q", i+1, s, w)
		}
	}
	thread := itemText(items[3])
	if strings.Index(thread, "Why this?") > strings.Index(thread, "To keep it short.") || !strings.Contains(thread, "@octocat") {
		t.Errorf("thread order:\n%s", thread)
	}
}

func TestPRReview_ConversationNoRawEscape(t *testing.T) {
	m := openReview(t)
	g := m.prReview.Gen
	m = sendMsg(m, prDetailMsg{gen: g, detail: &tracker.PRDetail{Number: 42,
		Comments: []tracker.PRComment{{Author: hostile, Body: "hi " + hostile}}}})
	m = sendMsg(m, prCommentsMsg{gen: g, comments: []tracker.ReviewComment{{ID: 9, Path: hostile, Line: 1, Author: "x", Body: hostile}}})
	assertNoRawEscape(t, "conversation", sendKey(m, "1").viewString())
}

func TestPRReview_ConversationEmpty(t *testing.T) {
	m := openReview(t)
	m = sendMsg(m, prDetailMsg{gen: m.prReview.Gen, detail: &tracker.PRDetail{Number: 42}})
	m = sendMsg(m, prCommentsMsg{gen: m.prReview.Gen})
	if text := stripAnsi(sendKey(m, "1").viewString()); !strings.Contains(text, "no comments yet") {
		t.Errorf("empty conversation:\n%s", text)
	}
}

// A file whose line 12 is far down, so jumping must scroll to it.
func jumpFile() git.FileDiff {
	var lines []git.DiffLine
	for i := 1; i <= 40; i++ {
		lines = append(lines, git.DiffLine{Kind: git.LineContext, Text: "ctx", OldNo: i, NewNo: i})
	}
	lines = append(lines, git.DiffLine{Kind: git.LineRemoved, Text: "old line 41", OldNo: 41})
	lines = append(lines, git.DiffLine{Kind: git.LineAdded, Text: "new line 41", NewNo: 41})
	return git.FileDiff{OldPath: "main.go", NewPath: "main.go", Hunks: []git.Hunk{{Header: "@@ -1,41 +1,41 @@ func main() {", OldStart: 1, OldLines: 41, NewStart: 1, NewLines: 41, Lines: lines}}}
}

func conversationReview(t *testing.T, threads []tracker.ReviewComment) Model {
	t.Helper()
	m := openReview(t)
	g := m.prReview.Gen
	m = sendMsg(m, prDetailMsg{gen: g, detail: &tracker.PRDetail{Number: 42}})
	m = sendMsg(m, prDiffMsg{gen: g, files: []git.FileDiff{diffFile("a.go", 1, 0), jumpFile()}})
	m = sendMsg(m, prCommentsMsg{gen: g, comments: threads})
	return sendKey(m, "1")
}

// enter on a thread opens its file in the files tab with its line on
// screen; a RIGHT thread matches the new side, a LEFT one the old side.
func TestPRReview_EnterOnThreadJumps(t *testing.T) {
	for _, c := range []struct {
		side, want string
	}{{"RIGHT", "new line 41"}, {"LEFT", "old line 41"}} {
		m := conversationReview(t, []tracker.ReviewComment{{ID: 1, Path: "main.go", Line: 41, Side: c.side, Author: "a", Body: "here"}})
		m.viewString() // the conversation builds its items as it draws
		m = sendKey(m, "enter")
		if m.prReview.Tab != 4 || currentPath(m) != "main.go" {
			t.Fatalf("%s: tab %d file %q", c.side, m.prReview.Tab, currentPath(m))
		}
		if text := stripAnsi(m.viewString()); !strings.Contains(text, c.want) {
			t.Errorf("%s: %q not on screen:\n%s", c.side, c.want, text)
		}
	}
}

// An outdated thread falls back to its hunk header; with no match the
// file opens at its top.
func TestPRReview_OutdatedThreadFallsBackToHunk(t *testing.T) {
	m := conversationReview(t, []tracker.ReviewComment{{ID: 1, Path: "main.go", OriginalLine: 7, Side: "RIGHT", Author: "a", Body: "x",
		DiffHunk: "@@ -1,41 +1,41 @@ func main() {\n ctx"}})
	m.viewString()
	m = sendKey(m, "enter")
	if m.prReview.Tab != 4 || currentPath(m) != "main.go" || m.prReview.Scroll != 0 {
		t.Errorf("tab %d file %q scroll %d", m.prReview.Tab, currentPath(m), m.prReview.Scroll)
	}
}

func TestGolden_PRReviewConversation(t *testing.T) {
	m := goldenModel(t, sizeWide, withItems(3))
	next, _ := m.openPRReview(&tracker.PullRequest{Number: 42, Title: "Add user authentication flow"})
	m = next.(Model)
	m.prReview.now = convNow
	g := m.prReview.Gen
	m = sendMsg(m, prDetailMsg{gen: g, detail: convDetail})
	m = sendMsg(m, prCommentsMsg{gen: g, comments: convThreads})
	m = sendMsg(m, prDiffMsg{gen: g})
	golden.RequireEqual(t, []byte(m.viewString()))
}
