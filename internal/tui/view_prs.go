package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tuilog"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/LeahArmstrong/grove-cli/plugins/tracker"
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
	FilterInput      textinput.Model
	ShowPreview      bool // toggle PR preview panel with Tab
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
		s.ShowPreview = !s.ShowPreview
		return m, nil

	case msg.String() == "o":
		if s.ShowPreview && len(filtered) > 0 && s.Cursor < len(filtered) {
			pr := filtered[s.Cursor]
			if pr.URL != "" {
				var cmd *exec.Cmd
				switch runtime.GOOS {
				case "darwin":
					cmd = exec.Command("open", pr.URL)
				case "windows":
					cmd = exec.Command("cmd", "/c", "start", pr.URL)
				default:
					cmd = exec.Command("xdg-open", pr.URL)
				}
				if err := cmd.Start(); err != nil {
					tuilog.Printf("warning: failed to open URL %q: %v", pr.URL, err)
				}
			}
		}
		return m, nil

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
			name := tracker.GenerateWorktreeName("pr", pr.Number, pr.Title)
			return m, tea.Batch(m.spinner.Tick, createPRWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, name, pr.Branch))
		}
		return m, nil

	default:
		// Route remaining keys through the filter textinput
		prevVal := s.FilterInput.Value()
		var cmd tea.Cmd
		s.FilterInput, cmd = s.FilterInput.Update(msg)
		if s.FilterInput.Value() != prevVal {
			s.Cursor = 0
		}
		return m, cmd
	}
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
	return func() tea.Msg {
		err := mgr.CreateFromBranch(name, branch)
		if err != nil {
			return prWorktreeCreatedMsg{name: name, err: err}
		}
		wt, err := mgr.Find(name)
		if err != nil || wt == nil {
			return prWorktreeCreatedMsg{name: name, err: errWorktreeNotFound}
		}

		result := runPostCreate(mgr, stateMgr, projectRoot, name, wt)
		return prWorktreeCreatedMsg{name: name, path: wt.Path, hookOutput: result.hookOutput, hookErr: result.hookErr}
	}
}
