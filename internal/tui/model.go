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

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/internal/plugins"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/tuilog"
	"github.com/lost-in-the/grove/internal/worktree"
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
	ViewRename
	ViewCheckout
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
	header      Header
	toast       *ToastModel
	helpFooter  *HelpFooter
	helpOverlay *HelpOverlay

	// Keys
	keys KeyMap

	// State
	activeView    ActiveView
	ready         bool // true after first WindowSizeMsg
	loading       bool // true while fetching worktrees
	detailFocused bool // true when dashboard detail panel has focus

	// Sort
	sortMode SortMode

	// Overlay state
	deleteState   *DeleteState
	createState   *CreateState
	bulkState     *BulkState
	prState       *PRViewState
	issueState    *IssueViewState
	forkState     *ForkState
	syncState     *SyncState
	configState   *ConfigState
	renameState   *RenameState
	checkoutState *CheckoutState

	// Post-create selection
	pendingSelect string

	// Output
	switchTo            string
	switchToDisplayName string // display name for tmux session naming
	switchForceUp       bool   // when true, signal CLI to start containers after switch
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

	// Determine compact mode: persisted preference > config > default (false)
	compact := cfg != nil && cfg.TUI.CompactList != nil && *cfg.TUI.CompactList
	if prefs := loadUIPrefs(projectRoot); prefs != nil && prefs.CompactMode != nil {
		compact = *prefs.CompactMode
	}

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
		helpOverlay:  NewHelpOverlay(),
		activeView:   ViewDashboard,
		loading:      true,
	}
}

// SwitchTo returns the path the user selected to switch to, if any.
func (m Model) SwitchTo() string { return m.switchTo }

// SwitchForceUp returns true if the CLI should start containers after switching.
func (m Model) SwitchForceUp() bool { return m.switchForceUp }

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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case worktreesFetchedMsg:
		return m.handleWorktreesFetched(msg)
	case prLookupMsg:
		return m.handlePRLookup(msg)
	case worktreeDeletedMsg:
		return m.handleWorktreeDeleted(msg)
	case creationLogMsg:
		return m.handleCreationLog(msg)
	case creationDoneMsg:
		return m.handleCreationDone(msg)
	case worktreeCreatedMsg:
		return m.handleWorktreeCreated(msg)
	case prsFetchedMsg:
		return m.handlePRsFetched(msg)
	case issuesFetchedMsg:
		return m.handleIssuesFetched(msg)
	case bulkDeleteDoneMsg:
		return m.handleBulkDeleteDone(msg)
	case wipCheckMsg:
		return m.handleWIPCheck(msg)
	case forkCompleteMsg:
		return m.handleForkComplete(msg)
	case syncWIPInfoMsg:
		return m.handleSyncWIPInfo(msg)
	case syncCompleteMsg:
		return m.handleSyncComplete(msg)
	case configLoadedMsg:
		return m.handleConfigLoaded(msg)
	case configSavedMsg:
		return m.handleConfigSaved(msg)
	case renameCompleteMsg:
		return m.handleRenameComplete(msg)
	case checkoutBranchesMsg:
		return m.handleCheckoutBranches(msg)
	case checkoutCompleteMsg:
		return m.handleCheckoutComplete(msg)
	case spinner.TickMsg:
		return m.handleSpinnerTick(msg)
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	default:
		return m.handleDefault(msg)
	}
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
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
	if m.helpOverlay.Active {
		m.helpOverlay.Open(m.helpOverlay.ForView, m.width, m.height)
	}
	return m, nil
}

func (m Model) handleWorktreesFetched(msg worktreesFetchedMsg) (tea.Model, tea.Cmd) {
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

	var branches []string
	for _, li := range listItems {
		if item, ok := li.(WorktreeItem); ok {
			branches = append(branches, item.Branch)
		}
	}
	return m, lookupPRsCmd(branches)
}

func (m Model) handlePRLookup(msg prLookupMsg) (tea.Model, tea.Cmd) {
	if msg.prs != nil {
		listItems := m.list.Items()
		for i, li := range listItems {
			if item, ok := li.(WorktreeItem); ok {
				if pr, found := msg.prs[item.Branch]; found {
					item.AssociatedPR = pr
					listItems[i] = item
				}
			}
		}
		m.list.SetItems(listItems)
		m.updateDetailContent()
	}
	return m, nil
}

func (m Model) handleWorktreeDeleted(msg worktreeDeletedMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
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
}

func (m Model) handleCreationLog(msg creationLogMsg) (tea.Model, tea.Cmd) {
	ct, _, _ := m.creationTrackerForSource(msg.source)
	if ct != nil {
		if log := ct.getActivityLog(); log != nil {
			log.AddLine(msg.line)
		}
	}
	return m, readCreationLog(msg.ch, msg.source)
}

func (m Model) handleCreationDone(msg creationDoneMsg) (tea.Model, tea.Cmd) {
	ct, label, cleanup := m.creationTrackerForSource(msg.source)
	if ct != nil {
		if log := ct.getActivityLog(); log != nil {
			log.SetDone(msg.err)
		}
		if msg.err != nil {
			ct.setCreatingDone(msg.err.Error())
			return m, nil
		}
		ct.setCreatingDone("")
	}
	if msg.err != nil {
		return m, nil
	}
	m.activeView = ViewDashboard
	if cleanup != nil {
		cleanup()
	}
	m.pendingSelect = msg.name
	if msg.hookErr != nil {
		m.toast.Show(NewToast(fmt.Sprintf("Created %s%q (hook failed: %s)", label, msg.name, msg.hookErr), ToastWarning))
	} else {
		m.toast.Show(NewToast(fmt.Sprintf("Created %s%q", label, msg.name), ToastSuccess))
	}
	if msg.hookOutput != "" {
		tuilog.Printf("hook output for %s%q: %s", label, msg.name, msg.hookOutput)
	}
	return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)
}

func (m Model) handleWorktreeCreated(msg worktreeCreatedMsg) (tea.Model, tea.Cmd) {
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
	return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)
}

func (m Model) handlePRsFetched(msg prsFetchedMsg) (tea.Model, tea.Cmd) {
	if m.prState != nil {
		m.prState.Loading = false
		if msg.err != nil {
			m.prState.Error = msg.err.Error()
		} else {
			m.prState.PRs = msg.prs
		}
	}
	return m, nil
}

func (m Model) handleIssuesFetched(msg issuesFetchedMsg) (tea.Model, tea.Cmd) {
	if m.issueState != nil {
		m.issueState.Loading = false
		if msg.err != nil {
			m.issueState.Error = msg.err.Error()
		} else {
			m.issueState.Issues = msg.issues
		}
	}
	return m, nil
}

func (m Model) handleBulkDeleteDone(msg bulkDeleteDoneMsg) (tea.Model, tea.Cmd) {
	m.activeView = ViewDashboard
	m.bulkState = nil
	if len(msg.failed) > 0 {
		names := make([]string, 0, len(msg.failed))
		for name, errMsg := range msg.failed {
			names = append(names, name)
			tuilog.Printf("bulk delete failed for %q: %s", name, errMsg)
		}
		m.toast.Show(NewToast(fmt.Sprintf("Deleted %d, failed: %s", msg.count, strings.Join(names, ", ")), ToastWarning))
	} else {
		m.toast.Show(NewToast(fmt.Sprintf("Deleted %d worktrees", msg.count), ToastSuccess))
	}
	return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)
}

func (m Model) handleWIPCheck(msg wipCheckMsg) (tea.Model, tea.Cmd) {
	if m.forkState != nil {
		m.forkState.HasWIP = msg.hasWIP
		m.forkState.WIPFiles = msg.files
		if msg.err != nil {
			m.forkState.Err = msg.err
		}
	}
	if m.checkoutState != nil {
		m.checkoutState.HasWIP = msg.hasWIP
		m.checkoutState.WIPCheckDone = true
		m.checkoutState.WIPFiles = msg.files
		if msg.err != nil {
			m.checkoutState.Err = msg.err
		}
	}
	return m, nil
}

func (m Model) handleForkComplete(msg forkCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		if msg.name != "" {
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
}

func (m Model) handleSyncWIPInfo(msg syncWIPInfoMsg) (tea.Model, tea.Cmd) {
	if m.syncState != nil {
		if msg.err != nil {
			m.syncState.Err = msg.err
		} else {
			m.syncState.Sources = msg.sources
		}
	}
	return m, nil
}

func (m Model) handleSyncComplete(msg syncCompleteMsg) (tea.Model, tea.Cmd) {
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
}

func (m Model) handleConfigLoaded(msg configLoadedMsg) (tea.Model, tea.Cmd) {
	if m.configState != nil {
		if msg.err != nil {
			m.configState.Err = msg.err
		} else {
			m.configState.Config = msg.cfg
			m.configState.Fields = populateConfigFields(msg.cfg)

			overlayWidth := m.width * 60 / 100
			if overlayWidth < 60 {
				overlayWidth = 60
			}
			if overlayWidth > 80 {
				overlayWidth = 80
			}
			contentWidth := overlayWidth - 6
			form, vals := buildConfigForm(m.configState.Fields, contentWidth)
			m.configState.Form = form
			m.configState.FormValues = vals
			return m, form.Init()
		}
	}
	return m, nil
}

func (m Model) handleConfigSaved(msg configSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		if m.configState != nil {
			m.configState.Err = msg.err
		} else {
			m.toast.Show(NewToast("Config save failed: "+msg.err.Error(), ToastError))
		}
		return m, nil
	}
	if m.configState != nil {
		m.activeView = ViewDashboard
		m.configState = nil
	}
	m.toast.Show(NewToast("Configuration saved", ToastSuccess))
	if cfg, err := config.Load(); err != nil {
		tuilog.Printf("error: config reload after save failed: %v", err)
		m.toast.Show(NewToast("Saved but reload failed — restart TUI to apply", ToastWarning))
	} else {
		m.cfg = cfg
	}
	return m, nil
}

func (m Model) handleRenameComplete(msg renameCompleteMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	m.activeView = ViewDashboard
	m.renameState = nil
	if msg.err != nil {
		m.toast.Show(NewToast(fmt.Sprintf("Rename failed: %s", msg.err), ToastError))
	} else {
		m.toast.Show(NewToast(fmt.Sprintf("Renamed %q to %q", msg.oldName, msg.newName), ToastSuccess))
		m.pendingSelect = msg.newName
		cmds = append(cmds, m.spinner.Tick)
	}
	cmds = append(cmds, m.fetchWorktrees)
	return m, tea.Batch(cmds...)
}

func (m Model) handleCheckoutBranches(msg checkoutBranchesMsg) (tea.Model, tea.Cmd) {
	if m.checkoutState != nil {
		if msg.err != nil {
			m.checkoutState.Err = msg.err
		} else {
			var available []string
			for _, br := range msg.branches {
				if !msg.usedBranches[br] {
					available = append(available, br)
				}
			}
			m.checkoutState.Branches = available
		}
	}
	return m, nil
}

func (m Model) handleCheckoutComplete(msg checkoutCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		if m.checkoutState != nil {
			m.checkoutState.Err = msg.err
			m.checkoutState.Switching = false
		}
		return m, nil
	}
	m.activeView = ViewDashboard
	m.checkoutState = nil
	m.toast.Show(NewToast(fmt.Sprintf("Switched to branch %q", msg.branch), ToastSuccess))
	return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)
}

func (m Model) handleSpinnerTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	if m.toast != nil {
		m.toast.Tick()
	}
	m.helpFooter.ClearExpiredHighlight()
	if m.isAnimating() {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	tuilog.Printf("KeyMsg: code=%d string=%q text=%q", msg.Code, msg.String(), msg.Text)
	if m.activeView != ViewDashboard {
		return m.handleKey(msg)
	}

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
}

func (m Model) handleDefault(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.activeView == ViewConfig && m.configState != nil && m.configState.Form != nil {
		return m.handleConfigFormMsg(msg)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// creationTrackerForSource returns the creationTracker for the given source,
// plus a label used in toast/log messages (e.g. "from PR "), and a cleanup
// function that nils the appropriate state pointer on success.
func (m *Model) creationTrackerForSource(source string) (ct creationTracker, label string, cleanup func()) {
	switch source {
	case "create":
		if m.createState != nil {
			return m.createState, "", func() { m.createState = nil }
		}
	case "pr":
		if m.prState != nil {
			return m.prState, "from PR ", func() { m.prState = nil }
		}
	case "issue":
		if m.issueState != nil {
			return m.issueState, "from issue ", func() { m.issueState = nil }
		}
	}
	return nil, "", nil
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

	// Persist preference (best-effort, don't block on errors)
	newVal := m.compactMode
	if err := saveUIPrefs(m.projectRoot, &UIPrefs{CompactMode: &newVal}); err != nil {
		tuilog.Printf("warning: failed to save UI prefs: %v", err)
	}

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

// isAnimating reports whether the spinner should keep ticking because an
// async operation, toast, or key highlight is active.
func (m Model) isAnimating() bool {
	if m.loading {
		return true
	}
	if m.isOperationAnimating() {
		return true
	}
	if m.isTrackerAnimating() {
		return true
	}
	if m.toast != nil && m.toast.Current != nil {
		return true
	}
	return m.helpFooter.HasHighlight()
}

// isOperationAnimating reports whether any worktree operation spinner is active.
func (m Model) isOperationAnimating() bool {
	return (m.createState != nil && m.createState.Creating) ||
		(m.forkState != nil && m.forkState.Forking) ||
		(m.syncState != nil && m.syncState.Syncing) ||
		(m.deleteState != nil && m.deleteState.Deleting) ||
		(m.renameState != nil && m.renameState.Renaming) ||
		(m.checkoutState != nil && m.checkoutState.Switching)
}

// isTrackerAnimating reports whether PR or issue state is loading/creating.
func (m Model) isTrackerAnimating() bool {
	return (m.prState != nil && (m.prState.Loading || m.prState.Creating)) ||
		(m.issueState != nil && (m.issueState.Loading || m.issueState.Creating))
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
	// Help overlay intercepts all keys when active
	if m.helpOverlay.Active {
		if key.Matches(msg, m.keys.Help) || key.Matches(msg, m.keys.Escape) {
			m.helpOverlay.Close()
			return m, nil
		}
		var cmd tea.Cmd
		m.helpOverlay, cmd = m.helpOverlay.Update(msg)
		return m, cmd
	}

	// ? opens help for views without text input
	// Views with active text inputs (Create, Rename, Checkout, Fork) pass ? through
	if key.Matches(msg, m.keys.Help) && !m.viewHasTextInput() {
		m.helpOverlay.Open(m.activeView, m.width, m.height)
		return m, nil
	}

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
	case ViewRename:
		return m.handleRenameKey(msg)
	case ViewCheckout:
		return m.handleCheckoutKey(msg)
	case ViewDashboard:
		return m.handleDashboardKey(msg)
	}
	return m, nil
}

func (m Model) handleDashboardKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.detailFocused {
		return m.handleDashboardDetailKey(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Escape):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Tab):
		m.detailFocused = true
		m.refreshDetailForFocus()
		return m, nil

	case msg.String() == "B":
		m.openSelectedPRURL()
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		m.helpFooter.SetHighlight("r")
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)

	case key.Matches(msg, m.keys.New):
		return m.handleDashboardNewKey()

	case key.Matches(msg, m.keys.Delete):
		return m.handleDashboardDeleteKey()

	case key.Matches(msg, m.keys.SwitchUp):
		return m.handleDashboardSwitchUpKey()

	case key.Matches(msg, m.keys.Enter):
		return m.handleDashboardEnterKey()

	case key.Matches(msg, m.keys.ViewMode):
		m.helpFooter.SetHighlight("v")
		m.toggleCompactMode()
		return m, nil

	case key.Matches(msg, m.keys.Sort):
		m.helpFooter.SetHighlight("o")
		m.sortMode = m.sortMode.Next()
		return m, m.applySortToList()

	case key.Matches(msg, m.keys.All):
		return m.enterBulkMode()

	case key.Matches(msg, m.keys.PRs):
		return m.enterPRView()

	case key.Matches(msg, m.keys.Issues):
		return m.enterIssueView()

	case key.Matches(msg, m.keys.Fork):
		return m.handleDashboardForkKey()

	case key.Matches(msg, m.keys.Sync):
		m.activeView = ViewSync
		m.syncState = NewSyncState(m.existingWorktreeItems())
		return m, gatherWIPInfoCmd(m.existingWorktreeItems())

	case key.Matches(msg, m.keys.Config):
		m.activeView = ViewConfig
		m.configState = NewConfigState()
		return m, loadConfigCmd()

	case key.Matches(msg, m.keys.Rename):
		return m.handleDashboardRenameKey()

	case key.Matches(msg, m.keys.Checkout):
		return m.handleDashboardCheckoutKey()
	}

	if updated, cmd, handled := m.handleQuickSwitch(msg); handled {
		return updated, cmd
	}

	return m.routeToList(msg)
}

func (m Model) handleDashboardDetailKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Escape) || key.Matches(msg, m.keys.Tab) {
		m.detailFocused = false
		m.refreshDetailForFocus()
		return m, nil
	}
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	if msg.String() == "B" {
		m.openSelectedPRURL()
		return m, nil
	}
	handleDetailFocusedKey(msg, m.keys, &m.detail)
	return m, nil
}

func (m *Model) openSelectedPRURL() {
	item, ok := m.selectedItem()
	if ok && item.AssociatedPR != nil && item.AssociatedPR.URL != "" {
		openURL(item.AssociatedPR.URL)
	}
}

func (m Model) handleDashboardNewKey() (tea.Model, tea.Cmd) {
	m.activeView = ViewCreate
	branches, branchErr := git.ListLocalBranches(m.projectRoot)
	if branchErr != nil {
		tuilog.Printf("warning: failed to list branches: %v", branchErr)
	}
	m.createState = &CreateState{
		Step:              CreateStepBranchChoice,
		ReturnView:        ViewDashboard,
		ProjectName:       m.projectName,
		Branches:          branches,
		BranchFilterInput: newBranchFilterInput(),
		BranchNameInput:   newBranchNameInput(),
		WorktreeBranches:  m.worktreeBranchMap(),
	}
	return m, nil
}

func (m Model) handleDashboardDeleteKey() (tea.Model, tea.Cmd) {
	item, ok := m.selectedItem()
	if ok && !item.IsMain && !item.IsProtected {
		m.activeView = ViewDelete
		m.deleteState = &DeleteState{
			Item:     &item,
			Warnings: gatherDeleteWarnings(&item),
		}
	}
	return m, nil
}

func (m Model) handleDashboardSwitchUpKey() (tea.Model, tea.Cmd) {
	item, ok := m.selectedItem()
	if !ok {
		return m, nil
	}
	tuilog.Printf("SwitchUp: item=%q path=%q isCurrent=%v", item.ShortName, item.Path, item.IsCurrent)
	m.switchForceUp = true
	m.switchTo = item.Path
	m.switchToDisplayName = item.displayName()
	return m, tea.Quit
}

func (m Model) handleDashboardEnterKey() (tea.Model, tea.Cmd) {
	item, ok := m.selectedItem()
	if !ok {
		return m, nil
	}
	tuilog.Printf("Enter: item=%q path=%q isCurrent=%v", item.ShortName, item.Path, item.IsCurrent)
	if item.IsCurrent {
		return m, tea.Quit
	}
	m.switchTo = item.Path
	m.switchToDisplayName = item.displayName()
	return m, tea.Quit
}

func (m Model) handleDashboardForkKey() (tea.Model, tea.Cmd) {
	item, ok := m.selectedItem()
	if !ok {
		return m, nil
	}
	m.activeView = ViewFork
	m.forkState = NewForkState(item)
	return m, wipCheckCmd(item.Path)
}

func (m Model) handleDashboardRenameKey() (tea.Model, tea.Cmd) {
	item, ok := m.selectedItem()
	if ok && !item.IsMain && !item.IsProtected {
		m.activeView = ViewRename
		m.renameState = NewRenameState(&item)
		return m, m.renameState.Input.Focus()
	}
	return m, nil
}

func (m Model) handleDashboardCheckoutKey() (tea.Model, tea.Cmd) {
	item, ok := m.selectedItem()
	if ok && !item.IsMain && !item.IsProtected {
		m.activeView = ViewCheckout
		m.checkoutState = NewCheckoutState(item)
		return m, tea.Batch(
			m.checkoutState.BranchFilterInput.Focus(),
			listCheckoutBranchesCmd(m.projectRoot, item.Path),
			wipCheckCmd(item.Path),
		)
	}
	return m, nil
}

func (m Model) handleQuickSwitch(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	runes := []rune(msg.Text)
	if len(runes) != 1 || m.list.FilterState() != list.Unfiltered {
		return m, nil, false
	}
	r := runes[0]
	if r < '1' || r > '9' {
		return m, nil, false
	}
	idx := int(r - '1')
	items := m.list.Items()
	if idx >= len(items) {
		return m, nil, true
	}
	item, ok := items[idx].(WorktreeItem)
	if !ok {
		return m, nil, true
	}
	if item.IsCurrent {
		return m, tea.Quit, true
	}
	m.switchTo = item.Path
	m.switchToDisplayName = item.displayName()
	return m, tea.Quit, true
}

func (m Model) routeToList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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

	if m.deleteState.Deleting {
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
		m.deleteState.Deleting = true
		return m, tea.Batch(m.spinner.Tick, deleteWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, name, m.deleteState.DeleteBranch))

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

func (m Model) handleRenameKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.renameState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	if m.renameState.Renaming {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Escape):
		m.activeView = ViewDashboard
		m.renameState = nil
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		newName := m.renameState.Input.Value()
		if newName == "" {
			m.renameState.Error = "name cannot be empty"
			return m, nil
		}
		if newName == m.renameState.Item.ShortName {
			m.renameState.Error = "new name is the same as current name"
			return m, nil
		}

		// Check if name is already taken
		for _, li := range m.list.Items() {
			if item, ok := li.(WorktreeItem); ok {
				if item.ShortName == newName {
					m.renameState.Error = fmt.Sprintf("worktree %q already exists", newName)
					return m, nil
				}
			}
		}

		m.renameState.Error = ""
		m.renameState.Renaming = true
		return m, tea.Batch(m.spinner.Tick, renameWorktreeCmd(m.worktreeMgr, m.stateMgr, m.renameState.Item.ShortName, newName))

	default:
		// Forward to text input
		var cmd tea.Cmd
		m.renameState.Input, cmd = m.renameState.Input.Update(msg)
		// Clear error on typing
		if m.renameState.Error != "" {
			m.renameState.Error = ""
		}
		return m, cmd
	}
}

func (m Model) handleCreateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.createState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	switch m.createState.Step {
	case CreateStepBranchChoice:
		return m.handleBranchChoiceKey(msg)
	case CreateStepBranchSelect:
		return m.handleBranchSelectKey(msg)
	case CreateStepBranchCreate:
		return m.handleBranchCreateKey(msg)
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
	case key.Matches(msg, m.keys.ShiftTab):
		s.Step = CreateStepBranchSelect
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		returnView := s.ReturnView
		m.createState = nil
		m.activeView = returnView
		return m, nil

	case key.Matches(msg, m.keys.Back):
		s.Step = CreateStepBranchSelect
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
		s.NameInput = newNameInput("")
		s.Step = CreateStepName
		return m, s.NameInput.Focus()
	}
	return m, nil
}

// handleConfirmKey handles the confirmation step key input.
func (m Model) handleConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.createState
	switch {
	case key.Matches(msg, m.keys.ShiftTab):
		s.Step = CreateStepName
		return m, s.NameInput.Focus()

	case key.Matches(msg, m.keys.Escape):
		returnView := s.ReturnView
		m.createState = nil
		m.activeView = returnView
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
	m.createState.ActivityLog = NewActivityLog(60, 10)
	return m, tea.Batch(m.spinner.Tick, createWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, name, baseBranch))
}

// handleBranchChoiceKey handles the initial "Select existing" vs "Create new" choice.
func (m Model) handleBranchChoiceKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.createState
	switch {
	case key.Matches(msg, m.keys.ShiftTab):
		if s.Source != "" {
			returnView := s.ReturnView
			m.createState = nil
			m.activeView = returnView
			return m, nil
		}
		// First step from dashboard — no back, treat as no-op
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		returnView := s.ReturnView
		m.createState = nil
		m.activeView = returnView
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if s.BranchChoice > 0 {
			s.BranchChoice--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if s.BranchChoice < 1 {
			s.BranchChoice++
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		if s.BranchChoice == 0 {
			s.Step = CreateStepBranchSelect
			s.BranchCursor = 0
			s.BranchFilterMode = BranchFilterOff
			return m, nil
		}
		// Create new branch — pre-fill from source context if available
		s.Step = CreateStepBranchCreate
		s.BranchNameInput = newBranchNameInput()
		if s.NameSuggestion != "" {
			s.BranchNameInput.SetValue(s.NameSuggestion)
		}
		return m, s.BranchNameInput.Focus()
	}
	return m, nil
}

// handleBranchSelectKey handles the filterable branch list step.
func (m Model) handleBranchSelectKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.createState

	// When filter is active, route most keys to the textinput
	if s.BranchFilterMode == BranchFilterOn {
		switch {
		case key.Matches(msg, m.keys.Escape):
			// Exit filter mode, keep filter text
			s.BranchFilterMode = BranchFilterOff
			s.BranchFilterInput.Blur()
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			// Accept filter and exit filter mode
			s.BranchFilterMode = BranchFilterOff
			s.BranchFilterInput.Blur()
			return m, nil

		default:
			// All other keys go to the textinput (j/k are typed as text)
			prevVal := s.BranchFilterInput.Value()
			var cmd tea.Cmd
			s.BranchFilterInput, cmd = s.BranchFilterInput.Update(msg)
			if s.BranchFilterInput.Value() != prevVal {
				s.BranchCursor = 0
			}
			return m, cmd
		}
	}

	// Filter is off — j/k navigate, / enters filter mode
	filter := s.BranchFilterInput.Value()
	filtered := filteredBranches(s.Branches, filter)
	totalItems := len(filtered)

	switch {
	case key.Matches(msg, m.keys.ShiftTab):
		s.Step = CreateStepBranchChoice
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		returnView := s.ReturnView
		m.createState = nil
		m.activeView = returnView
		return m, nil

	case key.Matches(msg, m.keys.Back):
		s.Step = CreateStepBranchChoice
		s.BranchFilterInput.SetValue("")
		s.BranchCursor = 0
		return m, nil

	case msg.String() == "/":
		s.BranchFilterMode = BranchFilterOn
		return m, s.BranchFilterInput.Focus()

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
			return m.selectExistingBranch(filtered[s.BranchCursor])
		}
		return m, nil
	}
	return m, nil
}

// handleBranchCreateKey handles the new branch name text input step.
func (m Model) handleBranchCreateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.createState

	switch {
	case key.Matches(msg, m.keys.ShiftTab):
		s.Step = CreateStepBranchChoice
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		returnView := s.ReturnView
		m.createState = nil
		m.activeView = returnView
		return m, nil

	case key.Matches(msg, m.keys.Back):
		if s.BranchNameInput.Value() == "" {
			s.Step = CreateStepBranchChoice
			return m, nil
		}
		// Let textinput handle backspace
		var cmd tea.Cmd
		s.BranchNameInput, cmd = s.BranchNameInput.Update(msg)
		return m, cmd

	case key.Matches(msg, m.keys.Enter):
		name := s.BranchNameInput.Value()
		if name == "" {
			return m, nil
		}
		s.BaseBranch = ""
		s.NewBranchName = name
		if s.NameSuggestion == "" {
			s.NameSuggestion = name
		}
		s.NameInput = newNameInput("")
		s.Step = CreateStepName
		return m, s.NameInput.Focus()

	default:
		var cmd tea.Cmd
		s.BranchNameInput, cmd = s.BranchNameInput.Update(msg)
		return m, cmd
	}
}

func (m Model) selectExistingBranch(branch string) (tea.Model, tea.Cmd) {
	s := m.createState
	s.BaseBranch = branch
	s.NewBranchName = ""

	strategy := ""
	if m.cfg != nil {
		strategy = m.cfg.TUI.WorktreeNameFromBranch
	}
	// Keep the existing suggestion if pre-set from a source (e.g., issue name)
	if s.NameSuggestion == "" {
		s.NameSuggestion = worktree.DeriveWorktreeName(branch, strategy)
	}
	s.NameInput = newNameInput("")

	if m.cfg != nil && m.cfg.TUI.SkipBranchNotice != nil && *m.cfg.TUI.SkipBranchNotice {
		if m.cfg.TUI.DefaultBranchAction == "fork" {
			s.BaseBranch = ""
		}
		s.Step = CreateStepName
		return m, s.NameInput.Focus()
	}

	s.ActionChoice = 0
	s.DontShowAgain = false
	s.Step = CreateStepBranchAction
	return m, nil
}

// handleNameKey handles the name step key input.
func (m Model) handleNameKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.createState

	switch {
	case key.Matches(msg, m.keys.ShiftTab):
		// Always go back regardless of input content
		if s.Source != "" {
			// From PR/issue view — back returns to source view
			returnView := s.ReturnView
			m.createState = nil
			m.activeView = returnView
			return m, nil
		}
		s.Step = CreateStepBranchChoice
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		returnView := s.ReturnView
		m.createState = nil
		m.activeView = returnView
		return m, nil

	case key.Matches(msg, m.keys.Back):
		// Only go back if the name input is empty
		if s.NameInput.Value() == "" {
			if s.Source != "" {
				// From PR/issue view — dismiss wizard, return to source view
				returnView := s.ReturnView
				m.createState = nil
				m.activeView = returnView
				return m, nil
			}
			s.Step = CreateStepBranchChoice
			return m, nil
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

func (m *Model) applySortToList() tea.Cmd {
	sorted := sortWorktreeItems(m.list.Items(), m.sortMode)
	cmd := m.list.SetItems(sorted)
	m.computeColumnWidths()
	m.updateDetailContent()
	return cmd
}

func (m Model) enterPRView() (tea.Model, tea.Cmd) {
	branches := m.worktreeBranchMap()
	m.activeView = ViewPRs
	m.prState = &PRViewState{
		Loading:          true,
		WorktreeBranches: branches,
		FilterInput:      newFilterInput(""),
	}
	return m, tea.Batch(m.spinner.Tick, m.fetchPRsCmd)
}

func (m Model) enterIssueView() (tea.Model, tea.Cmd) {
	branches := m.worktreeBranchMap()
	m.activeView = ViewIssues
	m.issueState = &IssueViewState{
		Loading:          true,
		FilterInput:      newFilterInput(""),
		WorktreeBranches: branches,
	}
	return m, tea.Batch(m.spinner.Tick, m.fetchIssuesCmd)
}

// worktreeBranchMap builds a map from branch name to worktree short name
// for all current worktrees. Used by PR and issue views to detect existing worktrees.
func (m Model) worktreeBranchMap() map[string]string {
	branches := make(map[string]string)
	for _, li := range m.list.Items() {
		if item, ok := li.(WorktreeItem); ok {
			branches[item.Branch] = item.ShortName
		}
	}
	return branches
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
		return m.viewNotReady()
	}
	if m.loading {
		return m.viewLoading()
	}
	if len(m.list.Items()) == 0 {
		return m.viewEmpty()
	}

	result := m.viewForActiveView()
	if result == "" {
		result = m.renderDashboard()
	}
	return m.compositeHelpOverlay(result)
}

func (m Model) viewNotReady() string {
	tuilog.Printf("View: not ready")
	if m.activeView == ViewPRs && m.prState != nil {
		return m.spinner.View() + " Loading PRs..."
	}
	if m.activeView == ViewIssues && m.issueState != nil {
		return m.spinner.View() + " Loading issues..."
	}
	return "loading..."
}

func (m Model) viewLoading() string {
	brand := Styles.Header.Render("  grove")
	loading := m.spinner.View() + " " + Styles.TextMuted.Render("Loading worktrees...")
	content := brand + "\n\n" + loading
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) viewEmpty() string {
	brand := Styles.Header.Render("  grove")
	msg := Styles.TextMuted.Render("No worktrees found")
	hint := Styles.HelpKey.Render("n") + " " + Styles.HelpDesc.Render("to create your first worktree")
	content := lipgloss.JoinVertical(lipgloss.Center, brand, "", msg, hint)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) viewForActiveView() string {
	switch m.activeView {
	case ViewHelp:
		return m.renderDashboard()
	case ViewDelete:
		return m.overlayOnDashboard(m.deleteState != nil, func() string { return renderDeleteV2(m.deleteState, m.width) })
	case ViewCreate:
		if m.createState != nil {
			overlay := renderCreateV2(m.createState, m.width, m.spinner.View())
			var bg string
			switch m.createState.Source {
			case "pr":
				if m.prState != nil {
					bg = m.renderPRPanel()
				}
			case "issue":
				if m.issueState != nil {
					bg = m.renderIssuePanel()
				}
			}
			if bg == "" {
				bg = m.renderDashboard()
			}
			return centerOverlay(bg, overlay, m.width, m.height)
		}
		return ""
	case ViewBulk:
		return m.overlayOnDashboard(m.bulkState != nil, func() string { return renderBulk(m.bulkState) })
	case ViewPRs:
		if m.prState != nil {
			return m.renderPRPanel()
		}
	case ViewIssues:
		if m.issueState != nil {
			return m.renderIssuePanel()
		}
	case ViewFork:
		return m.overlayOnDashboard(m.forkState != nil, func() string { return renderFork(m.forkState, m.width) })
	case ViewSync:
		return m.overlayOnDashboard(m.syncState != nil, func() string { return renderSync(m.syncState, m.width) })
	case ViewConfig:
		return m.overlayOnDashboard(m.configState != nil, func() string { return renderConfig(m.configState, m.width) })
	case ViewRename:
		return m.overlayOnDashboard(m.renameState != nil, func() string { return renderRename(m.renameState, m.width) })
	case ViewCheckout:
		return m.overlayOnDashboard(m.checkoutState != nil, func() string { return renderCheckout(m.checkoutState, m.width) })
	}
	return ""
}

// compositeHelpOverlay renders the help overlay on top of the given content if active.
func (m Model) compositeHelpOverlay(content string) string {
	if m.helpOverlay.Active {
		helpPanel := m.helpOverlay.View(m.width, m.height)
		return centerOverlay(content, helpPanel, m.width, m.height)
	}
	return content
}

// viewHasTextInput returns true if the active view has a focused text input
// that should receive ? as a character rather than triggering help.
func (m Model) viewHasTextInput() bool {
	switch m.activeView {
	case ViewCreate, ViewRename, ViewCheckout, ViewFork:
		return true
	case ViewPRs:
		return m.prState != nil && m.prState.Filtering
	case ViewIssues:
		return m.issueState != nil && m.issueState.Filtering
	case ViewDashboard:
		return m.list.FilterState() == list.Filtering
	}
	return false
}

func (m Model) renderDashboard() string {
	m.header = m.buildDashboardHeader()
	statusBar := m.header.View(m.width)

	body := m.renderDashboardBody()

	var footer string
	if m.detailFocused {
		footer = m.helpFooter.RenderCompactWithHints(m.dashboardDetailFocusedHints(), m.width-4)
	} else {
		footer = m.helpFooter.RenderCompact(ViewDashboard, m.width-4)
	}

	statusBar = m.applyToastToHeader(statusBar)

	body = lipgloss.NewStyle().Padding(0, 1).Render(body)
	body = m.clampBodyToHeight(body, footer)

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, body, footer)
}

// buildDashboardHeader populates the header from the current worktree list state.
func (m Model) buildDashboardHeader() Header {
	h := Header{
		ProjectName:   m.projectName,
		WorktreeCount: len(m.list.Items()),
		SortLabel:     m.sortMode.String(),
	}
	for _, li := range m.list.Items() {
		item, ok := li.(WorktreeItem)
		if !ok {
			continue
		}
		if item.IsCurrent {
			h.CurrentBranch = item.Branch
			h.CurrentName = item.ShortName
		}
		for _, ps := range item.PluginStatuses {
			if strings.Contains(ps.Detail, "pointed") {
				h.ContainerTarget = item.ShortName
			}
		}
	}
	return h
}

// renderDashboardBody renders the list and detail panels in side-by-side or stacked layout.
func (m Model) renderDashboardBody() string {
	if m.width > 100 {
		return m.renderSideBySideBody()
	}
	return m.renderStackedBody()
}

func (m Model) renderSideBySideBody() string {
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
	return lipgloss.JoinHorizontal(lipgloss.Top, listContent, divider, detailView)
}

func (m Model) renderStackedBody() string {
	bodyWidth := m.width - 2
	listView := m.list.View()
	if m.compactMode {
		header := renderListHeader(m.listDelegate, bodyWidth)
		listView = lipgloss.JoinVertical(lipgloss.Left, header, listView)
	}
	separator := renderNamedSeparator(m.selectedItemName(), bodyWidth)
	detailView := m.renderDetailPanel()
	return lipgloss.JoinVertical(lipgloss.Left, listView, separator, detailView)
}

// applyToastToHeader composites a toast notification onto the header if one is active.
func (m Model) applyToastToHeader(statusBar string) string {
	if m.toast == nil || m.toast.Current == nil {
		return statusBar
	}
	toastView := m.toast.View(m.width)
	if toastView == "" {
		return statusBar
	}
	return compositeToastOnHeader(statusBar, toastView, m.width)
}

// clampBodyToHeight trims the body so statusBar + body + footer fits exactly m.height lines.
func (m Model) clampBodyToHeight(body, footer string) string {
	if m.height <= 0 {
		return body
	}
	footerLines := strings.Count(footer, "\n") + 1
	bodyBudget := m.height - 1 - footerLines
	if bodyBudget > 0 {
		return clampLines(body, bodyBudget)
	}
	return body
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

func (m Model) renderDetailPanel() string {
	return m.detail.View()
}

// refreshDetailForFocus updates the detail viewport content with the
// appropriate border style (highlighted when focused, normal when not).
func (m *Model) refreshDetailForFocus() {
	item, ok := m.selectedItem()
	if !ok {
		return
	}
	var content string
	if m.detailFocused {
		content = renderDetailV2Focused(&item, m.detail.Width())
	} else {
		content = renderDetailV2(&item, m.detail.Width())
	}
	m.detail.SetContent(content)
}

// renderDetailV2Focused renders the worktree detail panel with a highlighted
// border to indicate the panel has keyboard focus.
func renderDetailV2Focused(item *WorktreeItem, width int) string {
	if item == nil || width < 20 {
		return ""
	}

	innerWidth := max(width-6, 16)

	var sections []string
	sections = append(sections, renderMetadataGrid(item, innerWidth))
	if len(item.DirtyFiles) > 0 {
		sections = append(sections, renderChangesSection(item.DirtyFiles, innerWidth))
	}

	body := strings.Join(sections, "\n\n")

	card := Styles.DetailBorder.
		BorderForeground(Colors.Primary).
		Width(width - 2).
		Render(body)

	title := " " + Styles.DetailTitle.Render(item.ShortName) + " "
	card = injectBorderTitleWithColor(card, title, Colors.Primary)

	return card
}

// dashboardDetailFocusedHints returns hints for when the dashboard detail panel is focused.
func (m Model) dashboardDetailFocusedHints() []Hint {
	hints := []Hint{
		{"↑↓", "scroll"},
		{"g/G", "top/bottom"},
	}
	if item, ok := m.selectedItem(); ok && item.AssociatedPR != nil {
		hints = append(hints, Hint{"B", "open PR"})
	}
	hints = append(hints, Hint{"tab", "list"}, Hint{"esc", "quit"})
	return hints
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

// clampLines pads or trims s to exactly n lines.
func clampLines(s string, n int) string {
	if n <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for len(lines) < n {
		lines = append(lines, "")
	}
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// overlayOnDashboard renders the overlay produced by renderFn centered on top
// of the dashboard. If active is false (state is nil), it returns "" so the
// caller falls through to the default dashboard render.
func (m Model) overlayOnDashboard(active bool, renderFn func() string) string {
	if !active {
		return ""
	}
	return centerOverlay(m.renderDashboard(), renderFn(), m.width, m.height)
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
	m.activeView = ViewPRs
	m.prState = &PRViewState{
		Loading:          true,
		WorktreeBranches: m.worktreeBranchMap(),
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
func RunPRs(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string, pluginMgr ...*plugins.Manager) (string, bool, error) {
	tuilog.Init()
	defer tuilog.Close()

	model := NewModel(mgr, stateMgr, projectRoot, pluginMgr...).ConfigureForPRs()
	return runModel(model)
}

// RunIssues starts the TUI directly in the issue browser view.
func RunIssues(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string, pluginMgr ...*plugins.Manager) (string, bool, error) {
	tuilog.Init()
	defer tuilog.Close()

	model := NewModel(mgr, stateMgr, projectRoot, pluginMgr...).ConfigureForIssues()
	return runModel(model)
}

// Run starts the TUI and returns the path to switch to (if any) and whether to force docker up.
func Run(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string, pluginMgr ...*plugins.Manager) (string, bool, error) {
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

	useCC := tmux.ShouldUseControlMode(nil)
	if m.cfg != nil {
		useCC = tmux.ShouldUseControlMode(m.cfg.Tmux.ControlMode)
	}

	var attachErr error
	if useCC {
		attachErr = tmux.AttachSessionControlMode(sessionName)
	} else {
		attachErr = tmux.AttachSession(sessionName)
	}
	if attachErr != nil {
		tuilog.Printf("warning: failed to attach tmux session: %v", attachErr)
		return false
	}
	return true
}

func runModel(model Model) (string, bool, error) {
	tuilog.Printf("runModel: projectRoot=%s activeView=%d", model.projectRoot, model.activeView)

	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		tuilog.Printf("runModel: tea.Program error: %v", err)
		return "", false, fmt.Errorf("tui error: %w", err)
	}

	m := finalModel.(Model)
	tuilog.Printf("runModel: exit ready=%v loading=%v items=%d switchTo=%q err=%v",
		m.ready, m.loading, len(m.list.Items()), m.switchTo, m.err)

	if m.Err() != nil {
		return "", false, m.Err()
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
					return "", false, fmt.Errorf("failed to write cd file: %w", err)
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

	return switchPath, m.SwitchForceUp(), nil
}
