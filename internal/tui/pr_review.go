package tui

import (
	"fmt"
	"strings"

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
}

func (s *PRReviewState) loading() bool {
	return s.LoadingDetail || s.LoadingDiff || s.LoadingComments
}

// openPRReview pushes the review frame for pr. It opens on the files tab
// until the other tabs are built (plan 8b decision 4). Unchanged PRs open
// from the session cache; others fetch their data in the background.
func (m Model) openPRReview(pr *tracker.PullRequest) (tea.Model, tea.Cmd) {
	m.prReviewGen++
	m.pushView(ViewPRReview)
	s := &PRReviewState{PR: pr, Tab: 4, Gen: m.prReviewGen}
	m.prReview = s
	if c := m.prReviewCache[pr.Number]; c != nil && c.updatedAt.Equal(pr.UpdatedAt) {
		s.Detail, s.Files, s.Comments = c.detail, c.files, c.comments
		return m, nil
	}
	s.LoadingDetail, s.LoadingDiff, s.LoadingComments = true, true, true
	if m.ghRepo != "" {
		return m, tea.Batch(m.spinner.Tick, prReviewFetchCmds(s.Gen, m.ghRepo, pr.Number))
	}
	return m, tea.Batch(m.spinner.Tick, prReviewRepoCmd(s.Gen))
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
	}
	return m, nil
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
		counts[3] = fmt.Sprint(len(s.Files))
	}
	return counts
}

func (m Model) prReviewFooter() string {
	return m.helpFooter.RenderCompact(ViewPRReview, m.width-4)
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
		Styles.TextBright.Render(fmt.Sprintf("#%d %s", m.prReview.PR.Number, m.prReview.PR.Title))
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
		text = Styles.ErrorText.Render(s.RepoErr.Error())
	case s.Tab != 4:
		text = Styles.TextMuted.Render("arrives in the next build")
	default:
		text = m.renderFilesTab(width, height)
	}
	return text + strings.Repeat("\n", max(0, height-lipgloss.Height(text)))
}

// renderFilesTab is tab 4: the diff, or why there is none.
func (m Model) renderFilesTab(width, height int) string {
	s := m.prReview
	switch {
	case s.LoadingDiff:
		return Styles.TextMuted.Render(m.spinner.View() + " loading diff")
	case s.DiffErr != nil:
		return Styles.ErrorText.Render("diff unavailable: " + s.DiffErr.Error())
	case len(s.Files) == 0:
		return Styles.TextMuted.Render("no files changed")
	}
	lines := make([]string, 0, len(s.Files))
	for _, f := range s.Files {
		lines = append(lines, f.Path())
	}
	return strings.Join(lines, "\n")
}
