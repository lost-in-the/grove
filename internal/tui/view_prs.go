package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tuilog"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/tracker"
)

// PRViewState holds the state for the PR browser view.
type PRViewState struct {
	PRs              []*tracker.PullRequest
	Cursor           int
	Loading          bool
	Error            string
	WorktreeBranches map[string]bool // branches that have worktrees
	Creating         bool
	CreatingPR       *tracker.PullRequest // PR being created
	ActivityLog      *ActivityLog         // streaming creation progress
	FilterInput      textinput.Model
	Filtering        bool // true when filter input is active (activated by /)
	DetailFocused    bool // true when detail panel has focus (Tab to toggle)
	DetailViewport   viewport.Model
	lastCursor       int // tracks cursor changes to update viewport content
}

func (s *PRViewState) getActivityLog() *ActivityLog { return s.ActivityLog }
func (s *PRViewState) setCreatingDone(errMsg string) {
	s.Creating = false
	s.Error = errMsg
}

func (m Model) fetchPRsCmd() tea.Msg {
	if !tracker.IsGHInstalled() {
		return prsFetchedMsg{err: fmt.Errorf("gh CLI not installed or not authenticated")}
	}

	repo, err := tracker.DetectRepo()
	if err != nil {
		return prsFetchedMsg{err: fmt.Errorf("failed to detect repository: %w", err)}
	}

	gh := tracker.NewGitHubAdapter(repo)
	prs, err := gh.ListPRs(tracker.ListOptions{State: "open", Limit: 30})
	return prsFetchedMsg{prs: prs, err: err}
}

func (m Model) handlePRKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.prState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	s := m.prState

	if s.Loading || s.Creating {
		if key.Matches(msg, m.keys.Escape) {
			m.activeView = ViewDashboard
			m.prState = nil
			return m, nil
		}
		return m, nil
	}

	filtered := filteredPRs(s.PRs, s.FilterInput.Value())

	// When filtering is active, route keys to the textinput
	if s.Filtering {
		var cmd tea.Cmd
		var stop bool
		s.FilterInput, s.Cursor, cmd, stop = handleListFilterKey(msg, m.keys, s.FilterInput, s.Cursor)
		if stop {
			s.Filtering = false
		}
		return m, cmd
	}

	// When detail panel is focused, route keys for scrolling
	if s.DetailFocused {
		if key.Matches(msg, m.keys.Escape) || key.Matches(msg, m.keys.Tab) {
			s.DetailFocused = false
			return m, nil
		}
		handleDetailFocusedKey(msg, m.keys, &s.DetailViewport)
		return m, nil
	}

	// Normal mode (not filtering, list focused)
	return m.handlePRNormalKey(msg, filtered)
}

func (m Model) handlePRNormalKey(msg tea.KeyPressMsg, filtered []*tracker.PullRequest) (tea.Model, tea.Cmd) {
	s := m.prState

	switch {
	case key.Matches(msg, m.keys.Escape):
		m.activeView = ViewDashboard
		m.prState = nil
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if s.Cursor > 0 {
			s.Cursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if s.Cursor < len(filtered)-1 {
			s.Cursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Tab):
		s.DetailFocused = true
		return m, nil

	case msg.String() == "B":
		if len(filtered) > 0 && s.Cursor < len(filtered) {
			pr := filtered[s.Cursor]
			openURL(pr.URL)
		}
		return m, nil

	case key.Matches(msg, m.keys.Filter):
		s.Filtering = true
		return m, s.FilterInput.Focus()

	case key.Matches(msg, m.keys.Enter):
		return m.handlePREnter(s, filtered)
	}

	return m, nil
}

func (m Model) handlePREnter(s *PRViewState, filtered []*tracker.PullRequest) (tea.Model, tea.Cmd) {
	if len(filtered) == 0 || s.Cursor >= len(filtered) {
		return m, nil
	}

	pr := filtered[s.Cursor]
	if s.WorktreeBranches[pr.Branch] {
		s.Error = fmt.Sprintf("worktree already exists for branch %q", pr.Branch)
		return m, nil
	}

	s.Creating = true
	s.CreatingPR = pr
	s.Error = ""
	s.ActivityLog = NewActivityLog(60, 10)
	name := tracker.GenerateWorktreeName("pr", pr.Number, pr.Title)
	return m, tea.Batch(m.spinner.Tick, createPRWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, name, pr.Branch))
}

func filteredPRs(prs []*tracker.PullRequest, filter string) []*tracker.PullRequest {
	if filter == "" {
		return prs
	}
	lower := strings.ToLower(filter)
	var result []*tracker.PullRequest
	for _, pr := range prs {
		if strings.Contains(strings.ToLower(pr.Title), lower) ||
			strings.Contains(strings.ToLower(pr.Branch), lower) ||
			strings.Contains(strings.ToLower(pr.Author), lower) ||
			strings.Contains(fmt.Sprintf("#%d", pr.Number), filter) {
			result = append(result, pr)
		}
	}
	return result
}

func createPRWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot, name, branch string) tea.Cmd {
	return streamingCreateCmd(mgr, stateMgr, projectRoot, name, "pr",
		[]string{fmt.Sprintf("Creating worktree '%s' from PR branch '%s'...", name, branch)},
		func() error { return mgr.CreateFromBranch(name, branch) },
	)
}

// openURL opens a URL in the user's default browser.
func openURL(url string) {
	if url == "" {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		tuilog.Printf("warning: failed to open URL %q: %v", url, err)
	}
}
