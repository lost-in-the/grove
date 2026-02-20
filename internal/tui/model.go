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
	"github.com/charmbracelet/huh"
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
	ViewFork
	ViewSync
	ViewConfig
)

// Model is the root Bubble Tea model.
type Model struct {
	// Data
	worktreeMgr *worktree.Manager
	stateMgr    *state.Manager
	projectRoot string
	projectName string
	cfg         *config.Config
	cfgLoadErr  error // non-nil if config failed to load at startup

	// Child components (bubbles)
	list    list.Model
	detail  viewport.Model
	spinner spinner.Model
	help    help.Model

	// V2 components
	header     Header
	toast      *ToastModel
	helpFooter *HelpFooter

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
	forkState   *ForkState
	syncState   *SyncState
	configState *ConfigState

	// Post-create selection
	pendingSelect     string
	pendingSelectPath string

	// Output
	switchTo string
	err      error

	// Layout
	width, height int
}

// NewModel creates a new TUI model.
func NewModel(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string) Model {
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		tuilog.Printf("warning: failed to load config: %v", cfgErr)
	}
	keys := DefaultKeyMap()

	s := GroveSpinner()

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
		cfgLoadErr:  cfgErr,
		keys:        keys,
		list:        l,
		spinner:     s,
		help:        h,
		toast:       NewToastModel(),
		helpFooter:  NewHelpFooter(),
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
	tuilog.Printf("Init: loading=%v ready=%v activeView=%d", m.loading, m.ready, m.activeView)
	cmds := []tea.Cmd{m.spinner.Tick, m.fetchWorktrees}
	if m.activeView == ViewPRs {
		cmds = append(cmds, m.fetchPRsCmd)
	}
	if m.activeView == ViewIssues {
		cmds = append(cmds, m.fetchIssuesCmd)
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		wasReady := m.ready
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.updateLayout()
		tuilog.Printf("WindowSizeMsg: %dx%d, list size: %dx%d", msg.Width, msg.Height, m.list.Width(), m.list.Height())
		if !wasReady && m.cfgLoadErr != nil {
			m.toast.Show(NewToast("Config load failed: "+m.cfgLoadErr.Error(), ToastWarning))
			m.cfgLoadErr = nil
		}
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

		// Select newly created worktree if pending (highlight it in the list)
		if m.pendingSelect != "" {
			for i, li := range listItems {
				if item, ok := li.(WorktreeItem); ok && item.ShortName == m.pendingSelect {
					m.list.Select(i)
					tuilog.Printf("pendingSelect: highlighted %q at index %d, path=%q", m.pendingSelect, i, item.Path)
					break
				}
			}
			m.pendingSelect = ""
			m.pendingSelectPath = ""
		}

		m.updateDetailContent()
		return m, nil

	case worktreeDeletedMsg:
		m.activeView = ViewDashboard
		m.deleteState = nil
		if msg.err != nil {
			m.toast.Show(NewToast(fmt.Sprintf("Delete failed: %s", msg.err), ToastError))
		} else if msg.branchErr != nil {
			m.statusMsg = fmt.Sprintf("Deleted %q (branch kept)", msg.name)
			m.statusTTL = time.Now().Add(3 * time.Second)
			m.toast.Show(NewToast(fmt.Sprintf("Deleted %q but %s", msg.name, msg.branchErr), ToastWarning))
			cmds = append(cmds, m.spinner.Tick)
		} else {
			m.statusMsg = fmt.Sprintf("Deleted %q", msg.name)
			m.statusTTL = time.Now().Add(3 * time.Second)
			m.toast.Show(NewToast(fmt.Sprintf("Deleted %q", msg.name), ToastSuccess))
			cmds = append(cmds, m.spinner.Tick)
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
		m.pendingSelect = msg.name
		m.pendingSelectPath = msg.path
		m.statusMsg = fmt.Sprintf("Created %q", msg.name)
		m.statusTTL = time.Now().Add(3 * time.Second)
		if msg.hookErr != nil {
			m.toast.Show(NewToast(fmt.Sprintf("Created %q (hook failed: %s)", msg.name, msg.hookErr), ToastWarning))
		} else {
			m.toast.Show(NewToast(fmt.Sprintf("Created %q", msg.name), ToastSuccess))
		}
		if msg.hookOutput != "" {
			tuilog.Printf("hook output for %q: %s", msg.name, msg.hookOutput)
		}
		cmds = append(cmds, m.spinner.Tick, m.fetchWorktrees)
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
		m.pendingSelect = msg.name
		m.pendingSelectPath = msg.path
		m.statusMsg = fmt.Sprintf("Created worktree from issue %q", msg.name)
		m.statusTTL = time.Now().Add(3 * time.Second)
		if msg.hookErr != nil {
			m.toast.Show(NewToast(fmt.Sprintf("Created from issue %q (hook failed: %s)", msg.name, msg.hookErr), ToastWarning))
		} else {
			m.toast.Show(NewToast(fmt.Sprintf("Created worktree from issue %q", msg.name), ToastSuccess))
		}
		if msg.hookOutput != "" {
			tuilog.Printf("hook output for issue worktree %q: %s", msg.name, msg.hookOutput)
		}
		cmds = append(cmds, m.spinner.Tick, m.fetchWorktrees)
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
		m.pendingSelect = msg.name
		m.pendingSelectPath = msg.path
		m.statusMsg = fmt.Sprintf("Created worktree from PR %q", msg.name)
		m.statusTTL = time.Now().Add(3 * time.Second)
		if msg.hookErr != nil {
			m.toast.Show(NewToast(fmt.Sprintf("Created from PR %q (hook failed: %s)", msg.name, msg.hookErr), ToastWarning))
		} else {
			m.toast.Show(NewToast(fmt.Sprintf("Created worktree from PR %q", msg.name), ToastSuccess))
		}
		if msg.hookOutput != "" {
			tuilog.Printf("hook output for PR worktree %q: %s", msg.name, msg.hookOutput)
		}
		cmds = append(cmds, m.spinner.Tick, m.fetchWorktrees)
		return m, tea.Batch(cmds...)

	case bulkDeleteDoneMsg:
		m.activeView = ViewDashboard
		m.bulkState = nil
		if len(msg.failed) > 0 {
			var names []string
			for name, errMsg := range msg.failed {
				names = append(names, name)
				tuilog.Printf("bulk delete failed for %q: %s", name, errMsg)
			}
			m.statusMsg = fmt.Sprintf("Deleted %d worktrees (%d failed: %s)", msg.count, len(msg.failed), strings.Join(names, ", "))
			m.toast.Show(NewToast(fmt.Sprintf("Deleted %d, failed: %s", msg.count, strings.Join(names, ", ")), ToastWarning))
		} else {
			m.statusMsg = fmt.Sprintf("Deleted %d worktrees", msg.count)
			m.toast.Show(NewToast(fmt.Sprintf("Deleted %d worktrees", msg.count), ToastSuccess))
		}
		m.statusTTL = time.Now().Add(3 * time.Second)
		return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)

	case forkWIPCheckMsg:
		if m.forkState != nil {
			m.forkState.HasWIP = msg.hasWIP
			m.forkState.WIPFiles = msg.files
			if msg.err != nil {
				m.forkState.Err = msg.err
				// Don't skip WIP step on error — we don't know the real state
			} else if !msg.hasWIP {
				// Skip WIP step if no WIP
				m.forkState.Step = ForkStepConfirm
				m.forkState.Stepper.Current = 2
			}
		}
		return m, nil

	case forkCompleteMsg:
		if msg.err != nil {
			if msg.name != "" {
				// Partial success: worktree created but WIP patch failed
				m.activeView = ViewDashboard
				m.forkState = nil
				m.pendingSelect = msg.name
				m.pendingSelectPath = msg.path
				m.toast.Show(NewToast(fmt.Sprintf("Forked %q (warning: %s)", msg.name, msg.err), ToastWarning))
				return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)
			}
			if m.forkState != nil {
				m.forkState.Err = msg.err
				m.forkState.Forking = false
			}
			return m, nil
		}
		m.activeView = ViewDashboard
		m.forkState = nil
		m.pendingSelect = msg.name
		m.pendingSelectPath = msg.path
		m.toast.Show(NewToast(fmt.Sprintf("Forked %q", msg.name), ToastSuccess))
		return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)

	case syncWIPInfoMsg:
		if m.syncState != nil {
			if msg.err != nil {
				m.syncState.Err = msg.err
			} else {
				m.syncState.Sources = msg.sources
			}
		}
		return m, nil

	case syncCompleteMsg:
		if msg.err != nil {
			if m.syncState != nil {
				m.syncState.Err = msg.err
				m.syncState.Syncing = false
			}
			return m, nil
		}
		m.activeView = ViewDashboard
		m.syncState = nil
		m.toast.Show(NewToast(fmt.Sprintf("Synced %d files", msg.filesApplied), ToastSuccess))
		return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)

	case configLoadedMsg:
		if m.configState != nil {
			if msg.err != nil {
				m.configState.Err = msg.err
			} else {
				m.configState.Config = msg.cfg
				m.configState.Fields = populateConfigFields(msg.cfg)
			}
		}
		return m, nil

	case configSavedMsg:
		if msg.err != nil {
			if m.configState != nil {
				m.configState.Err = msg.err
			} else {
				// Config overlay was already closed (save-on-close), show toast
				m.toast.Show(NewToast("Config save failed: "+msg.err.Error(), ToastError))
			}
			return m, nil
		}
		if m.configState != nil {
			// Config overlay still open — close it
			m.activeView = ViewDashboard
			m.configState = nil
		}
		m.toast.Show(NewToast("Configuration saved", ToastSuccess))
		// Reload config so in-memory state reflects saved changes
		if cfg, err := config.Load(); err != nil {
			tuilog.Printf("error: config reload after save failed: %v", err)
			m.toast.Show(NewToast("Saved but reload failed — restart TUI to apply", ToastWarning))
		} else {
			m.cfg = cfg
		}
		return m, nil

	case spinner.TickMsg:
		// Tick toast expiry on every spinner tick
		if m.toast != nil {
			m.toast.Tick()
		}
		var spinnerCmds []tea.Cmd
		if m.loading || (m.createState != nil && m.createState.Creating) || (m.forkState != nil && m.forkState.Forking) || (m.syncState != nil && m.syncState.Syncing) || (m.prState != nil && (m.prState.Loading || m.prState.Creating)) || (m.issueState != nil && (m.issueState.Loading || m.issueState.Creating)) || (m.toast != nil && m.toast.Current != nil) {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			spinnerCmds = append(spinnerCmds, cmd)
		}
		return m, tea.Batch(spinnerCmds...)

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
		// Forward messages to active Huh form when in create view
		if m.activeView == ViewCreate && m.createState != nil && m.createState.UseHuhForms {
			return m.forwardToActiveHuhForm(msg)
		}
		// Forward messages to active Huh form when in fork view
		if m.activeView == ViewFork && m.forkState != nil && m.forkState.Form != nil {
			return m.forwardToForkHuhForm(msg)
		}
		// Forward messages to active Huh form when in config view
		if m.activeView == ViewConfig && m.configState != nil && m.configState.Editing && m.configState.EditForm != nil {
			return m.forwardToConfigHuhForm(msg)
		}
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

	contentWidth := m.width - 2 // body padding (1 char each side)

	useSideBySide := m.width > 120
	if useSideBySide {
		// Adaptive list width: based on content, clamped to [40, 55% of terminal]
		listWidth := m.adaptiveListWidth()
		maxListWidth := contentWidth * 55 / 100
		if listWidth > maxListWidth {
			listWidth = maxListWidth
		}
		dividerWidth := 3 // " │ "
		detailWidth := contentWidth - listWidth - dividerWidth
		m.list.SetSize(listWidth, available)
		m.detail.Width = detailWidth
		m.detail.Height = available
	} else {
		// Stacked: cap list height at item count * row height + padding
		itemCount := len(m.list.Items())
		rowHeight := 2                             // delegate Height()
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
		m.list.SetSize(contentWidth, listHeight)
		m.detail.Width = contentWidth
		m.detail.Height = detailHeight
	}

	m.help.Width = contentWidth
}

// adaptiveListWidth calculates list panel width based on the widest rendered row.
func (m *Model) adaptiveListWidth() int {
	// Floor: 45% of terminal in side-by-side mode
	minWidth := m.width * 45 / 100
	if minWidth < 40 {
		minWidth = 40
	}
	maxRendered := minWidth
	for _, li := range m.list.Items() {
		item, ok := li.(WorktreeItem)
		if !ok {
			continue
		}
		// Match delegate Render(): num(2) + cursor(2) + name(padded) + gap(2) + branch(padded) + gap(2) + age(8) + gap(2) + status(8) + gap(2) + tmux(12)
		nameWidth := 16
		branchWidth := 12
		if m.width > 100 {
			nameWidth = 24
			branchWidth = 20
		} else if m.width > 80 {
			nameWidth = 20
			branchWidth = 16
		}
		w := 2 + 2 + nameWidth + 2 + branchWidth + 2 + 8 + 2 + 8
		if item.TmuxStatus != "none" {
			w += 2 + 12
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
	content := renderDetailV2(&item, m.detail.Width)
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
	case ViewFork:
		return m.handleForkKey(msg)
	case ViewSync:
		return m.handleSyncKey(msg)
	case ViewConfig:
		return m.handleConfigKey(msg)
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
		m.helpFooter.Toggle()
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)

	case key.Matches(msg, m.keys.New):
		m.activeView = ViewCreate
		m.createState = &CreateState{
			Step:        CreateStepName,
			ProjectName: m.projectName,
			UseHuhForms: true,
		}
		if m.createState.UseHuhForms {
			m.createState.NameForm = NewCreateNameForm(
				&m.createState.Name,
				m.projectName,
				m.existingWorktreeItems(),
			)
			return m, m.createState.NameForm.Init()
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
			tuilog.Printf("Enter: item=%q path=%q isCurrent=%v", item.ShortName, item.Path, item.IsCurrent)
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

	case key.Matches(msg, m.keys.Fork):
		item, ok := m.selectedItem()
		if ok {
			m.activeView = ViewFork
			m.forkState = NewForkState(item)
			return m, checkWIPCmd(item)
		}
		return m, nil

	case key.Matches(msg, m.keys.Sync):
		m.activeView = ViewSync
		m.syncState = NewSyncState(m.existingWorktreeItems())
		return m, gatherWIPInfoCmd(m.existingWorktreeItems())

	case key.Matches(msg, m.keys.Config):
		m.activeView = ViewConfig
		m.configState = NewConfigState()
		return m, loadConfigCmd()
	}

	// Quick-switch: number keys 1-9 jump to nth visible item
	// Disabled when the list has an active filter to avoid switching to hidden items
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && m.list.FilterState() == list.Unfiltered {
		r := msg.Runes[0]
		if r >= '1' && r <= '9' {
			idx := int(r - '1')
			items := m.list.Items()
			if idx < len(items) {
				if item, ok := items[idx].(WorktreeItem); ok {
					if item.IsCurrent {
						return m, tea.Quit
					}
					m.switchTo = item.Path
					return m, tea.Quit
				}
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
		if m.deleteState.Item == nil {
			m.activeView = ViewDashboard
			m.deleteState = nil
			return m, nil
		}
		name := m.deleteState.Item.ShortName
		return m, deleteWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, name, m.deleteState.DeleteBranch)

	case key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.Deny):
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

	// Delegate to Huh forms when enabled
	if s.UseHuhForms {
		return m.handleCreateKeyHuh(msg)
	}

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
			s.BaseBranch = ""
			s.Step = CreateStepConfirm
			return m, nil
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
				if err := config.SetProjectConfigValues(map[string]string{
					"tui.skip_branch_notice":    "true",
					"tui.default_branch_action": `"` + action + `"`,
				}); err != nil {
					m.toast.Show(NewToast("Failed to save preference: "+err.Error(), ToastWarning))
				}
			}

			if s.ActionChoice == 1 {
				s.BaseBranch = ""
			}
			s.Step = CreateStepConfirm
			return m, nil
		}

	case CreateStepConfirm:
		return m.handleConfirmKey(msg)
	}

	return m, nil
}

// handleCreateKeyHuh handles create wizard input using Huh forms.
func (m Model) handleCreateKeyHuh(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.createState

	switch s.Step {
	case CreateStepName:
		if key.Matches(msg, m.keys.Escape) {
			m.activeView = ViewDashboard
			m.createState = nil
			return m, nil
		}
		if s.NameForm == nil {
			return m, nil
		}
		model, cmd := s.NameForm.Update(msg)
		s.NameForm = model.(*huh.Form)

		if s.NameForm.State == huh.StateAborted {
			m.activeView = ViewDashboard
			m.createState = nil
			return m, nil
		}
		return m.checkCreateFormCompletion(cmd)

	case CreateStepBranch:
		if key.Matches(msg, m.keys.Escape) {
			m.activeView = ViewDashboard
			m.createState = nil
			return m, nil
		}
		// Intercept backspace to go back to Name step before Huh consumes it
		if key.Matches(msg, m.keys.Back) {
			s.Step = CreateStepName
			s.NameForm = NewCreateNameForm(&s.Name, s.ProjectName, m.existingWorktreeItems())
			return m, s.NameForm.Init()
		}
		if s.BranchForm == nil {
			return m, nil
		}
		model, cmd := s.BranchForm.Update(msg)
		s.BranchForm = model.(*huh.Form)

		if s.BranchForm.State == huh.StateAborted {
			m.activeView = ViewDashboard
			m.createState = nil
			return m, nil
		}
		return m.checkCreateFormCompletion(cmd)

	case CreateStepPickBranch:
		if key.Matches(msg, m.keys.Escape) {
			m.activeView = ViewDashboard
			m.createState = nil
			return m, nil
		}
		// Intercept backspace to go back to Branch step before Huh consumes it
		if key.Matches(msg, m.keys.Back) {
			s.Step = CreateStepBranch
			s.BranchForm = NewCreateBranchForm(&s.BranchChoiceStr)
			return m, s.BranchForm.Init()
		}
		if s.BranchPickForm == nil {
			return m, nil
		}
		model, cmd := s.BranchPickForm.Update(msg)
		s.BranchPickForm = model.(*huh.Form)

		if s.BranchPickForm.State == huh.StateAborted {
			m.activeView = ViewDashboard
			m.createState = nil
			return m, nil
		}
		return m.checkCreateFormCompletion(cmd)

	case CreateStepBranchAction:
		return m.handleBranchActionKey(msg)

	case CreateStepConfirm:
		return m.handleConfirmKey(msg)
	}

	return m, nil
}

// forwardToActiveHuhForm forwards non-key messages to the active Huh form.
func (m Model) forwardToActiveHuhForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	s := m.createState
	var activeForm **huh.Form
	switch s.Step {
	case CreateStepName:
		activeForm = &s.NameForm
	case CreateStepBranch:
		activeForm = &s.BranchForm
	case CreateStepPickBranch:
		activeForm = &s.BranchPickForm
	default:
		return m, nil
	}
	if *activeForm == nil {
		return m, nil
	}
	model, cmd := (*activeForm).Update(msg)
	*activeForm = model.(*huh.Form)

	// Check if the async update completed the form (e.g. Enter key result)
	return m.checkCreateFormCompletion(cmd)
}

// checkCreateFormCompletion checks if the current Huh form has completed
// and transitions to the next step. Called after both key and non-key updates.
func (m Model) checkCreateFormCompletion(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	s := m.createState

	switch s.Step {
	case CreateStepName:
		if s.NameForm != nil && s.NameForm.State == huh.StateCompleted {
			s.Error = ""
			s.Step = CreateStepBranch
			s.BranchForm = NewCreateBranchForm(&s.BranchChoiceStr)
			return m, s.BranchForm.Init()
		}

	case CreateStepBranch:
		if s.BranchForm != nil && s.BranchForm.State == huh.StateCompleted {
			if s.BranchChoiceStr == "existing" {
				branches, err := git.ListLocalBranches(m.projectRoot)
				if err != nil {
					s.Error = fmt.Sprintf("failed to list branches: %v", err)
					return m, nil
				}
				s.Branches = branches
				s.BranchPickForm = NewBranchPickerForm(&s.SelectedBranch, branches)
				s.Step = CreateStepPickBranch
				return m, s.BranchPickForm.Init()
			}
			s.BaseBranch = ""
			s.Step = CreateStepConfirm
			return m, nil
		}

	case CreateStepPickBranch:
		if s.BranchPickForm != nil && s.BranchPickForm.State == huh.StateCompleted {
			s.BaseBranch = s.SelectedBranch
			if m.cfg != nil && m.cfg.TUI.SkipBranchNotice != nil && *m.cfg.TUI.SkipBranchNotice {
				action := m.cfg.TUI.DefaultBranchAction
				if action == "fork" {
					s.BaseBranch = ""
				}
				s.Step = CreateStepConfirm
				return m, nil
			}
			s.ActionChoice = 0
			s.DontShowAgain = false
			s.Step = CreateStepBranchAction
			return m, nil
		}
	}

	return m, cmd
}

// handleBranchActionKey handles the branch action step (split vs fork).
func (m Model) handleBranchActionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.createState
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.activeView = ViewDashboard
		m.createState = nil
		return m, nil

	case key.Matches(msg, m.keys.Back):
		s.Step = CreateStepPickBranch
		if s.BranchPickForm != nil {
			return m, s.BranchPickForm.Init()
		}
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
			if err := config.SetProjectConfigValues(map[string]string{
				"tui.skip_branch_notice":    "true",
				"tui.default_branch_action": `"` + action + `"`,
			}); err != nil {
				m.toast.Show(NewToast("Failed to save preference: "+err.Error(), ToastWarning))
			}
		}

		if s.ActionChoice == 1 {
			// fork: new branch, don't use existing
			s.BaseBranch = ""
		}
		s.Step = CreateStepConfirm
		return m, nil
	}
	return m, nil
}

// handleConfirmKey handles the confirmation step key input.
func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.createState
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.activeView = ViewDashboard
		m.createState = nil
		return m, nil

	case key.Matches(msg, m.keys.Back):
		s.Step = CreateStepBranch
		if s.UseHuhForms {
			s.BranchForm = NewCreateBranchForm(&s.BranchChoiceStr)
			return m, s.BranchForm.Init()
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		return m.startCreate(s.Name, s.BaseBranch)
	}
	return m, nil
}

func (m *Model) startCreate(name, baseBranch string) (tea.Model, tea.Cmd) {
	if m.createState.Creating {
		return m, nil
	}
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
		if item, ok := li.(WorktreeItem); ok {
			branches[item.Branch] = true
		}
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
		if item, ok := li.(WorktreeItem); ok {
			if !item.IsMain && !item.IsProtected && !item.IsCurrent {
				deletable = append(deletable, item)
			}
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
		if len(s.Items) > 0 && s.Cursor < len(s.Items)-1 {
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
		failed := make(map[string]string)
		for _, item := range items {
			result := deleteWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, item.ShortName, false)()
			if msg, ok := result.(worktreeDeletedMsg); ok && msg.err != nil {
				failed[item.ShortName] = msg.err.Error()
			}
		}
		return bulkDeleteDoneMsg{count: len(items) - len(failed), failed: failed}
	}
}

func (m Model) View() string {
	if !m.ready {
		tuilog.Printf("View: not ready")
		// For PR/Issue views launched directly, render a loading overlay
		// even before WindowSizeMsg arrives so alt-screen isn't blank.
		if m.activeView == ViewPRs && m.prState != nil {
			return m.spinner.View() + " Loading PRs..."
		}
		if m.activeView == ViewIssues && m.issueState != nil {
			return m.spinner.View() + " Loading issues..."
		}
		return "loading..."
	}

	if m.loading {
		brand := Styles.Header.Render("  grove")
		loading := m.spinner.View() + " " + Styles.TextMuted.Render("Loading worktrees...")
		content := brand + "\n\n" + loading
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			content,
		)
	}

	// Empty state: no worktrees after loading
	if len(m.list.Items()) == 0 {
		brand := Styles.Header.Render("  grove")
		msg := Styles.TextMuted.Render("No worktrees found")
		hint := Styles.HelpKey.Render("n") + " " + Styles.HelpDesc.Render("to create your first worktree")
		content := brand + "\n\n" + msg + "\n" + hint
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			content,
		)
	}

	switch m.activeView {
	case ViewHelp:
		// ViewHelp is unused (help is rendered via helpFooter overlay on dashboard).
		// Fall through to dashboard rendering.
		return m.renderDashboard()

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

	case ViewFork:
		if m.forkState != nil {
			overlay := renderFork(m.forkState, m.width)
			bg := m.renderDashboard()
			return centerOverlay(bg, overlay, m.width, m.height)
		}

	case ViewSync:
		if m.syncState != nil {
			overlay := renderSync(m.syncState, m.width)
			bg := m.renderDashboard()
			return centerOverlay(bg, overlay, m.width, m.height)
		}

	case ViewConfig:
		if m.configState != nil {
			overlay := renderConfig(m.configState, m.width)
			bg := m.renderDashboard()
			return centerOverlay(bg, overlay, m.width, m.height)
		}
	}

	return m.renderDashboard()
}

func (m Model) renderDashboard() string {
	// Build header from current state
	m.header = Header{
		ProjectName:   m.projectName,
		WorktreeCount: len(m.list.Items()),
	}
	// Find current worktree info for header
	for _, li := range m.list.Items() {
		if item, ok := li.(WorktreeItem); ok && item.IsCurrent {
			m.header.CurrentBranch = item.Branch
			m.header.CurrentName = item.ShortName
			break
		}
	}
	statusBar := m.header.View(m.width)

	useSideBySide := m.width > 120
	bodyWidth := m.width - 2 // 1-char padding each side

	var body string
	if useSideBySide {
		listView := m.list.View()
		divider := renderVerticalDivider(m.list.Height(), Colors.SurfaceDim)
		detailView := m.renderDetailPanel()
		body = lipgloss.JoinHorizontal(lipgloss.Top, listView, divider, detailView)
	} else {
		listView := m.list.View()
		// Named separator showing selected worktree
		separator := renderNamedSeparator(m.selectedItemName(), bodyWidth)
		detailView := m.renderDetailPanel()
		body = listView + "\n" + separator + "\n" + detailView
	}

	// Help footer: always show compact hints
	footer := m.helpFooter.RenderCompact(m.activeView, m.width)

	// Composite toast onto the header line (right-aligned) to avoid layout shift
	if m.toast != nil && m.toast.Current != nil {
		toastView := m.toast.View(m.width)
		if toastView != "" {
			statusBar = compositeToastOnHeader(statusBar, toastView, m.width)
		}
	}

	// Wrap body in 1-char horizontal padding for visual framing
	body = lipgloss.NewStyle().Padding(0, 1).Render(body)

	dashboard := statusBar + "\n" + body + "\n" + footer

	// Render expanded help as centered overlay
	if m.helpFooter.Expanded {
		helpOverlay := m.helpFooter.RenderExpanded(m.width)
		return centerOverlay(dashboard, helpOverlay, m.width, m.height)
	}

	return dashboard
}

// selectedItemName returns the short name of the currently selected worktree.
func (m Model) selectedItemName() string {
	item, ok := m.selectedItem()
	if !ok {
		return ""
	}
	return item.ShortName
}

// renderVerticalDivider creates a thin column of │ characters for side-by-side layout.
func renderVerticalDivider(height int, color lipgloss.AdaptiveColor) string {
	style := lipgloss.NewStyle().Foreground(color).Padding(0, 1)
	lines := make([]string, height)
	for i := range lines {
		lines[i] = style.Render("│")
	}
	return strings.Join(lines, "\n")
}

// renderNamedSeparator renders a horizontal rule with the selected worktree name embedded.
func renderNamedSeparator(name string, width int) string {
	if name == "" {
		return Styles.TextMuted.Render(strings.Repeat("─", width))
	}
	label := " " + name + " "
	labelWidth := lipgloss.Width(label)
	leftLen := 2
	rightLen := width - leftLen - labelWidth
	if rightLen < 0 {
		rightLen = 0
	}
	return Styles.TextMuted.Render(strings.Repeat("─", leftLen)) +
		Styles.TextMuted.Render(label) +
		Styles.TextMuted.Render(strings.Repeat("─", rightLen))
}

func (m Model) renderStatusBar() string {
	parts := []string{
		Styles.Header.Render(" " + m.projectName),
		Styles.TextMuted.Render(fmt.Sprintf(" %d worktrees", len(m.list.Items()))),
	}

	if m.sortMode != SortByName {
		parts = append(parts, Styles.TextMuted.Render("↕ "+m.sortMode.String()))
	}

	if m.statusMsg != "" {
		parts = append(parts, " "+Styles.StatusSuccess.Render("✓ "+m.statusMsg))
	}

	return strings.Join(parts, "  ")
}

func (m Model) renderDetailPanel() string {
	return m.detail.View()
}

// compositeToastOnHeader overlays a right-aligned toast onto the header line,
// avoiding layout shift in alt-screen mode.
func compositeToastOnHeader(header, toast string, width int) string {
	headerWidth := lipgloss.Width(header)
	toastWidth := lipgloss.Width(toast)

	// If both fit, place toast on the right side of the header line
	if headerWidth+toastWidth+2 <= width {
		gap := width - headerWidth - toastWidth
		return header + strings.Repeat(" ", gap) + toast
	}

	// If toast alone fits, it takes priority (temporary notification)
	if toastWidth < width {
		padding := width - toastWidth
		return strings.Repeat(" ", padding) + toast
	}

	// Fallback: just show header
	return header
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
		warnings = append(warnings, "worktree has uncommitted changes")
	}
	if item.IsEnvironment {
		warnings = append(warnings, "This is an environment worktree")
	}
	return warnings
}

// ConfigureForPRs configures the model to start directly in the PR browser view.
func (m Model) ConfigureForPRs() Model {
	branches := make(map[string]bool)
	for _, li := range m.list.Items() {
		if item, ok := li.(WorktreeItem); ok {
			branches[item.Branch] = true
		}
	}
	m.activeView = ViewPRs
	m.prState = &PRViewState{
		Loading:          true,
		WorktreeBranches: branches,
	}
	return m
}

// ConfigureForIssues configures the model to start directly in the issue browser view.
func (m Model) ConfigureForIssues() Model {
	m.activeView = ViewIssues
	m.issueState = &IssueViewState{
		Loading: true,
	}
	return m
}

// RunPRs starts the TUI directly in the PR browser view.
func RunPRs(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string) (string, error) {
	tuilog.Init()
	defer tuilog.Close()

	model := NewModel(mgr, stateMgr, projectRoot).ConfigureForPRs()
	return runModel(model)
}

// RunIssues starts the TUI directly in the issue browser view.
func RunIssues(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string) (string, error) {
	tuilog.Init()
	defer tuilog.Close()

	model := NewModel(mgr, stateMgr, projectRoot).ConfigureForIssues()
	return runModel(model)
}

// Run starts the TUI and returns the path to switch to (if any).
func Run(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string) (string, error) {
	tuilog.Init()
	defer tuilog.Close()

	model := NewModel(mgr, stateMgr, projectRoot)
	return runModel(model)
}

func runModel(model Model) (string, error) {
	tuilog.Printf("runModel: projectRoot=%s activeView=%d", model.projectRoot, model.activeView)

	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		tuilog.Printf("runModel: tea.Program error: %v", err)
		return "", fmt.Errorf("tui error: %w", err)
	}

	m := finalModel.(Model)
	tuilog.Printf("runModel: exit ready=%v loading=%v items=%d switchTo=%q err=%v",
		m.ready, m.loading, len(m.list.Items()), m.switchTo, m.err)

	if m.Err() != nil {
		return "", m.Err()
	}

	switchPath := m.SwitchTo()
	if switchPath != "" {
		if cdFile := os.Getenv("GROVE_CD_FILE"); cdFile != "" {
			tuilog.Printf("runModel: writing switchTo=%q to GROVE_CD_FILE=%q", switchPath, cdFile)
			if err := os.WriteFile(cdFile, []byte(switchPath), 0600); err != nil {
				return "", fmt.Errorf("failed to write cd file: %w", err)
			}
		} else if os.Getenv("GROVE_SHELL") == "1" {
			tuilog.Printf("runModel: printing cd directive for switchTo=%q", switchPath)
			// Leading newline ensures cd: directive is on its own line,
			// separated from any bubbletea alt-screen exit escape codes
			// that may precede it on stdout.
			fmt.Printf("\ncd:%s\n", switchPath)
		} else {
			tuilog.Printf("runModel: switchTo=%q but no GROVE_CD_FILE or GROVE_SHELL set — cannot switch", switchPath)
		}
	}

	return switchPath, nil
}
