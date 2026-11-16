package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/plugins/tracker"
)

// prCommitDiffMsg is one commit's diff, for the files tab's commit scope.
type prCommitDiffMsg struct {
	gen   int
	sha   string
	files []git.FileDiff
	err   error
}

// commitDiffCmd fetches a commit's diff over REST and parses it.
func commitDiffCmd(gen int, repo, sha string) tea.Cmd {
	return func() tea.Msg {
		diff, err := tracker.NewGitHubAdapter(repo).FetchCommitDiff(sha)
		if err != nil {
			return prCommitDiffMsg{gen: gen, sha: sha, err: err}
		}
		files, err := git.ParseDiff(diff)
		return prCommitDiffMsg{gen: gen, sha: sha, files: files, err: err}
	}
}

func commitCacheKey(number int, sha string) string { return fmt.Sprintf("%d:%s", number, sha) }

// commitItems is tab 2: one row per commit, "sha headline", as gh lists
// them (oldest first).
func commitItems(commits []tracker.PRCommit) [][]string {
	items := make([][]string, len(commits))
	for i, c := range commits {
		items[i] = []string{Styles.WarningText.Render(c.SHA) + " " + safeText(c.Message)}
	}
	return items
}

// renderCommitsTab is tab 2.
func (m Model) renderCommitsTab(width, height int) string {
	s := m.prReview
	switch {
	case s.LoadingDetail:
		return Styles.TextMuted.Render(m.spinner.View() + " loading commits")
	case s.DetailErr != nil:
		return Styles.ErrorText.Render("commits unavailable: " + safeText(s.DetailErr.Error()))
	case s.Detail == nil || len(s.Detail.Commits) == 0:
		return Styles.TextMuted.Render("no commits")
	}
	return renderItems(commitItems(s.Detail.Commits), s.Cursor[2], width, height)
}

// scopeToCommit shows one commit's files in the files tab (plan 8c
// decision 5), from the session cache or by fetching its diff.
func (m Model) scopeToCommit(c tracker.PRCommit) (Model, tea.Cmd) {
	s := m.prReview
	sha := c.OID
	if sha == "" {
		sha = c.SHA
	}
	s.Tab, s.ScopedSHA, s.ScopeErr = 4, sha, nil
	if files, ok := m.commitDiffs[commitCacheKey(s.PR.Number, sha)]; ok {
		s.LoadingScope = false
		s.showFiles(files)
		return m, nil
	}
	s.LoadingScope = true
	repo := m.ghRepo
	if repo == "" {
		repo = repoFromPRURL(s.PR.URL)
	}
	return m, tea.Batch(m.spinner.Tick, commitDiffCmd(s.Gen, repo, sha))
}

// unscope returns the files tab to the whole PR's diff.
func (s *PRReviewState) unscope() {
	s.ScopedSHA, s.LoadingScope, s.ScopeErr = "", false, nil
	s.showFiles(s.prFiles)
}

func (m Model) handlePRCommitDiff(msg prCommitDiffMsg) (tea.Model, tea.Cmd) {
	s := m.reviewFor(msg.gen)
	if s == nil || s.ScopedSHA != msg.sha {
		return m, nil
	}
	s.LoadingScope, s.ScopeErr = false, msg.err
	if msg.err == nil {
		if m.commitDiffs == nil {
			m.commitDiffs = map[string][]git.FileDiff{}
		}
		m.commitDiffs[commitCacheKey(s.PR.Number, msg.sha)] = msg.files
		s.showFiles(msg.files)
	}
	return m, nil
}

// shortSHA is a sha cut to the 7 characters the review shows.
func shortSHA(sha string) string { return sha[:min(7, len(sha))] }
