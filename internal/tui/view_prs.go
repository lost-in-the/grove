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

// newPRFilterInput creates a configured textinput for PR filtering.
func newPRFilterInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = "Filter: "
	ti.Placeholder = ""
	ti.CharLimit = 100
	return ti
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
		switch {
		case key.Matches(msg, m.keys.Escape):
			s.Filtering = false
			s.FilterInput.Blur()
			s.FilterInput.SetValue("")
			s.Cursor = 0
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			s.Filtering = false
			s.FilterInput.Blur()
			return m, nil
		default:
			prevVal := s.FilterInput.Value()
			var cmd tea.Cmd
			s.FilterInput, cmd = s.FilterInput.Update(msg)
			if s.FilterInput.Value() != prevVal {
				s.Cursor = 0
			}
			return m, cmd
		}
	}

	// When detail panel is focused, route keys for scrolling
	if s.DetailFocused {
		switch {
		case key.Matches(msg, m.keys.Escape):
			s.DetailFocused = false
			return m, nil
		case key.Matches(msg, m.keys.Tab):
			s.DetailFocused = false
			return m, nil
		case key.Matches(msg, m.keys.Up):
			s.DetailViewport.ScrollUp(1)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			s.DetailViewport.ScrollDown(1)
			return m, nil
		case msg.String() == "g":
			s.DetailViewport.GotoTop()
			return m, nil
		case msg.String() == "G":
			s.DetailViewport.GotoBottom()
			return m, nil
		case msg.String() == "ctrl+u":
			s.DetailViewport.HalfPageUp()
			return m, nil
		case msg.String() == "ctrl+d":
			s.DetailViewport.HalfPageDown()
			return m, nil
		default:
			// Swallow all other keys while detail is focused
			return m, nil
		}
	}

	// Normal mode (not filtering, list focused)
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
		if len(filtered) > 0 && s.Cursor < len(filtered) {
			pr := filtered[s.Cursor]
			// Check if worktree already exists for this branch
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
		return m, nil
	}

	return m, nil
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
	ch := make(chan creationEvent, 10)

	go func() {
		defer close(ch)

		ch <- creationEvent{line: fmt.Sprintf("Creating worktree '%s' from PR branch '%s'...", name, branch)}

		err := mgr.CreateFromBranch(name, branch)
		if err != nil {
			ch <- creationEvent{done: true, name: name, err: err}
			return
		}

		wt, err := mgr.Find(name)
		if err != nil || wt == nil {
			ch <- creationEvent{done: true, name: name, err: errWorktreeNotFound}
			return
		}

		result := runPostCreateStreaming(ch, mgr, stateMgr, projectRoot, name, wt)
		ch <- creationEvent{
			done:       true,
			name:       name,
			path:       wt.Path,
			hookOutput: result.hookOutput,
			hookErr:    result.hookErr,
		}
	}()

	return readCreationLog(ch, "pr")
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
