package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/plugins/tracker"
)

// prReviewData is one PR's loaded review data, kept for the session and
// reused while the PR's UpdatedAt is unchanged (spec 13.3).
type prReviewData struct {
	updatedAt time.Time
	detail    *tracker.PRDetail
	files     []git.FileDiff
	comments  []tracker.ReviewComment
}

// Each review result carries the generation of the open that asked for it;
// one from an older open is dropped.
type (
	prReviewRepoMsg struct {
		gen       int
		repo      string
		ghMissing bool
		err       error
	}
	prDetailMsg struct {
		gen    int
		detail *tracker.PRDetail
		err    error
	}
	prDiffMsg struct {
		gen   int
		files []git.FileDiff
		err   error
	}
	prCommentsMsg struct {
		gen      int
		comments []tracker.ReviewComment
		err      error
	}
)

// prReviewRepoCmd checks gh once and finds the repository, so the three
// fetches that follow need neither.
func prReviewRepoCmd(gen int) tea.Cmd {
	return func() tea.Msg {
		if !tracker.IsGHInstalled() {
			return prReviewRepoMsg{gen: gen, ghMissing: true}
		}
		repo, err := tracker.DetectRepo()
		if err != nil {
			return prReviewRepoMsg{gen: gen, err: fmt.Errorf("failed to detect repository: %w", err)}
		}
		return prReviewRepoMsg{gen: gen, repo: repo}
	}
}

// prReviewFetchCmds fetch a PR's detail, diff, and review comments at once;
// each lands on its own, so one failure blanks nothing else (spec 13.5).
func prReviewFetchCmds(gen int, repo string, number int) tea.Cmd {
	gh := tracker.NewGitHubAdapter(repo)
	return tea.Batch(
		func() tea.Msg {
			d, err := gh.FetchPRDetail(number)
			return prDetailMsg{gen: gen, detail: d, err: err}
		},
		func() tea.Msg {
			diff, err := gh.FetchPRDiff(number)
			if err != nil {
				return prDiffMsg{gen: gen, err: err}
			}
			files, err := git.ParseDiff(diff)
			return prDiffMsg{gen: gen, files: files, err: err}
		},
		func() tea.Msg {
			cs, err := gh.FetchReviewComments(number)
			return prCommentsMsg{gen: gen, comments: cs, err: err}
		},
	)
}

// reviewFor returns the open review when msg's generation is its own.
func (m Model) reviewFor(gen int) *PRReviewState {
	if s := m.prReview; s != nil && s.Gen == gen {
		return s
	}
	return nil
}

func (m Model) handlePRReviewRepo(msg prReviewRepoMsg) (tea.Model, tea.Cmd) {
	s := m.reviewFor(msg.gen)
	if s == nil {
		return m, nil
	}
	if msg.ghMissing || msg.err != nil {
		s.GHMissing, s.RepoErr = msg.ghMissing, msg.err
		s.LoadingDetail, s.LoadingDiff, s.LoadingComments = false, false, false
		return m, nil
	}
	m.ghRepo = msg.repo
	return m, prReviewFetchCmds(msg.gen, msg.repo, s.PR.Number)
}

func (m Model) handlePRDetail(msg prDetailMsg) (tea.Model, tea.Cmd) {
	if s := m.reviewFor(msg.gen); s != nil {
		s.Detail, s.DetailErr, s.LoadingDetail = msg.detail, msg.err, false
		m.cacheReviewIfComplete()
	}
	return m, nil
}

func (m Model) handlePRDiff(msg prDiffMsg) (tea.Model, tea.Cmd) {
	if s := m.reviewFor(msg.gen); s != nil {
		s.Files, s.DiffErr, s.LoadingDiff = msg.files, msg.err, false
		m.cacheReviewIfComplete()
	}
	return m, nil
}

func (m Model) handlePRComments(msg prCommentsMsg) (tea.Model, tea.Cmd) {
	if s := m.reviewFor(msg.gen); s != nil {
		s.Comments, s.CommentsErr, s.LoadingComments = msg.comments, msg.err, false
		m.cacheReviewIfComplete()
	}
	return m, nil
}

// cacheReviewIfComplete keeps the open review's data once all three
// fetches have succeeded. The map is shared by every copy of the model.
func (m *Model) cacheReviewIfComplete() {
	s := m.prReview
	if s.loading() || s.DetailErr != nil || s.DiffErr != nil || s.CommentsErr != nil {
		return
	}
	if m.prReviewCache == nil {
		m.prReviewCache = map[int]*prReviewData{}
	}
	m.prReviewCache[s.PR.Number] = &prReviewData{updatedAt: s.PR.UpdatedAt, detail: s.Detail, files: s.Files, comments: s.Comments}
}
