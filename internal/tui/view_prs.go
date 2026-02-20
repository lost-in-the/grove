package tui

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
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
	Filter           string
	ShowPreview      bool // toggle PR preview panel with Tab
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

func (m Model) handlePRKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

	filtered := filteredPRs(s.PRs, s.Filter)

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

	case msg.Type == tea.KeyBackspace:
		if len(s.Filter) > 0 {
			s.Filter = s.Filter[:len(s.Filter)-1]
			s.Cursor = 0
		}
		return m, nil

	case msg.Type == tea.KeyRunes:
		s.Filter += string(msg.Runes)
		s.Cursor = 0
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
			return prWorktreeCreatedMsg{name: name, err: fmt.Errorf("worktree created but not found")}
		}

		projectName := mgr.GetProjectName()

		// Register in state
		if stateMgr != nil {
			now := time.Now()
			wsState := &state.WorktreeState{
				Path:           wt.Path,
				Branch:         wt.Branch,
				CreatedAt:      now,
				LastAccessedAt: now,
			}
			if err := stateMgr.AddWorktree(name, wsState); err != nil {
				tuilog.Printf("warning: failed to register PR worktree %q in state: %v", name, err)
			}
		}

		// Create tmux session
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			if err := tmux.CreateSession(sessionName, wt.Path); err != nil {
				tuilog.Printf("warning: failed to create tmux session %q: %v", sessionName, err)
			}
		}

		// Run post-create hooks
		var hookBuf bytes.Buffer
		var hookExecErr error
		hookExecutor, hookErr := hooks.NewExecutor()
		if hookErr == nil && hookExecutor.HasHooksForEvent(hooks.EventPostCreate) {
			hookExecutor.Output = &hookBuf
			hookCtx := &hooks.ExecutionContext{
				Event:        hooks.EventPostCreate,
				Worktree:     name,
				WorktreeFull: projectName + "-" + name,
				Branch:       wt.Branch,
				Project:      projectName,
				MainPath:     projectRoot,
				NewPath:      wt.Path,
			}
			hookExecErr = hookExecutor.Execute(hooks.EventPostCreate, hookCtx)
			if hookExecErr != nil {
				tuilog.Printf("warning: post-create hook failed for PR worktree %q: %v", name, hookExecErr)
			}
		}

		return prWorktreeCreatedMsg{name: name, path: wt.Path, hookOutput: hookBuf.String(), hookErr: hookExecErr}
	}
}
