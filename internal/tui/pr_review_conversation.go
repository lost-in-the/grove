package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/plugins/tracker"
)

type convKind int

const (
	convSummary convKind = iota
	convComment
	convReview
	convThread
)

// convItem is one entry of the conversation tab. A thread also carries
// its anchor, for enter to jump to.
type convItem struct {
	kind  convKind
	at    time.Time
	lines []string
	path  string
	line  int    // 0 when the thread is outdated
	side  string // RIGHT (new side) or LEFT (old side)
	hunk  string // the thread's diff_hunk, for the fallback match
}

// reviewVerbs are how a review's state reads after its author.
var reviewVerbs = map[string]string{
	"APPROVED":          "approved",
	"CHANGES_REQUESTED": "requested changes",
	"COMMENTED":         "commented",
	"DISMISSED":         "dismissed",
}

// buildConversation is tab 1 (plan 8c decision 2): a summary of the
// checks, then the PR's comments, reviews, and review threads, oldest
// first, replies under their thread. Bodies render as markdown, indented
// within width.
func buildConversation(d *tracker.PRDetail, cs []tracker.ReviewComment, width int, now time.Time) []convItem {
	byline := func(author string, at time.Time, verb string) string {
		s := Styles.TextBright.Render("@" + safeText(author))
		if verb != "" {
			s += " " + verb
		}
		return s + Styles.TextMuted.Render(" · "+shortAge(at, now))
	}
	body := func(md string) []string {
		md = strings.TrimSpace(md)
		if md == "" {
			return nil
		}
		var out []string
		for _, l := range strings.Split(strings.Trim(renderMarkdown(md, max(10, width-4)), "\n"), "\n") {
			out = append(out, "    "+l)
		}
		return out
	}

	var items []convItem
	if d != nil {
		for _, c := range d.Comments {
			items = append(items, convItem{kind: convComment, at: c.CreatedAt, lines: append([]string{byline(c.Author, c.CreatedAt, "")}, body(c.Body)...)})
		}
		for _, r := range d.Reviews {
			verb, ok := reviewVerbs[r.State]
			if !ok {
				verb = strings.ToLower(safeText(r.State))
			}
			items = append(items, convItem{kind: convReview, at: r.SubmittedAt, lines: append([]string{byline(r.Author, r.SubmittedAt, verb)}, body(r.Body)...)})
		}
	}
	replies := map[int64][]tracker.ReviewComment{}
	for _, c := range cs {
		if c.InReplyTo != 0 {
			replies[c.InReplyTo] = append(replies[c.InReplyTo], c)
		}
	}
	for _, root := range cs {
		if root.InReplyTo != 0 {
			continue
		}
		where := fmt.Sprintf("%s:%d", safeText(root.Path), root.Line)
		if root.Line == 0 {
			where = fmt.Sprintf("%s (outdated, was line %d)", safeText(root.Path), root.OriginalLine)
		}
		lines := []string{Styles.Header.Render(where)}
		for _, c := range append([]tracker.ReviewComment{root}, replies[root.ID]...) {
			lines = append(lines, "  "+byline(c.Author, c.CreatedAt, ""))
			lines = append(lines, body(c.Body)...)
		}
		items = append(items, convItem{kind: convThread, at: root.CreatedAt, lines: lines,
			path: root.Path, line: root.Line, side: root.Side, hunk: root.DiffHunk})
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].at.Before(items[b].at) })

	if d != nil && len(d.Checks) > 0 {
		var n [3]int
		for _, c := range d.Checks {
			n[checkBucket(c.State)]++
		}
		var parts []string
		for _, p := range []struct {
			n     int
			label string
			style func(...string) string
		}{{n[checkPassed], "passed", Styles.SuccessText.Render}, {n[checkFailed], "failed", Styles.ErrorText.Render}, {n[checkRunning], "running", Styles.WarningText.Render}} {
			if p.n > 0 {
				parts = append(parts, p.style(fmt.Sprintf("%d %s", p.n, p.label)))
			}
		}
		summary := Styles.TextMuted.Render("checks: ") + strings.Join(parts, Styles.TextMuted.Render(" · "))
		items = append([]convItem{{kind: convSummary, lines: []string{summary}}}, items...)
	}
	return items
}

// threadRow is the row of fileLines (unfolded) that a thread points at:
// its line on its side, else the hunk whose header its diff_hunk starts
// with, else 0.
func threadRow(f git.FileDiff, it convItem) int {
	header, _, _ := strings.Cut(it.hunk, "\n")
	row, hunkRow := 0, -1
	for _, h := range f.Hunks {
		if hunkRow < 0 && header != "" && strings.HasPrefix(h.Header, header) {
			hunkRow = row
		}
		row++
		for _, l := range h.Lines {
			if it.line > 0 {
				if (it.side == "LEFT" && l.OldNo == it.line && l.Kind != git.LineAdded) ||
					(it.side != "LEFT" && l.NewNo == it.line && l.Kind != git.LineRemoved) {
					return row
				}
			}
			row++
		}
	}
	return max(0, hunkRow)
}

// jumpToThread opens the files tab at a thread's file and line.
func (m Model) jumpToThread(it convItem) {
	s := m.prReview
	for i, f := range s.Files {
		if f.Path() != it.path {
			continue
		}
		s.Tab, s.FileIdx, s.Folded = 4, i, false
		_, h := m.prReviewBodySize()
		paneH := max(1, h-2)
		lines := s.fileLines(m.paneWidth())
		s.Scroll = max(0, min(threadRow(f, it)-paneH/3, len(lines)-paneH))
		return
	}
}

// conversation is the open review's tab 1 items at width, built on first
// draw and kept until new data arrives or the width changes.
func (m Model) conversation(width int) []convItem {
	s := m.prReview
	if s.conv == nil || s.convW != width {
		now := s.now
		if now.IsZero() {
			now = time.Now()
		}
		s.conv, s.convW = buildConversation(s.Detail, s.Comments, width, now), width
	}
	return s.conv
}

// renderConversationTab is tab 1.
func (m Model) renderConversationTab(width, height int) string {
	s := m.prReview
	switch {
	case s.LoadingDetail && s.LoadingComments:
		return Styles.TextMuted.Render(m.spinner.View() + " loading conversation")
	case s.DetailErr != nil && s.CommentsErr != nil:
		return Styles.ErrorText.Render("conversation unavailable: " + safeText(s.DetailErr.Error()))
	}
	items := m.conversation(width)
	lines := make([][]string, len(items))
	talk := 0
	for i, it := range items {
		lines[i] = append(it.lines, "")
		if it.kind != convSummary {
			talk++
		}
	}
	if talk == 0 {
		lines = append(lines, []string{Styles.TextMuted.Render("no comments yet")})
	}
	return renderItems(lines, s.Cursor[1], width, height)
}
