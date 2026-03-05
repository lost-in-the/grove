package tui

import (
	"fmt"
	"image/color"
	"os"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/git"
	"github.com/LeahArmstrong/grove-cli/internal/plugins"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
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
	pluginMgr   *plugins.Manager
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
	pendingSelect string

	// Output
	switchTo            string
	switchToDisplayName string // display name for tmux session naming
	err                 error

	// Layout
	width, height int
	compactMode   bool // true = V1 single-line delegate, false = V2 two-line

	// List delegate (stored for header rendering in compact mode)
	listDelegate WorktreeDelegate
}

// NewModel creates a new TUI model.
func NewModel(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string, pluginMgr ...*plugins.Manager) Model {
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		tuilog.Printf("warning: failed to load config: %v", cfgErr)
	}
	keys := DefaultKeyMap()

	s := GroveSpinner()

	// Determine compact mode from config
	compact := cfg != nil && cfg.TUI.CompactList != nil && *cfg.TUI.CompactList

	var delegate list.ItemDelegate
	var v1Delegate WorktreeDelegate
	if compact {
		v1Delegate = NewWorktreeDelegate()
		delegate = v1Delegate
	} else {
		delegate = NewWorktreeDelegateV2()
	}

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

	var pm *plugins.Manager
	if len(pluginMgr) > 0 {
		pm = pluginMgr[0]
	}

	return Model{
		worktreeMgr:  mgr,
		stateMgr:     stateMgr,
		pluginMgr:    pm,
		projectRoot:  projectRoot,
		projectName:  mgr.GetProjectName(),
		cfg:          cfg,
		cfgLoadErr:   cfgErr,
		keys:         keys,
		list:         l,
		listDelegate: v1Delegate,
		compactMode:  compact,
		spinner:      s,
		help:         h,
		toast:        NewToastModel(),
		helpFooter:   NewHelpFooter(),
		activeView:   ViewDashboard,
		loading:      true,
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
		m.computeColumnWidths()
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
		}

		m.updateDetailContent()
		return m, nil

	case worktreeDeletedMsg:
		m.activeView = ViewDashboard
		m.deleteState = nil
		if msg.err != nil {
			m.toast.Show(NewToast(fmt.Sprintf("Delete failed: %s", msg.err), ToastError))
		} else if msg.branchErr != nil {
			m.toast.Show(NewToast(fmt.Sprintf("Deleted %q but %s", msg.name, msg.branchErr), ToastWarning))
			cmds = append(cmds, m.spinner.Tick)
		} else {
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
			m.toast.Show(NewToast(fmt.Sprintf("Deleted %d, failed: %s", msg.count, strings.Join(names, ", ")), ToastWarning))
		} else {
			m.toast.Show(NewToast(fmt.Sprintf("Deleted %d worktrees", msg.count), ToastSuccess))
		}
		return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)

	case forkWIPCheckMsg:
		if m.forkState != nil {
			m.forkState.HasWIP = msg.hasWIP
			m.forkState.WIPFiles = msg.files
			if msg.err != nil {
				m.forkState.Err = msg.err
				// Don't skip WIP step on error — we don't know the real state
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

	case tea.KeyPressMsg:
		tuilog.Printf("KeyMsg: code=%d string=%q text=%q", msg.Code, msg.String(), msg.Text)
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
	footerHeight := m.helpFooter.CompactHeight(m.activeView, m.width-4)
	available := m.height - headerHeight - footerHeight

	contentWidth := m.width - 2 // body padding (1 char each side)

	// Column headers + separator are only shown in compact mode (2 lines).
	listHeaderHeight := 0
	if m.compactMode {
		listHeaderHeight = 2
	}

	useSideBySide := m.width > 100
	if useSideBySide {
		// List gets 60% of content width, detail gets the remainder
		listWidth := contentWidth * 60 / 100
		dividerWidth := 3 // " │ "
		detailWidth := contentWidth - listWidth - dividerWidth
		m.list.SetSize(listWidth, available-listHeaderHeight)
		m.detail.SetWidth(detailWidth)
		m.detail.SetHeight(available)
	} else {
		// Stacked: cap list height at item count * row height + padding
		itemCount := len(m.list.Items())
		rowHeight := m.delegateHeight()
		idealListHeight := itemCount*rowHeight + 2 // +2 for padding
		maxListHeight := (available - listHeaderHeight) * 75 / 100
		listHeight := idealListHeight
		if listHeight > maxListHeight {
			listHeight = maxListHeight
		}
		if listHeight < 3 {
			listHeight = 3
		}
		detailHeight := available - listHeaderHeight - listHeight - 1
		if detailHeight < 3 {
			detailHeight = 3
		}
		m.list.SetSize(contentWidth, listHeight)
		m.detail.SetWidth(contentWidth)
		m.detail.SetHeight(detailHeight)
	}

	m.computeColumnWidths()

	m.help.SetWidth(contentWidth)
}

// toggleCompactMode switches between V1 (compact) and V2 (two-line) delegates.
func (m *Model) toggleCompactMode() {
	m.compactMode = !m.compactMode
	if m.compactMode {
		d := ComputeDelegateWidths(m.list.Items(), m.list.Width())
		m.listDelegate = d
		m.list.SetDelegate(d)
	} else {
		d := ComputeDelegateWidthsV2(m.list.Items(), m.list.Width())
		m.list.SetDelegate(d)
	}
	m.updateLayout()
	m.updateDetailContent()
}

// delegateHeight returns the item height for the active delegate.
func (m *Model) delegateHeight() int {
	if m.compactMode {
		return 1
	}
	return 2
}

// computeColumnWidths scans list items to find max name/branch lengths,
// then distributes available width proportionally.
func (m *Model) computeColumnWidths() {
	listWidth := m.list.Width()
	if listWidth <= 0 {
		return
	}

	if m.compactMode {
		d := ComputeDelegateWidths(m.list.Items(), listWidth)
		m.listDelegate = d
		m.list.SetDelegate(d)
	} else {
		d := ComputeDelegateWidthsV2(m.list.Items(), listWidth)
		m.list.SetDelegate(d)
	}
}

func (m *Model) updateDetailContent() {
	item, ok := m.selectedItem()
	if !ok {
		m.detail.SetContent("")
		return
	}
	content := renderDetailV2(&item, m.detail.Width())
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

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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

func (m Model) handleDashboardKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Escape):
		if m.helpFooter.Expanded {
			m.helpFooter.Toggle()
			return m, nil
		}
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.helpFooter.Toggle()
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)

	case key.Matches(msg, m.keys.New):
		m.activeView = ViewCreate
		branches, branchErr := git.ListLocalBranches(m.projectRoot)
		if branchErr != nil {
			tuilog.Printf("warning: failed to list branches: %v", branchErr)
		}
		m.createState = &CreateState{
			Step:              CreateStepBranch,
			ProjectName:       m.projectName,
			Branches:          branches,
			BranchFilterInput: newBranchFilterInput(),
		}
		return m, m.createState.BranchFilterInput.Focus()

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
			m.switchToDisplayName = item.displayName()
			return m, tea.Quit
		}

	case key.Matches(msg, m.keys.ViewMode):
		m.toggleCompactMode()
		return m, nil

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
	runes := []rune(msg.Text)
	if len(runes) == 1 && m.list.FilterState() == list.Unfiltered {
		r := runes[0]
		if r >= '1' && r <= '9' {
			idx := int(r - '1')
			items := m.list.Items()
			if idx < len(items) {
				if item, ok := items[idx].(WorktreeItem); ok {
					if item.IsCurrent {
						return m, tea.Quit
					}
					m.switchTo = item.Path
					m.switchToDisplayName = item.displayName()
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

func (m Model) handleDeleteKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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

func (m Model) handleCreateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.createState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	switch m.createState.Step {
	case CreateStepBranch:
		return m.handleBranchSelectorKey(msg)
	case CreateStepBranchAction:
		return m.handleBranchActionKey(msg)
	case CreateStepName:
		return m.handleNameKey(msg)
	case CreateStepConfirm:
		return m.handleConfirmKey(msg)
	}

	return m, nil
}

// handleBranchActionKey handles the branch action step (split vs fork).
func (m Model) handleBranchActionKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.createState
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.activeView = ViewDashboard
		m.createState = nil
		return m, nil

	case key.Matches(msg, m.keys.Back):
		s.Step = CreateStepBranch
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
		// Initialize name input and proceed to Name step
		s.NameInput = newNameInput(s.NameSuggestion)
		s.Step = CreateStepName
		return m, s.NameInput.Focus()
	}
	return m, nil
}

// handleConfirmKey handles the confirmation step key input.
func (m Model) handleConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.createState
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.activeView = ViewDashboard
		m.createState = nil
		return m, nil

	case key.Matches(msg, m.keys.Back):
		s.Step = CreateStepName
		return m, s.NameInput.Focus()

	case key.Matches(msg, m.keys.Enter):
		return m.startCreate(s.Name, s.BaseBranch)
	}
	return m, nil
}

func (m *Model) startCreate(name, baseBranch string) (tea.Model, tea.Cmd) {
	if m.createState.Creating {
		return m, nil
	}
	m.createState.Error = ""
	m.createState.Creating = true
	return m, tea.Batch(m.spinner.Tick, createWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, name, baseBranch))
}

// handleBranchSelectorKey handles the unified branch selector step.
func (m Model) handleBranchSelectorKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.createState

	filter := s.BranchFilterInput.Value()
	filtered := filteredBranches(s.Branches, filter)
	showCreateNew := filter != "" && !exactBranchMatch(s.Branches, filter)
	totalItems := len(filtered)
	if showCreateNew {
		totalItems++
	}

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
		if totalItems > 0 && s.BranchCursor < totalItems-1 {
			s.BranchCursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		if totalItems == 0 {
			return m, nil
		}
		if s.BranchCursor < len(filtered) {
			// Selected an existing branch
			selected := filtered[s.BranchCursor]
			s.BaseBranch = selected
			s.NewBranchName = ""
			// Derive name suggestion
			strategy := ""
			if m.cfg != nil {
				strategy = m.cfg.TUI.WorktreeNameFromBranch
			}
			s.NameSuggestion = worktree.DeriveWorktreeName(selected, strategy)

			// Initialize name input with placeholder
			s.NameInput = newNameInput(s.NameSuggestion)

			// Check if branch action should be skipped
			if m.cfg != nil && m.cfg.TUI.SkipBranchNotice != nil && *m.cfg.TUI.SkipBranchNotice {
				action := m.cfg.TUI.DefaultBranchAction
				if action == "fork" {
					s.BaseBranch = ""
				}
				s.Step = CreateStepName
				return m, s.NameInput.Focus()
			}

			s.ActionChoice = 0
			s.DontShowAgain = false
			s.Step = CreateStepBranchAction
			return m, nil
		} else if showCreateNew {
			// Selected "Create new branch"
			s.BaseBranch = ""
			s.NewBranchName = filter
			s.NameSuggestion = filter

			// Initialize name input with placeholder
			s.NameInput = newNameInput(s.NameSuggestion)

			s.Step = CreateStepName
			return m, s.NameInput.Focus()
		}
		return m, nil

	default:
		// Route remaining keys through the filter textinput
		prevVal := s.BranchFilterInput.Value()
		var cmd tea.Cmd
		s.BranchFilterInput, cmd = s.BranchFilterInput.Update(msg)
		if s.BranchFilterInput.Value() != prevVal {
			s.BranchCursor = 0
		}
		return m, cmd
	}
}

// handleNameKey handles the name step key input.
func (m Model) handleNameKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.createState

	switch {
	case key.Matches(msg, m.keys.Escape):
		m.activeView = ViewDashboard
		m.createState = nil
		return m, nil

	case key.Matches(msg, m.keys.Back):
		// Only go back if the name input is empty
		if s.NameInput.Value() == "" {
			s.Step = CreateStepBranch
			return m, s.BranchFilterInput.Focus()
		}
		// Otherwise let textinput handle backspace
		prevVal := s.NameInput.Value()
		var cmd tea.Cmd
		s.NameInput, cmd = s.NameInput.Update(msg)
		if s.NameInput.Value() != prevVal {
			s.Name = s.NameInput.Value()
			s.Error = ""
			s.ExistingWorktree = checkDuplicateWorktree(s.Name, m.existingWorktreeItems())
		}
		return m, cmd

	case key.Matches(msg, m.keys.Enter):
		effectiveName := s.NameInput.Value()
		if effectiveName == "" && s.NameSuggestion != "" {
			effectiveName = s.NameSuggestion
			s.NameInput.SetValue(effectiveName)
		}
		s.Name = effectiveName
		if effectiveName == "" {
			s.Error = "name cannot be empty"
			return m, nil
		}
		if errMsg := ValidateWorktreeName(effectiveName); errMsg != "" {
			s.Error = errMsg
			return m, nil
		}
		// If duplicate exists, switch to it
		if s.ExistingWorktree != nil {
			m.switchTo = s.ExistingWorktree.Path
			m.switchToDisplayName = s.ExistingWorktree.displayName()
			m.activeView = ViewDashboard
			m.createState = nil
			return m, tea.Quit
		}
		s.Error = ""
		s.Step = CreateStepConfirm
		return m, nil

	default:
		// Route remaining keys through the name textinput
		prevVal := s.NameInput.Value()
		var cmd tea.Cmd
		s.NameInput, cmd = s.NameInput.Update(msg)
		if s.NameInput.Value() != prevVal {
			s.Name = s.NameInput.Value()
			if errMsg := ValidateWorktreeName(s.Name); errMsg != "" {
				s.Error = errMsg
			} else {
				s.Error = ""
			}
			s.ExistingWorktree = checkDuplicateWorktree(s.Name, m.existingWorktreeItems())
		}
		return m, cmd
	}
}

func (m *Model) applySortToList() {
	sorted := sortWorktreeItems(m.list.Items(), m.sortMode)
	m.list.SetItems(sorted)
	m.computeColumnWidths()
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
		FilterInput:      newPRFilterInput(),
	}
	return m, tea.Batch(m.spinner.Tick, m.fetchPRsCmd, m.prState.FilterInput.Focus())
}

func (m Model) enterIssueView() (tea.Model, tea.Cmd) {
	m.activeView = ViewIssues
	m.issueState = &IssueViewState{
		Loading:     true,
		FilterInput: newIssueFilterInput(),
	}
	return m, tea.Batch(m.spinner.Tick, m.fetchIssuesCmd, m.issueState.FilterInput.Focus())
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

func (m Model) handleBulkKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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

func (m Model) View() tea.View {
	v := tea.NewView(m.viewContent())
	v.AltScreen = true
	return v
}

func (m Model) viewContent() string {
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
		content := lipgloss.JoinVertical(lipgloss.Center, brand, "", msg, hint)
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
	// Find current worktree info and container target for header
	for _, li := range m.list.Items() {
		item, ok := li.(WorktreeItem)
		if !ok {
			continue
		}
		if item.IsCurrent {
			m.header.CurrentBranch = item.Branch
			m.header.CurrentName = item.ShortName
		}
		for _, ps := range item.PluginStatuses {
			if strings.Contains(ps.Detail, "pointed") {
				m.header.ContainerTarget = item.ShortName
			}
		}
	}
	statusBar := m.header.View(m.width)

	useSideBySide := m.width > 100
	bodyWidth := m.width - 2 // 1-char padding each side

	var body string
	if useSideBySide {
		// Force list view to its allocated width so JoinHorizontal
		// measures it correctly (list lines may be shorter than the panel).
		listWidth := m.list.Width()
		listContent := m.list.View()
		if m.compactMode {
			header := renderListHeader(m.listDelegate, listWidth)
			listContent = lipgloss.JoinVertical(lipgloss.Left, header, listContent)
		}
		listContent = lipgloss.NewStyle().Width(listWidth).Render(listContent)
		dividerHeight := m.list.Height()
		if m.compactMode {
			dividerHeight += 2
		}
		divider := renderVerticalDivider(dividerHeight, Colors.SurfaceDim)
		detailView := m.renderDetailPanel()
		body = lipgloss.JoinHorizontal(lipgloss.Top, listContent, divider, detailView)
	} else {
		listView := m.list.View()
		if m.compactMode {
			header := renderListHeader(m.listDelegate, bodyWidth)
			listView = lipgloss.JoinVertical(lipgloss.Left, header, listView)
		}
		// Named separator showing selected worktree
		separator := renderNamedSeparator(m.selectedItemName(), bodyWidth)
		detailView := m.renderDetailPanel()
		body = lipgloss.JoinVertical(lipgloss.Left, listView, separator, detailView)
	}

	// Help footer: always show compact hints
	footer := m.helpFooter.RenderCompact(m.activeView, m.width-4)

	// Composite toast onto the header line (right-aligned) to avoid layout shift
	if m.toast != nil && m.toast.Current != nil {
		toastView := m.toast.View(m.width)
		if toastView != "" {
			statusBar = compositeToastOnHeader(statusBar, toastView, m.width)
		}
	}

	// Wrap body in 1-char horizontal padding for visual framing
	body = lipgloss.NewStyle().Padding(0, 1).Render(body)

	dashboard := lipgloss.JoinVertical(lipgloss.Left, statusBar, body, footer)

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
func renderVerticalDivider(height int, color color.Color) string {
	style := lipgloss.NewStyle().Foreground(color).Padding(0, 1)
	lines := make([]string, height)
	for i := range lines {
		lines[i] = style.Render("│")
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
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

	if msg := m.toast.Message(); msg != "" {
		parts = append(parts, " "+Styles.StatusSuccess.Render("✓ "+msg))
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
func RunPRs(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string, pluginMgr ...*plugins.Manager) (string, error) {
	tuilog.Init()
	defer tuilog.Close()

	model := NewModel(mgr, stateMgr, projectRoot, pluginMgr...).ConfigureForPRs()
	return runModel(model)
}

// RunIssues starts the TUI directly in the issue browser view.
func RunIssues(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string, pluginMgr ...*plugins.Manager) (string, error) {
	tuilog.Init()
	defer tuilog.Close()

	model := NewModel(mgr, stateMgr, projectRoot, pluginMgr...).ConfigureForIssues()
	return runModel(model)
}

// Run starts the TUI and returns the path to switch to (if any).
func Run(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string, pluginMgr ...*plugins.Manager) (string, error) {
	tuilog.Init()
	defer tuilog.Close()

	model := NewModel(mgr, stateMgr, projectRoot, pluginMgr...)
	return runModel(model)
}

// handleTmuxSwitch creates/switches to the tmux session for the target worktree.
// Returns true if a tmux session switch happened (caller should skip cd).
func (m *Model) handleTmuxSwitch(switchPath string) bool {
	tmuxMode := "auto"
	if m.cfg != nil && m.cfg.Tmux.Mode != "" {
		tmuxMode = m.cfg.Tmux.Mode
	}

	if tmuxMode == "off" || !tmux.IsTmuxAvailable() || m.switchToDisplayName == "" {
		return false
	}

	sessionName := worktree.TmuxSessionName(m.projectName, m.switchToDisplayName)
	tuilog.Printf("handleTmuxSwitch: session=%q displayName=%q", sessionName, m.switchToDisplayName)

	// Store current session as last before switching
	if tmux.IsInsideTmux() {
		if currentSession, err := tmux.GetCurrentSession(); err == nil {
			_ = tmux.StoreLastSession(currentSession)
		}
	}

	// Create session if it doesn't exist
	exists, err := tmux.SessionExists(sessionName)
	if err != nil {
		tuilog.Printf("warning: failed to check tmux session %q: %v", sessionName, err)
		return false
	}
	if !exists {
		if err := tmux.CreateSession(sessionName, switchPath); err != nil {
			tuilog.Printf("warning: failed to create tmux session %q: %v", sessionName, err)
			return false
		}
	}

	// Switch or attach
	if tmux.IsInsideTmux() {
		if err := tmux.SwitchSession(sessionName); err != nil {
			tuilog.Printf("warning: failed to switch tmux session: %v", err)
			return false
		}
		return true
	}

	if err := tmux.AttachSession(sessionName); err != nil {
		tuilog.Printf("warning: failed to attach tmux session: %v", err)
		return false
	}
	return true
}

func runModel(model Model) (string, error) {
	tuilog.Printf("runModel: projectRoot=%s activeView=%d", model.projectRoot, model.activeView)

	p := tea.NewProgram(model)

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
		// Handle tmux session switching first — if it succeeds, skip cd
		// to avoid changing the OLD session's directory
		tmuxSwitched := m.handleTmuxSwitch(switchPath)

		if !tmuxSwitched {
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
	}

	return switchPath, nil
}
