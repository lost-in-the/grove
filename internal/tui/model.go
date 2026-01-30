package tui

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/git"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

// ActiveView tracks which view is currently displayed.
type ActiveView int

const (
	ViewDashboard ActiveView = iota
	ViewHelp
	ViewDelete
	ViewCreate
)

// Model is the root Bubble Tea model.
type Model struct {
	worktreeMgr *worktree.Manager
	stateMgr    *state.Manager
	projectRoot string
	projectName string
	items       []WorktreeItem
	keys        KeyMap

	activeView ActiveView
	cursor     int
	width      int
	height     int

	filtering  bool
	filterText string

	deleteState *DeleteState
	createState *CreateState

	cfg *config.Config

	switchTo string
	err      error
}

// NewModel creates a new TUI model.
func NewModel(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string) Model {
	cfg, _ := config.Load()
	return Model{
		worktreeMgr: mgr,
		stateMgr:    stateMgr,
		projectRoot: projectRoot,
		projectName: mgr.GetProjectName(),
		keys:        DefaultKeyMap(),
		activeView:  ViewDashboard,
		cfg:         cfg,
	}
}

// SwitchTo returns the path the user selected to switch to, if any.
func (m Model) SwitchTo() string { return m.switchTo }

// Err returns any error that occurred.
func (m Model) Err() error { return m.err }

// --- Messages ---

type worktreesFetchedMsg struct {
	items []WorktreeItem
	err   error
}

type worktreeDeletedMsg struct {
	name         string
	deleteBranch bool
	err          error
}

type worktreeCreatedMsg struct {
	name string
	path string
	err  error
}

// --- Commands ---

func (m Model) fetchWorktrees() tea.Msg {
	items, err := FetchWorktrees(m.worktreeMgr, m.stateMgr)
	return worktreesFetchedMsg{items: items, err: err}
}

func deleteWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot, name string, deleteBranch bool) tea.Cmd {
	return func() tea.Msg {
		projectName := mgr.GetProjectName()

		// Kill tmux session before removing worktree
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			if exists, _ := tmux.SessionExists(sessionName); exists {
				_ = tmux.KillSession(sessionName)
			}
		}

		// Capture the branch before removal so we can delete it afterwards
		var branch string
		wt, _ := mgr.Find(name)
		if wt != nil {
			branch = wt.Branch
		}

		// Run pre-remove hooks
		hookExecutor, hookErr := hooks.NewExecutor()
		if hookErr == nil && hookExecutor.HasHooksForEvent(hooks.EventPreRemove) {
			hookCtx := &hooks.ExecutionContext{
				Event:    hooks.EventPreRemove,
				Worktree: name,
				Project:  projectName,
			}
			if wt != nil {
				hookCtx.Branch = wt.Branch
				hookCtx.NewPath = wt.Path
				hookCtx.WorktreeFull = projectName + "-" + name
			}
			_ = hookExecutor.Execute(hooks.EventPreRemove, hookCtx)
		}

		err := mgr.Remove(name)
		if err != nil {
			return worktreeDeletedMsg{name: name, deleteBranch: deleteBranch, err: err}
		}

		// Remove from state
		_ = stateMgr.RemoveWorktree(name)

		// Delete branch if requested
		if deleteBranch && branch != "" {
			branchMgr, branchErr := git.NewBranchManager(projectRoot)
			if branchErr == nil {
				_ = branchMgr.Delete(branch, false)
			}
		}

		return worktreeDeletedMsg{name: name, deleteBranch: deleteBranch, err: nil}
	}
}

func createWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot, name, baseBranch string) tea.Cmd {
	return func() tea.Msg {
		branchArg := name
		if baseBranch != "" {
			branchArg = baseBranch
		}
		err := mgr.Create(name, branchArg)
		if err != nil {
			return worktreeCreatedMsg{name: name, err: err}
		}
		wt, err := mgr.Find(name)
		if err != nil || wt == nil {
			return worktreeCreatedMsg{name: name, err: fmt.Errorf("worktree created but not found")}
		}

		projectName := mgr.GetProjectName()

		// Register in state (matches grove new behavior)
		now := time.Now()
		wsState := &state.WorktreeState{
			Path:           wt.Path,
			Branch:         name,
			CreatedAt:      now,
			LastAccessedAt: now,
		}
		_ = stateMgr.AddWorktree(name, wsState)

		// Create tmux session
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			_ = tmux.CreateSession(sessionName, wt.Path)
		}

		// Run post-create hooks
		hookExecutor, hookErr := hooks.NewExecutor()
		if hookErr == nil && hookExecutor.HasHooksForEvent(hooks.EventPostCreate) {
			hookCtx := &hooks.ExecutionContext{
				Event:        hooks.EventPostCreate,
				Worktree:     name,
				WorktreeFull: projectName + "-" + name,
				Branch:       name,
				Project:      projectName,
				MainPath:     projectRoot,
				NewPath:      wt.Path,
			}
			_ = hookExecutor.Execute(hooks.EventPostCreate, hookCtx)
		}

		return worktreeCreatedMsg{name: name, path: wt.Path}
	}
}

// --- Tea interface ---

func (m Model) Init() tea.Cmd {
	return m.fetchWorktrees
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case worktreesFetchedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.items = msg.items
		return m, nil

	case worktreeDeletedMsg:
		m.activeView = ViewDashboard
		m.deleteState = nil
		return m, m.fetchWorktrees

	case worktreeCreatedMsg:
		if msg.err != nil {
			if m.createState != nil {
				m.createState.Error = msg.err.Error()
			}
			return m, nil
		}
		m.activeView = ViewDashboard
		m.createState = nil
		return m, m.fetchWorktrees

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.activeView {
	case ViewHelp:
		m.activeView = ViewDashboard
		return m, nil
	case ViewDelete:
		return m.handleDeleteKey(msg)
	case ViewCreate:
		return m.handleCreateKey(msg)
	case ViewDashboard:
		return m.handleDashboardKey(msg)
	}
	return m, nil
}

func (m Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.filtering = false
			m.filterText = ""
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			m.filtering = false
			return m, nil
		case msg.Type == tea.KeyBackspace:
			if len(m.filterText) > 0 {
				m.filterText = m.filterText[:len(m.filterText)-1]
			}
			return m, nil
		case msg.Type == tea.KeyRunes:
			m.filterText += string(msg.Runes)
			return m, nil
		}
		return m, nil
	}

	visible := m.visibleItems()

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Escape):
		if m.filterText != "" {
			m.filterText = ""
			return m, nil
		}
		return m, tea.Quit

	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(visible)-1 {
			m.cursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		if m.cursor >= 0 && m.cursor < len(visible) {
			m.switchTo = visible[m.cursor].Path
			return m, tea.Quit
		}

	case key.Matches(msg, m.keys.Filter):
		m.filtering = true
		m.filterText = ""
		return m, nil

	case key.Matches(msg, m.keys.Help):
		m.activeView = ViewHelp
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		return m, m.fetchWorktrees

	case key.Matches(msg, m.keys.New):
		m.activeView = ViewCreate
		m.createState = &CreateState{
			Step:        CreateStepName,
			ProjectName: m.projectName,
		}
		return m, nil

	case key.Matches(msg, m.keys.Delete):
		if m.cursor >= 0 && m.cursor < len(visible) {
			item := &visible[m.cursor]
			if item.IsMain || item.IsProtected {
				return m, nil
			}
			m.activeView = ViewDelete
			m.deleteState = &DeleteState{
				Item:     item,
				Warnings: gatherDeleteWarnings(item),
			}
			return m, nil
		}
	}

	return m, nil
}

func (m Model) handleDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.deleteState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Confirm):
		name := m.deleteState.Item.ShortName
		return m, deleteWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, name, m.deleteState.DeleteBranch)

	case key.Matches(msg, m.keys.Escape), msg.String() == "n":
		m.activeView = ViewDashboard
		m.deleteState = nil
		return m, nil

	case key.Matches(msg, m.keys.Toggle):
		m.deleteState.DeleteBranch = !m.deleteState.DeleteBranch
		return m, nil
	}

	return m, nil
}

func (m Model) handleCreateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.createState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	s := m.createState

	switch s.Step {
	case CreateStepName:
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.activeView = ViewDashboard
			m.createState = nil
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			if s.Name == "" {
				s.Error = "name cannot be empty"
				return m, nil
			}
			if errMsg := ValidateWorktreeName(s.Name); errMsg != "" {
				s.Error = errMsg
				return m, nil
			}
			s.Error = ""
			s.Step = CreateStepBranch
			return m, nil

		case msg.Type == tea.KeyBackspace:
			if len(s.Name) > 0 {
				s.Name = s.Name[:len(s.Name)-1]
				s.Error = ""
			}
			return m, nil

		case msg.Type == tea.KeyRunes:
			s.Name += string(msg.Runes)
			if errMsg := ValidateWorktreeName(s.Name); errMsg != "" {
				s.Error = errMsg
			} else {
				s.Error = ""
			}
			return m, nil
		}

	case CreateStepBranch:
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.activeView = ViewDashboard
			m.createState = nil
			return m, nil

		case key.Matches(msg, m.keys.Back):
			s.Step = CreateStepName
			return m, nil

		case key.Matches(msg, m.keys.Up):
			if s.BranchChoice > 0 {
				s.BranchChoice--
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if s.BranchChoice < BranchFromExisting {
				s.BranchChoice++
			}
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			if s.BranchChoice == BranchFromExisting {
				branches, err := git.ListLocalBranches(m.projectRoot)
				if err != nil {
					s.Error = fmt.Sprintf("failed to list branches: %v", err)
					return m, nil
				}
				s.Branches = branches
				s.BranchCursor = 0
				s.BranchFilter = ""
				s.Step = CreateStepPickBranch
				return m, nil
			}
			return m, createWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, s.Name, "")
		}

	case CreateStepPickBranch:
		filtered := filteredBranches(s.Branches, s.BranchFilter)
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.activeView = ViewDashboard
			m.createState = nil
			return m, nil

		case key.Matches(msg, m.keys.Back):
			s.Step = CreateStepBranch
			s.BranchFilter = ""
			return m, nil

		case key.Matches(msg, m.keys.Up):
			if s.BranchCursor > 0 {
				s.BranchCursor--
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if s.BranchCursor < len(filtered)-1 {
				s.BranchCursor++
			}
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			if len(filtered) > 0 && s.BranchCursor < len(filtered) {
				s.BaseBranch = filtered[s.BranchCursor]
				// Check if we should skip the action notice
				if m.cfg != nil && m.cfg.TUI.SkipBranchNotice != nil && *m.cfg.TUI.SkipBranchNotice {
					action := m.cfg.TUI.DefaultBranchAction
					if action == "fork" {
						return m, createWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, s.Name, "")
					}
					// default to split
					return m, createWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, s.Name, s.BaseBranch)
				}
				s.ActionChoice = 0
				s.DontShowAgain = false
				s.Step = CreateStepBranchAction
			}
			return m, nil

		case msg.Type == tea.KeyBackspace:
			if len(s.BranchFilter) > 0 {
				s.BranchFilter = s.BranchFilter[:len(s.BranchFilter)-1]
				s.BranchCursor = 0
			} else {
				s.Step = CreateStepBranch
			}
			return m, nil

		case msg.Type == tea.KeyRunes:
			s.BranchFilter += string(msg.Runes)
			s.BranchCursor = 0
			return m, nil
		}

	case CreateStepBranchAction:
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.activeView = ViewDashboard
			m.createState = nil
			return m, nil

		case key.Matches(msg, m.keys.Back):
			s.Step = CreateStepPickBranch
			return m, nil

		case key.Matches(msg, m.keys.Up):
			if s.ActionChoice > 0 {
				s.ActionChoice--
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if s.ActionChoice < 1 {
				s.ActionChoice++
			}
			return m, nil

		case key.Matches(msg, m.keys.Toggle):
			s.DontShowAgain = !s.DontShowAgain
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			// Persist "don't show again" preference
			if s.DontShowAgain && m.cfg != nil {
				action := "fork"
				if s.ActionChoice == 0 {
					action = "split"
				}
				_ = config.SetProjectConfigValues(map[string]string{
					"tui.skip_branch_notice":    "true",
					"tui.default_branch_action": `"` + action + `"`,
				})
			}

			if s.ActionChoice == 0 {
				// Split: use existing branch as-is
				return m, createWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, s.Name, s.BaseBranch)
			}
			// Fork: create new branch (name-based) from current HEAD
			return m, createWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, s.Name, "")
		}
	}

	return m, nil
}

func (m Model) visibleItems() []WorktreeItem {
	if m.filterText == "" {
		return m.items
	}
	return filterItems(m.items, m.filterText)
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}

	switch m.activeView {
	case ViewHelp:
		return renderHelp(m.width, m.height)

	case ViewDelete:
		if m.deleteState != nil {
			overlay := renderDelete(m.deleteState, m.width)
			bg := renderDashboard(m.items, m.cursor, m.filterText, m.filtering, m.width, m.height)
			return centerOverlay(bg, overlay, m.width, m.height)
		}

	case ViewCreate:
		if m.createState != nil {
			overlay := renderCreate(m.createState, m.width)
			bg := renderDashboard(m.items, m.cursor, m.filterText, m.filtering, m.width, m.height)
			return centerOverlay(bg, overlay, m.width, m.height)
		}
	}

	return renderDashboard(m.items, m.cursor, m.filterText, m.filtering, m.width, m.height)
}

func centerOverlay(_, overlay string, width, height int) string {
	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceChars(" "),
	)
}

func gatherDeleteWarnings(item *WorktreeItem) []string {
	var warnings []string
	if item.IsProtected {
		warnings = append(warnings, "This worktree is protected")
	}
	if item.IsDirty {
		warnings = append(warnings, "Working tree is dirty")
	}
	if item.IsEnvironment {
		warnings = append(warnings, "This is an environment worktree")
	}
	return warnings
}

// Run starts the TUI and returns the path to switch to (if any).
func Run(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string) (string, error) {
	model := NewModel(mgr, stateMgr, projectRoot)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("TUI error: %w", err)
	}

	m := finalModel.(Model)
	if m.Err() != nil {
		return "", m.Err()
	}

	switchPath := m.SwitchTo()
	if switchPath != "" && os.Getenv("GROVE_SHELL") == "1" {
		fmt.Printf("cd:%s\n", switchPath)
	}

	return switchPath, nil
}
