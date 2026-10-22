package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/plugins/tracker"
)

// prReviewChromeRows are the review frame's rows besides the body and the
// footer: header, tabs, and a blank row.
const prReviewChromeRows = 3

// prReviewTabNames are the tab labels, tab 1 first (spec 13.2).
var prReviewTabNames = [4]string{"conversation", "commits", "checks", "files"}

// PRReviewState is the review frame for one pull request (spec 13).
type PRReviewState struct {
	PR  *tracker.PullRequest
	Tab int // 1 conversation, 2 commits, 3 checks, 4 files
	Gen int // this open's generation; results from another are dropped

	Detail   *tracker.PRDetail
	Files    []git.FileDiff
	Comments []tracker.ReviewComment

	LoadingDetail, LoadingDiff, LoadingComments bool
	DetailErr, DiffErr, CommentsErr             error
	GHMissing                                   bool
	RepoErr                                     error

	Cursor       [5]int         // the cursor of tabs 1-3, by tab number
	FileIdx      int            // the files tab's current file
	prFiles      []git.FileDiff // the PR's whole diff; Files may be one commit's
	ScopedSHA    string         // the commit the files tab shows, or ""
	LoadingScope bool
	ScopeErr     error
	Order        []int             // the files in the rail's order
	Scroll       int               // first hunk-pane line shown
	Folded       bool              // z: runs of removed lines collapsed
	RailChoice   int               // t: 0 by width, 1 shown, -1 hidden
	Viewed       map[string]string // path -> fileHash of the saved marks

	hashes     []string   // fileHash of each file, computed once per diff
	conv       []convItem // tab 1's items, built at convW
	convW      int
	now        time.Time // tests pin it; zero means time.Now
	cacheKey   [3]int    // file, width, folded of cacheLines
	cacheLines []string
}

// setFiles takes a loaded diff: the files, their rail order, and the first
// file in it.
func (s *PRReviewState) setFiles(files []git.FileDiff) {
	s.prFiles = files
	if s.ScopedSHA == "" {
		s.showFiles(files)
	}
}

// showFiles makes files the files tab's: their rail order, hashes, and
// the first file in the rail.
func (s *PRReviewState) showFiles(files []git.FileDiff) {
	s.Files, s.FileIdx, s.Scroll = files, 0, 0
	s.Order, _ = railLines(files, nil)
	s.hashes = make([]string, len(files))
	for i, f := range files {
		s.hashes[i] = fileHash(f)
	}
	if len(s.Order) > 0 {
		s.FileIdx = s.Order[0]
	}
	s.cacheLines = nil
}

// fileLines is the current file's rendered lines at width, cached until
// the file, the width, or the fold changes.
func (s *PRReviewState) fileLines(width int) []string {
	folded := 0
	if s.Folded {
		folded = 1
	}
	key := [3]int{s.FileIdx, width, folded}
	if s.cacheLines == nil || s.cacheKey != key {
		s.cacheKey, s.cacheLines = key, renderFileDiff(s.Files[s.FileIdx], width, s.Folded)
	}
	return s.cacheLines
}

func (s *PRReviewState) loading() bool {
	return s.LoadingDetail || s.LoadingDiff || s.LoadingComments
}

// openPRReview pushes the review frame for pr, on its conversation tab.
// Unchanged PRs open
// from the session cache; others fetch their data in the background.
func (m Model) openPRReview(pr *tracker.PullRequest) (tea.Model, tea.Cmd) {
	m.prReviewGen++
	m.pushView(ViewPRReview)
	s := &PRReviewState{PR: pr, Tab: 1, Gen: m.prReviewGen}
	m.prReview = s
	s.Viewed = savedViewed(m.projectRoot, pr.Number)
	if c := m.prReviewCache[pr.Number]; c != nil && !pr.UpdatedAt.IsZero() && c.updatedAt.Equal(pr.UpdatedAt) {
		s.Detail, s.Comments = c.detail, c.comments
		s.setFiles(c.files)
		return m, nil
	}
	s.LoadingDetail, s.LoadingDiff, s.LoadingComments = true, true, true
	if m.ghRepo != "" {
		return m, tea.Batch(m.spinner.Tick, prReviewFetchCmds(s.Gen, m.ghRepo, pr.Number))
	}
	return m, tea.Batch(m.spinner.Tick, prReviewRepoCmd(s.Gen, pr.URL))
}

func (m Model) handlePRReviewKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.prReview
	if s == nil {
		m.popView()
		return m, nil
	}
	switch k := msg.String(); {
	case key.Matches(msg, m.keys.Escape):
		m.popView()
		m.prReview = nil
	case len(k) == 1 && k >= "1" && k <= "4":
		s.Tab = int(k[0] - '0')
	case k == "B":
		openURL(s.PR.URL)
	case s.Tab == 4 && k == "a":
		if s.ScopedSHA != "" {
			s.unscope()
		}
	case s.Tab == 4 && k == "v" && s.ScopedSHA != "":
		m.toast.Show(NewToast("viewed marks apply to the whole PR; a shows all commits", ToastInfo))
	case s.Tab == 4 && k == "enter" && len(s.Files) > 0 && !s.LoadingScope:
		return m.openInEditor()
	case s.Tab == 4 && len(s.Files) > 0 && !s.LoadingScope:
		m.handleFilesKey(k)
	case s.Tab == 2 && k == "enter":
		if c := s.Cursor[2]; s.Detail != nil && c < len(s.Detail.Commits) {
			return m.scopeToCommit(s.Detail.Commits[c])
		}
	case s.Tab == 1 && k == "enter":
		if c := s.Cursor[1]; c < len(s.conv) && s.conv[c].kind == convThread {
			m.jumpToThread(s.conv[c])
		}
	case s.Tab != 4:
		if s.Tab == 1 {
			w, _ := m.prReviewBodySize()
			m.conversation(w) // the cursor's bound is the item count
		}
		s.moveCursor(k)
	}
	return m, nil
}

// handleFilesKey moves between files and scrolls the hunk pane (plan 8b
// decision 9).
func (m Model) handleFilesKey(k string) {
	s := m.prReview
	_, h := m.prReviewBodySize()
	paneH := max(1, h-2) // below the file's title and rule
	switch k {
	case "J", "ctrl+n", "K", "ctrl+p":
		step := 1
		if k == "K" || k == "ctrl+p" {
			step = -1
		}
		for i, f := range s.Order {
			if f == s.FileIdx && i+step >= 0 && i+step < len(s.Order) {
				s.FileIdx, s.Scroll = s.Order[i+step], 0
				break
			}
		}
	case "j", "down":
		s.Scroll++
	case "k", "up":
		s.Scroll--
	case "pgdown", "space":
		s.Scroll += paneH
	case "pgup":
		s.Scroll -= paneH
	case "ctrl+d":
		s.Scroll += paneH / 2
	case "ctrl+u":
		s.Scroll -= paneH / 2
	case "g":
		s.Scroll = 0
	case "G":
		s.Scroll = 1 << 30
	case "t":
		if m.railShown() {
			s.RailChoice = -1
		} else {
			s.RailChoice = 1
		}
	case "z":
		s.Folded = !s.Folded
	case "v":
		m.toggleViewed()
	}
	s.Scroll = max(0, min(s.Scroll, len(s.fileLines(m.paneWidth()))-paneH))
}

// railShown reports whether the files tab draws its rail: once t has been
// pressed, as chosen, whatever the width; before that, from railBreakpoint
// columns up.
func (m Model) railShown() bool {
	if c := m.prReview.RailChoice; c != 0 {
		return c > 0
	}
	return m.width >= railBreakpoint
}

// railWidth is the rail's width in a body width wide.
func railWidth(width int) int { return max(railMinWidth, width*railShare/100) }

// paneWidth is the hunk pane's width: the body, less the rail and its
// divider when shown.
func (m Model) paneWidth() int {
	w, _ := m.prReviewBodySize()
	if m.railShown() {
		return max(1, w-railWidth(w)-3)
	}
	return w
}

// itemCount is the number of items tab tab lists.
func (s *PRReviewState) itemCount(tab int) int {
	switch {
	case tab == 1:
		return len(s.conv)
	case tab == 2 && s.Detail != nil:
		return len(s.Detail.Commits)
	case tab == 3 && s.Detail != nil:
		return len(s.Detail.Checks)
	}
	return 0
}

// moveCursor moves the active list tab's cursor for j/k and friends.
func (s *PRReviewState) moveCursor(k string) {
	c := &s.Cursor[s.Tab]
	switch k {
	case "j", "down":
		*c++
	case "k", "up":
		*c--
	case "g":
		*c = 0
	case "G":
		*c = 1 << 30
	}
	*c = max(0, min(*c, s.itemCount(s.Tab)-1))
}

// tabCounts is each tab's count, "-" until its data has arrived. The
// conversation counts comments, reviews, and review threads.
func (s *PRReviewState) tabCounts() [4]string {
	counts := [4]string{"-", "-", "-", "-"}
	if d := s.Detail; d != nil {
		threads := 0
		for _, c := range s.Comments {
			if c.InReplyTo == 0 {
				threads++
			}
		}
		counts[0] = fmt.Sprint(len(d.Comments) + len(d.Reviews) + threads)
		counts[1] = fmt.Sprint(len(d.Commits))
		counts[2] = fmt.Sprint(len(d.Checks))
	}
	if !s.LoadingDiff && s.DiffErr == nil && !s.GHMissing && s.RepoErr == nil {
		counts[3] = fmt.Sprint(len(s.prFiles))
	}
	return counts
}

// prReviewFooter shows the active tab's keys.
func (m Model) prReviewFooter() string {
	hints := []Hint{{"j/k", "move"}, {"1-4", "tab"}, {"B", "browser"}, {"esc", "back"}}
	switch m.prReview.Tab {
	case 1:
		hints = []Hint{{"j/k", "move"}, {"enter", "go to line"}, {"1-4", "tab"}, {"B", "browser"}, {"esc", "back"}}
	case 2:
		hints = []Hint{{"j/k", "move"}, {"enter", "its files"}, {"1-4", "tab"}, {"B", "browser"}, {"esc", "back"}}
	case 4:
		hints = m.helpFooter.CompactHints(ViewPRReview)
		if m.prReview.ScopedSHA != "" {
			hints = append([]Hint{{"a", "all commits"}}, hints...)
		}
	}
	return m.helpFooter.RenderCompactWithHints(hints, m.width-4)
}

// prReviewBodySize is the size of the frame's body: the terminal minus the
// chrome and the footer's real height.
func (m Model) prReviewBodySize() (int, int) {
	return max(1, m.width-2), max(1, m.height-prReviewChromeRows-lipgloss.Height(m.prReviewFooter()))
}

// renderPRReviewFrame draws the review as a full-screen frame: header, tab
// row, a blank row, the active tab's body, and the footer.
func (m Model) renderPRReviewFrame() string {
	s := m.prReview
	footer := m.prReviewFooter()
	counts := s.tabCounts()
	labels := make([]string, len(prReviewTabNames))
	for i, name := range prReviewTabNames {
		labels[i] = name + " " + counts[i]
	}
	if s.ScopedSHA != "" {
		labels[3] = "files · " + shortSHA(s.ScopedSHA)
	}
	w, h := m.prReviewBodySize()
	body := strings.Join([]string{renderTabRow(labels, s.Tab-1, w), "", m.renderPRReviewBody(w, h)}, "\n")
	body = lipgloss.NewStyle().Padding(0, 1).Render(body)
	body = m.clampBodyToHeight(body, footer)
	return lipgloss.JoinVertical(lipgloss.Left, m.prReviewHeader(), body, footer)
}

// prReviewHeader is "<project> ▸ #<n> <title>", the title cut to fit.
func (m Model) prReviewHeader() string {
	inner := max(0, m.width-2) // the header bar pads one cell each side
	left := Styles.Header.Render(m.projectName) + Styles.TextMuted.Render(" ▸ ") +
		Styles.TextBright.Render(fmt.Sprintf("#%d %s", m.prReview.PR.Number, safeText(m.prReview.PR.Title)))
	left = ansi.Truncate(left, inner, "…")
	return headerBar(left+strings.Repeat(" ", max(0, inner-lipgloss.Width(left))), m.width)
}

// renderPRReviewBody is the active tab's content, height rows tall.
func (m Model) renderPRReviewBody(width, height int) string {
	s := m.prReview
	var text string
	switch {
	case s.GHMissing:
		text = Styles.WarningText.Render("gh required") + "\n" +
			Styles.TextMuted.Render("Install the GitHub CLI and run gh auth login to review pull requests.")
	case s.RepoErr != nil:
		text = Styles.ErrorText.Render(safeText(s.RepoErr.Error()))
	case s.Tab == 1:
		text = m.renderConversationTab(width, height)
	case s.Tab == 2:
		text = m.renderCommitsTab(width, height)
	case s.Tab == 3:
		text = m.renderChecksTab(width, height)
	case s.Tab != 4:
		text = Styles.TextMuted.Render("arrives in the next build")
	default:
		text = m.renderFilesTab(width, height)
	}
	// Text of the frame's own wraps to the body; the files tab sizes itself.
	text = lipgloss.NewStyle().Width(width).MaxHeight(height).Render(text)
	return text + strings.Repeat("\n", max(0, height-lipgloss.Height(text)))
}

// renderChecksTab is tab 3: the check rollup, or why there is none.
func (m Model) renderChecksTab(width, height int) string {
	s := m.prReview
	switch {
	case s.LoadingDetail:
		return Styles.TextMuted.Render(m.spinner.View() + " loading checks")
	case s.DetailErr != nil:
		return Styles.ErrorText.Render("checks unavailable: " + safeText(s.DetailErr.Error()))
	case s.Detail == nil || len(s.Detail.Checks) == 0:
		return Styles.TextMuted.Render("no checks")
	}
	return renderItems(checkItems(s.Detail.Checks), s.Cursor[3], width, height)
}

// renderFilesTab is tab 4: the diff, or why there is none.
func (m Model) renderFilesTab(width, height int) string {
	s := m.prReview
	switch {
	case s.LoadingScope:
		return Styles.TextMuted.Render(m.spinner.View() + " loading " + shortSHA(s.ScopedSHA))
	case s.ScopeErr != nil:
		return Styles.ErrorText.Render("diff unavailable: " + safeText(s.ScopeErr.Error()))
	case s.ScopedSHA == "" && s.LoadingDiff:
		return Styles.TextMuted.Render(m.spinner.View() + " loading diff")
	case s.ScopedSHA == "" && s.DiffErr != nil:
		return Styles.ErrorText.Render("diff unavailable: " + safeText(s.DiffErr.Error()))
	case len(s.Files) == 0:
		return Styles.TextMuted.Render("no files changed")
	}
	paneW := m.paneWidth()
	lines := s.fileLines(paneW)
	paneH := max(1, height-2)
	start := max(0, min(s.Scroll, len(lines)-paneH))
	shown := lines[start:min(len(lines), start+paneH)]
	pane := renderFileHeader(s.Files[s.FileIdx], paneW) + "\n" + strings.Join(shown, "\n")
	if !m.railShown() {
		return pane
	}
	_, rows := railLines(s.Files, s.viewedNow())
	rw := railWidth(width)
	rail := lipgloss.NewStyle().Width(rw).Height(height).MaxHeight(height).Render(renderRail(rows, s.FileIdx, rw, height))
	divider := Styles.TextMuted.Render(strings.TrimSuffix(strings.Repeat(" │ \n", height), "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, rail, divider, pane)
}
