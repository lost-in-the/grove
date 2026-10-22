package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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
}

// openPRReview pushes the review frame for pr. It opens on the files tab
// until the other tabs are built (plan 8b decision 4).
func (m Model) openPRReview(pr *tracker.PullRequest) (tea.Model, tea.Cmd) {
	m.pushView(ViewPRReview)
	m.prReview = &PRReviewState{PR: pr, Tab: 4}
	return m, nil
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

// tabCounts is each tab's count, "-" until its data has arrived.
func (s *PRReviewState) tabCounts() [4]string {
	return [4]string{"-", "-", "-", "-"}
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
	text := Styles.TextMuted.Render("arrives in the next build")
	return text + strings.Repeat("\n", max(0, height-1))
}
