package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

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

	case key.Matches(msg, m.keys.Enter):
		if len(filtered) > 0 && s.Cursor < len(filtered) {
			pr := filtered[s.Cursor]
			// Check if worktree already exists for this branch
			if s.WorktreeBranches[pr.Branch] {
				s.Error = fmt.Sprintf("worktree already exists for branch %q", pr.Branch)
				return m, nil
			}
			s.Creating = true
			s.Error = ""
			name := tracker.GenerateWorktreeName("pr", pr.Number, pr.Title)
			return m, tea.Batch(m.spinner.Tick, createPRWorktreeCmd(m.worktreeMgr, m.projectRoot, name, pr.Branch))
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

func renderPRView(s *PRViewState, width int, spinnerView string) string {
	if s.Loading {
		return Theme.OverlayBorder.Render(
			Theme.OverlayTitle.Render("Pull Requests") + "\n\n" +
				spinnerView + " Loading PRs...",
		)
	}

	if s.Creating {
		return Theme.OverlayBorder.Render(
			Theme.OverlayTitle.Render("Pull Requests") + "\n\n" +
				spinnerView + " Creating worktree from PR...",
		)
	}

	var b strings.Builder

	if s.Error != "" {
		b.WriteString(Theme.ErrorText.Render(s.Error) + "\n\n")
	}

	if s.Filter != "" {
		fmt.Fprintf(&b, "Filter: %s█\n\n", s.Filter)
	}

	filtered := filteredPRs(s.PRs, s.Filter)
	if len(filtered) == 0 {
		b.WriteString(Theme.DetailDim.Render("  (no matching PRs)") + "\n")
	} else {
		maxShow := 15
		start := 0
		if s.Cursor >= maxShow {
			start = s.Cursor - maxShow + 1
		}
		end := start + maxShow
		if end > len(filtered) {
			end = len(filtered)
		}
		for i := start; i < end; i++ {
			pr := filtered[i]
			cursor := "  "
			if i == s.Cursor {
				cursor = Theme.ListCursor.String()
			}

			number := Theme.DetailDim.Render(fmt.Sprintf("#%-5d", pr.Number))
			title := truncate(pr.Title, 50)
			branch := Theme.DetailDim.Render(truncate(pr.Branch, 20))
			author := Theme.DetailDim.Render("@" + pr.Author)

			badge := ""
			if s.WorktreeBranches[pr.Branch] {
				badge = " " + Theme.SuccessText.Render("[worktree]")
			}

			b.WriteString(fmt.Sprintf("%s%s %s  %s  %s%s\n", cursor, number, title, branch, author, badge))
		}
		if end < len(filtered) {
			b.WriteString(Theme.DetailDim.Render(fmt.Sprintf("  … and %d more", len(filtered)-end)) + "\n")
		}
	}

	b.WriteString("\n" + Theme.Footer.Render("[enter] create worktree  [esc] close  type to filter"))

	return Theme.OverlayBorder.Render(
		Theme.OverlayTitle.Render("Pull Requests") + "\n\n" + b.String(),
	)
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

func createPRWorktreeCmd(mgr *worktree.Manager, projectRoot, name, branch string) tea.Cmd {
	return func() tea.Msg {
		err := mgr.CreateFromBranch(name, branch)
		if err != nil {
			return prWorktreeCreatedMsg{name: name, err: err}
		}
		wt, err := mgr.Find(name)
		if err != nil || wt == nil {
			return prWorktreeCreatedMsg{name: name, path: projectRoot}
		}
		return prWorktreeCreatedMsg{name: name, path: wt.Path}
	}
}
