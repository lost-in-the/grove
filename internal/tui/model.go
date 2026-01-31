package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/git"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tuilog"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

// ActiveView tracks which view is currently displayed.
type ActiveView int

const (
	ViewDashboard ActiveView = iota
	ViewHelp
	ViewDelete
	ViewCreate
	ViewBulk
	ViewPRs
	ViewIssues
)

// Model is the root Bubble Tea model.
type Model struct {
	// Data
	worktreeMgr *worktree.Manager
	stateMgr    *state.Manager
	projectRoot string
	projectName string
	cfg         *config.Config

	// Child components (bubbles)
	list    list.Model
	detail  viewport.Model
	spinner spinner.Model
	help    help.Model

	// Keys
	keys KeyMap

	// State
	activeView ActiveView
	ready      bool // true after first WindowSizeMsg
	loading    bool // true while fetching worktrees
	statusMsg  string
	statusTTL  time.Time

	// Sort
	sortMode SortMode

	// Overlay state
	deleteState *DeleteState
	createState *CreateState
	bulkState   *BulkState
	prState     *PRViewState
	issueState  *IssueViewState

	// Output
	switchTo string
	err      error

	// Layout
	width, height int
}

// NewModel creates a new TUI model.
func NewModel(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string) Model {
	cfg, _ := config.Load()
	keys := DefaultKeyMap()

	s := spinner.New()
	s.Spinner = spinner.Dot

	delegate := NewWorktreeDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.KeyMap.Filter.SetKeys("/")
	// Map our navigation keys to list's keymap
	l.KeyMap.CursorUp.SetKeys("up", "k")
	l.KeyMap.CursorDown.SetKeys("down", "j")

	h := help.New()

	return Model{
		worktreeMgr: mgr,
		stateMgr:    stateMgr,
		projectRoot: projectRoot,
		projectName: mgr.GetProjectName(),
		cfg:         cfg,
		keys:        keys,
		list:        l,
		spinner:     s,
		help:        h,
		activeView:  ViewDashboard,
		loading:     true,
	}
}

// SwitchTo returns the path the user selected to switch to, if any.
func (m Model) SwitchTo() string { return m.switchTo }

// Err returns any error that occurred.
func (m Model) Err() error { return m.err }

// --- Tea interface ---

func (m Model) Init() tea.Cmd {
	tuilog.Printf("Init: loading=%v ready=%v", m.loading, m.ready)
	return tea.Batch(m.spinner.Tick, m.fetchWorktrees)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.updateLayout()
		tuilog.Printf("WindowSizeMsg: %dx%d, list size: %dx%d", msg.Width, msg.Height, m.list.Width(), m.list.Height())
		return m, nil

	case worktreesFetchedMsg:
		if msg.err != nil {
			tuilog.Printf("worktreesFetchedMsg: error=%v", msg.err)
			m.err = msg.err
			return m, tea.Quit
		}
		tuilog.Printf("worktreesFetchedMsg: %d items, ready=%v", len(msg.items), m.ready)
		listItems := make([]list.Item, len(msg.items))
		for i, item := range msg.items {
			listItems[i] = item
		}
		if m.sortMode != SortByName {
			listItems = sortWorktreeItems(listItems, m.sortMode)
		}
		m.list.SetItems(listItems)
		m.loading = false
		m.updateDetailContent()
		return m, nil

	case worktreeDeletedMsg:
		m.activeView = ViewDashboard
		m.deleteState = nil
		if msg.err == nil {
			m.statusMsg = fmt.Sprintf("Deleted %q", msg.name)
			m.statusTTL = time.Now().Add(3 * time.Second)
			cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return statusClearMsg{deadline: m.statusTTL}
			}))
		}
		cmds = append(cmds, m.fetchWorktrees)
		return m, tea.Batch(cmds...)

	case worktreeCreatedMsg:
		if msg.err != nil {
			if m.createState != nil {
				m.createState.Creating = false
				m.createState.Error = msg.err.Error()
			}
			return m, nil
		}
		m.activeView = ViewDashboard
		m.createState = nil
		m.statusMsg = fmt.Sprintf("Created %q", msg.name)
		m.statusTTL = time.Now().Add(3 * time.Second)
		cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return statusClearMsg{deadline: m.statusTTL}
		}))
		cmds = append(cmds, m.fetchWorktrees)
		return m, tea.Batch(cmds...)

	case statusClearMsg:
		if !msg.deadline.Before(m.statusTTL) {
			m.statusMsg = ""
		}
		return m, nil

	case prsFetchedMsg:
		if m.prState != nil {
			m.prState.Loading = false
			if msg.err != nil {
				m.prState.Error = msg.err.Error()
			} else {
				m.prState.PRs = msg.prs
			}
		}
		return m, nil

	case issuesFetchedMsg:
		if m.issueState != nil {
			m.issueState.Loading = false
			if msg.err != nil {
				m.issueState.Error = msg.err.Error()
			} else {
				m.issueState.Issues = msg.issues
			}
		}
		return m, nil

	case issueWorktreeCreatedMsg:
		if m.issueState != nil {
			m.issueState.Creating = false
		}
		if msg.err != nil {
			if m.issueState != nil {
				m.issueState.Error = msg.err.Error()
			}
			return m, nil
		}
		m.activeView = ViewDashboard
		m.issueState = nil
		m.statusMsg = fmt.Sprintf("Created worktree from issue %q", msg.name)
		m.statusTTL = time.Now().Add(3 * time.Second)
		cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return statusClearMsg{deadline: m.statusTTL}
		}))
		cmds = append(cmds, m.fetchWorktrees)
		return m, tea.Batch(cmds...)

	case prWorktreeCreatedMsg:
		if m.prState != nil {
			m.prState.Creating = false
		}
		if msg.err != nil {
			if m.prState != nil {
				m.prState.Error = msg.err.Error()
			}
			return m, nil
		}
		m.activeView = ViewDashboard
		m.prState = nil
		m.statusMsg = fmt.Sprintf("Created worktree from PR %q", msg.name)
		m.statusTTL = time.Now().Add(3 * time.Second)
		cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return statusClearMsg{deadline: m.statusTTL}
		}))
		cmds = append(cmds, m.fetchWorktrees)
		return m, tea.Batch(cmds...)

	case bulkDeleteDoneMsg:
		m.activeView = ViewDashboard
		m.bulkState = nil
		m.statusMsg = fmt.Sprintf("Deleted %d worktrees", msg.count)
		m.statusTTL = time.Now().Add(3 * time.Second)
		return m, tea.Batch(
			tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return statusClearMsg{deadline: m.statusTTL}
			}),
			m.fetchWorktrees,
		)

	case spinner.TickMsg:
		if m.loading || (m.createState != nil && m.createState.Creating) || (m.prState != nil && (m.prState.Loading || m.prState.Creating)) || (m.issueState != nil && (m.issueState.Loading || m.issueState.Creating)) {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		tuilog.Printf("KeyMsg: type=%d string=%q runes=%v", msg.Type, msg.String(), msg.Runes)
		// Overlays capture all input
		if m.activeView != ViewDashboard {
			return m.handleKey(msg)
		}

		// If list is filtering, let it handle all keys except esc
		if m.list.FilterState() == list.Filtering {
			prevIdx := m.list.Index()
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			if m.list.Index() != prevIdx {
				m.updateDetailContent()
			}
			return m, cmd
		}

		return m.handleKey(msg)

	default:
		// Forward unhandled messages to the list so it can process
		// internal messages (e.g. filter match results from fuzzy search).
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
}

func (m *Model) updateLayout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	headerHeight := 1 // status bar
	footerHeight := 1 // help
	available := m.height - headerHeight - footerHeight

	useSideBySide := m.width > 120
	if useSideBySide {
		// Adaptive list width: based on content, clamped to [40, 55% of terminal]
		listWidth := m.adaptiveListWidth()
		maxListWidth := m.width * 55 / 100
		if listWidth > maxListWidth {
			listWidth = maxListWidth
		}
		detailWidth := m.width - listWidth - 1
		m.list.SetSize(listWidth, available)
		m.detail.Width = detailWidth
		m.detail.Height = available
	} else {
		// Stacked: cap list height at item count * row height + padding
		itemCount := len(m.list.Items())
		rowHeight := 1 // delegate Height()
		idealListHeight := itemCount*rowHeight + 2 // +2 for padding
		maxListHeight := available * 6 / 10
		listHeight := idealListHeight
		if listHeight > maxListHeight {
			listHeight = maxListHeight
		}
		if listHeight < 3 {
			listHeight = 3
		}
		detailHeight := available - listHeight - 1
		if detailHeight < 3 {
			detailHeight = 3
		}
		m.list.SetSize(m.width, listHeight)
		m.detail.Width = m.width
		m.detail.Height = detailHeight
	}

	m.help.Width = m.width
}

// adaptiveListWidth calculates list panel width based on the widest rendered row.
func (m *Model) adaptiveListWidth() int {
	minWidth := 40
	maxRendered := minWidth
	for _, li := range m.list.Items() {
		item, ok := li.(WorktreeItem)
		if !ok {
			continue
		}
		// Approximate row width: num(2) + cursor(2) + name + gap(2) + branch + gap(2) + age + gap(2) + status(8) + tmux(12)
		w := 2 + 2 + len(item.ShortName) + 2 + len(item.Branch) + 2 + 8 + 2 + 8
		if item.TmuxStatus != "none" {
			w += 12
		}
		if w > maxRendered {
			maxRendered = w
		}
	}
	return maxRendered
}

func (m *Model) updateDetailContent() {
	item, ok := m.selectedItem()
	if !ok {
		m.detail.SetContent("")
		return
	}
	content := renderDetailContent(&item, m.detail.Width)
	m.detail.SetContent(content)
	m.detail.GotoTop()
}

// existingWorktreeItems returns all current worktree items from the list.
func (m Model) existingWorktreeItems() []WorktreeItem {
	items := m.list.Items()
	result := make([]WorktreeItem, 0, len(items))
	for _, li := range items {
		if item, ok := li.(WorktreeItem); ok {
			result = append(result, item)
		}
	}
	return result
}

func (m Model) selectedItem() (WorktreeItem, bool) {
	selected := m.list.SelectedItem()
	if selected == nil {
		return WorktreeItem{}, false
	}
	item, ok := selected.(WorktreeItem)
	return item, ok
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
	case ViewBulk:
		return m.handleBulkKey(msg)
	case ViewPRs:
		return m.handlePRKey(msg)
	case ViewIssues:
		return m.handleIssueKey(msg)
	case ViewDashboard:
		return m.handleDashboardKey(msg)
	}
	return m, nil
}

func (m Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Escape):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.activeView = ViewHelp
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)

	case key.Matches(msg, m.keys.New):
		m.activeView = ViewCreate
		m.createState = &CreateState{
			Step:        CreateStepName,
			ProjectName: m.projectName,
		}
		return m, nil

	case key.Matches(msg, m.keys.Delete):
		item, ok := m.selectedItem()
		if ok && !item.IsMain && !item.IsProtected {
			m.activeView = ViewDelete
			m.deleteState = &DeleteState{
				Item:     &item,
				Warnings: gatherDeleteWarnings(&item),
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		item, ok := m.selectedItem()
		if ok {
			if item.IsCurrent {
				return m, tea.Quit
			}
			m.switchTo = item.Path
			return m, tea.Quit
		}

	case key.Matches(msg, m.keys.Sort):
		m.sortMode = m.sortMode.Next()
		m.applySortToList()
		return m, nil

	case key.Matches(msg, m.keys.All):
		return m.enterBulkMode()

	case key.Matches(msg, m.keys.PRs):
		return m.enterPRView()

	case key.Matches(msg, m.keys.Issues):
		return m.enterIssueView()
	}

	// Quick-switch: number keys 1-9 jump to nth visible item
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		r := msg.Runes[0]
		if r >= '1' && r <= '9' {
			idx := int(r - '1')
			items := m.list.Items()
			if idx < len(items) {
				item := items[idx].(WorktreeItem)
				if item.IsCurrent {
					return m, tea.Quit
				}
				m.switchTo = item.Path
				return m, tea.Quit
			}
			return m, nil
		}
	}

	// Route to list
	prevIdx := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.list.Index() != prevIdx {
		m.updateDetailContent()
	}
	return m, cmd
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
			// If duplicate exists, switch to it
			if s.ExistingWorktree != nil {
				m.switchTo = s.ExistingWorktree.Path
				m.activeView = ViewDashboard
				m.createState = nil
				return m, tea.Quit
			}
			s.Error = ""
			s.Step = CreateStepBranch
			return m, nil

		case msg.Type == tea.KeyBackspace:
			if len(s.Name) > 0 {
				s.Name = s.Name[:len(s.Name)-1]
				s.Error = ""
				s.ExistingWorktree = checkDuplicateWorktree(s.Name, m.existingWorktreeItems())
			}
			return m, nil

		case msg.Type == tea.KeyRunes:
			s.Name += string(msg.Runes)
			if errMsg := ValidateWorktreeName(s.Name); errMsg != "" {
				s.Error = errMsg
			} else {
				s.Error = ""
			}
			s.ExistingWorktree = checkDuplicateWorktree(s.Name, m.existingWorktreeItems())
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
			return m.startCreate(s.Name, "")
		}

	case CreateStepPickBranch:
		filtered := filteredBranches(s.Branches, s.BranchFilter)
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.activeView = ViewDashboard
			m.createState = nil
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
				if m.cfg != nil && m.cfg.TUI.SkipBranchNotice != nil && *m.cfg.TUI.SkipBranchNotice {
					action := m.cfg.TUI.DefaultBranchAction
					if action == "fork" {
						return m.startCreate(s.Name, "")
					}
					return m.startCreate(s.Name, s.BaseBranch)
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
				return m.startCreate(s.Name, s.BaseBranch)
			}
			return m.startCreate(s.Name, "")
		}
	}

	return m, nil
}

func (m *Model) startCreate(name, baseBranch string) (tea.Model, tea.Cmd) {
	m.createState.Creating = true
	return m, tea.Batch(m.spinner.Tick, createWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, name, baseBranch))
}

func (m *Model) applySortToList() {
	sorted := sortWorktreeItems(m.list.Items(), m.sortMode)
	m.list.SetItems(sorted)
	m.updateDetailContent()
}

func (m Model) enterPRView() (tea.Model, tea.Cmd) {
	// Collect existing worktree branches for badge display
	branches := make(map[string]bool)
	for _, li := range m.list.Items() {
		item := li.(WorktreeItem)
		branches[item.Branch] = true
	}

	m.activeView = ViewPRs
	m.prState = &PRViewState{
		Loading:          true,
		WorktreeBranches: branches,
	}
	return m, tea.Batch(m.spinner.Tick, m.fetchPRsCmd)
}

func (m Model) enterIssueView() (tea.Model, tea.Cmd) {
	m.activeView = ViewIssues
	m.issueState = &IssueViewState{
		Loading: true,
	}
	return m, tea.Batch(m.spinner.Tick, m.fetchIssuesCmd)
}

func (m Model) enterBulkMode() (tea.Model, tea.Cmd) {
	// Collect deletable (non-main, non-protected) worktrees
	var deletable []WorktreeItem
	for _, li := range m.list.Items() {
		item := li.(WorktreeItem)
		if !item.IsMain && !item.IsProtected && !item.IsCurrent {
			deletable = append(deletable, item)
		}
	}

	m.activeView = ViewBulk
	m.bulkState = &BulkState{
		Items:    deletable,
		Selected: make([]bool, len(deletable)),
	}
	return m, nil
}

func (m Model) handleBulkKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.bulkState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	s := m.bulkState

	if s.Deleting {
		// Ignore input while deleting
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Escape):
		m.activeView = ViewDashboard
		m.bulkState = nil
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if s.Cursor > 0 {
			s.Cursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if s.Cursor < len(s.Items)-1 {
			s.Cursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Toggle):
		if s.Cursor < len(s.Items) {
			s.Selected[s.Cursor] = !s.Selected[s.Cursor]
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		selected := s.SelectedItems()
		if len(selected) == 0 {
			return m, nil
		}
		s.Deleting = true
		s.Progress = fmt.Sprintf("Deleting %d worktrees...", len(selected))
		return m, m.bulkDeleteCmd(selected)
	}

	return m, nil
}

func (m Model) bulkDeleteCmd(items []WorktreeItem) tea.Cmd {
	return func() tea.Msg {
		for _, item := range items {
			// Reuse existing delete logic inline (without branch deletion for bulk)
			deleteWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, item.ShortName, false)()
		}
		return bulkDeleteDoneMsg{count: len(items)}
	}
}

func (m Model) View() string {
	if !m.ready {
		tuilog.Printf("View: not ready")
		return "loading..."
	}

	if m.loading {
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.spinner.View()+" Loading worktrees...",
		)
	}

	switch m.activeView {
	case ViewHelp:
		return m.renderHelp()

	case ViewDelete:
		if m.deleteState != nil {
			overlay := renderDeleteV2(m.deleteState, m.width)
			bg := m.renderDashboard()
			return centerOverlay(bg, overlay, m.width, m.height)
		}

	case ViewCreate:
		if m.createState != nil {
			overlay := renderCreateV2(m.createState, m.width, m.spinner.View())
			bg := m.renderDashboard()
			return centerOverlay(bg, overlay, m.width, m.height)
		}

	case ViewBulk:
		if m.bulkState != nil {
			overlay := renderBulk(m.bulkState, m.width)
			bg := m.renderDashboard()
			return centerOverlay(bg, overlay, m.width, m.height)
		}

	case ViewPRs:
		if m.prState != nil {
			overlay := renderPRViewV2(m.prState, m.width, m.spinner.View())
			bg := m.renderDashboard()
			return centerOverlay(bg, overlay, m.width, m.height)
		}

	case ViewIssues:
		if m.issueState != nil {
			overlay := renderIssueView(m.issueState, m.width, m.spinner.View())
			bg := m.renderDashboard()
			return centerOverlay(bg, overlay, m.width, m.height)
		}
	}

	return m.renderDashboard()
}

func (m Model) renderDashboard() string {
	statusBar := m.renderStatusBar()

	useSideBySide := m.width > 120

	var body string
	if useSideBySide {
		listView := m.list.View()
		detailView := m.renderDetailPanel()
		body = lipgloss.JoinHorizontal(lipgloss.Top, listView, " ", detailView)
	} else {
		listView := m.list.View()
		separator := Theme.DetailDim.Render(strings.Repeat("─", m.width))
		detailView := m.renderDetailPanel()
		body = listView + "\n" + separator + "\n" + detailView
	}

	footer := m.help.View(m.keys)

	return statusBar + "\n" + body + "\n" + footer
}

func (m Model) renderStatusBar() string {
	parts := []string{
		Theme.Header.Render(" " + m.projectName),
		Theme.DetailDim.Render(fmt.Sprintf(" %d worktrees", len(m.list.Items()))),
	}

	if m.sortMode != SortByName {
		parts = append(parts, Theme.DetailDim.Render("↕ "+m.sortMode.String()))
	}

	if m.statusMsg != "" {
		parts = append(parts, " "+Theme.SuccessText.Render("✓ "+m.statusMsg))
	}

	return strings.Join(parts, "  ")
}

func (m Model) renderDetailPanel() string {
	return m.detail.View()
}

func (m Model) renderHelp() string {
	cols := []struct {
		header string
		items  [][2]string
	}{
		{
			header: "Navigation",
			items: [][2]string{
				{"j/k ↑/↓", "move"},
				{"enter", "switch to worktree"},
				{"esc", "back / close"},
			},
		},
		{
			header: "Actions",
			items: [][2]string{
				{"n", "new worktree"},
				{"d", "delete worktree"},
				{"p", "browse PRs"},
				{"a", "bulk delete merged"},
				{"o", "cycle sort mode"},
				{"r", "refresh list"},
			},
		},
		{
			header: "Views",
			items: [][2]string{
				{"1-9", "quick-switch"},
				{"/", "filter / search"},
				{"?", "this help"},
				{"q", "quit"},
			},
		},
	}

	var sections []string
	for _, col := range cols {
		var lines []string
		lines = append(lines, Theme.DetailTitle.Render(col.header))
		for _, item := range col.items {
			k := Theme.HelpKey.Render(padRight(item[0], 12))
			desc := Theme.HelpDesc.Render(item[1])
			lines = append(lines, "  "+k+desc)
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	body := strings.Join(sections, "\n\n")
	body += "\n\n" + Theme.DetailDim.Render("Guided flows: follow on-screen prompts.")
	body += "\n" + Theme.DetailDim.Render("Backspace goes back. Esc cancels.")
	body += "\n\n" + Theme.Footer.Render("[any key to close]")

	return Theme.OverlayBorder.Render(
		Theme.OverlayTitle.Render("Keybindings") + "\n\n" + body,
	)
}

func centerOverlay(bg, overlay string, width, height int) string {
	// Split background and overlay into lines
	bgLines := strings.Split(bg, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Pad/trim bg to exactly height lines
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}
	if len(bgLines) > height {
		bgLines = bgLines[:height]
	}

	// Dim the background lines
	dimStyle := lipgloss.NewStyle().Faint(true)
	for i, line := range bgLines {
		bgLines[i] = dimStyle.Render(line)
	}

	// Calculate overlay position (centered)
	overlayHeight := len(overlayLines)
	overlayWidth := 0
	for _, line := range overlayLines {
		if w := lipgloss.Width(line); w > overlayWidth {
			overlayWidth = w
		}
	}

	startRow := (height - overlayHeight) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (width - overlayWidth) / 2
	if startCol < 0 {
		startCol = 0
	}

	// Composite overlay on top of dimmed background
	padding := strings.Repeat(" ", startCol)
	for i, oLine := range overlayLines {
		row := startRow + i
		if row >= 0 && row < len(bgLines) {
			bgLines[row] = padding + oLine
		}
	}

	return strings.Join(bgLines, "\n")
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
	tuilog.Init()
	defer tuilog.Close()

	tuilog.Printf("Run: projectRoot=%s", projectRoot)

	model := NewModel(mgr, stateMgr, projectRoot)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		tuilog.Printf("Run: tea.Program error: %v", err)
		return "", fmt.Errorf("TUI error: %w", err)
	}

	m := finalModel.(Model)
	tuilog.Printf("Run: exit ready=%v loading=%v items=%d switchTo=%q err=%v",
		m.ready, m.loading, len(m.list.Items()), m.switchTo, m.err)

	if m.Err() != nil {
		return "", m.Err()
	}

	switchPath := m.SwitchTo()
	if switchPath != "" {
		if cdFile := os.Getenv("GROVE_CD_FILE"); cdFile != "" {
			os.WriteFile(cdFile, []byte(switchPath), 0600)
		} else if os.Getenv("GROVE_SHELL") == "1" {
			fmt.Printf("cd:%s\n", switchPath)
		}
	}

	return switchPath, nil
}
