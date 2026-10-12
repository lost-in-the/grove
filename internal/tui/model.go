package tui

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/plugins"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tuilog"
	"github.com/lost-in-the/grove/internal/updatecheck"
	"github.com/lost-in-the/grove/internal/version"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/tracker"
)

// ActiveView tracks which view is currently displayed.
type ActiveView int

const (
	ViewDashboard ActiveView = iota
	ViewHelp
	ViewDelete
	ViewBranchForm
	ViewBulk
	ViewPRs
	ViewIssues
	ViewSync
	ViewConfig
	ViewRename
	ViewUpdate
	ViewDetailSheet // full-screen detail below 110 columns
	ViewOmni        // search over worktrees, actions, PRs, and issues
	ViewMessages    // session history of warnings and errors
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
	header        Header
	toast         *ToastModel
	helpFooter    *HelpFooter
	helpOverlay   *HelpOverlay
	updateOverlay *UpdateOverlay

	// Cached update-available info, populated once at startup so we
	// don't read the cache file on every render. Empty strings = no update.
	updateLatestVersion string
	updateLatestURL     string
	updateNotify        bool // update notices allowed; opt-outs hide the badge only

	// Keys
	keys KeyMap

	// State
	views         viewStack
	ready         bool // true after first WindowSizeMsg
	loading       bool // true while fetching worktrees
	detailFocused bool // true when dashboard detail panel has focus

	// Sort
	sortMode     SortMode
	sortReversed bool // O reverses the current sort
	pendingG     bool // g was pressed; a second g goes to the top
	pendingM     bool // m was pressed; the pin strip is open
	pins         pinSet
	lastActive   string // display name of the worktree before this one (state.LastWorktree)

	// Overlay state
	deleteState *DeleteState
	branchForm  *BranchFormState
	omni        *OmniState

	// Session caches of PR and issue data, filled whenever a browser
	// fetches, so omni can search them without calling gh (spec 9.2).
	prCache        []*tracker.PullRequest
	issueCache     []*tracker.Issue
	pendingPR      int // after the PR browser fetches, put its cursor on this PR
	messagesOffset int // first row shown in the message history
	pendingIssue   int
	bulkState      *BulkState
	prState        *PRViewState
	issueState     *IssueViewState
	syncState      *SyncState
	configState    *ConfigState
	renameState    *RenameState

	// Post-create selection
	pendingSelect string

	// Generation counter for the deferred detail-metrics fetch. Each
	// FetchWorktrees bumps this; results from older generations are
	// dropped so a stale fetch can't apply old numbers to a fresh item set.
	detailMetricsGen int

	// Output
	switchTo            string
	switchToDisplayName string // display name for tmux session naming
	switchForceUp       bool   // when true, signal CLI to start containers after switch
	err                 error

	// Layout
	width, height int
	compactMode   bool // true = one-line rows, false = rows with the commit message

	// agentAware is true when the first fetch reported the herdr backend.
	// It is decided once, so columns never appear or vanish mid-session.
	agentAware    bool
	agentAwareSet bool

	// The agent poll (spec 7.4): polling is true once the loop starts;
	// pollGen numbers polls so a stale result is dropped.
	polling      bool
	pollInFlight bool
	pollGen      int

	// now is the clock rows use for ages; tests pin it.
	now func() time.Time
}

// updateSkip reports whether update notices are opted out; tests stub it.
var updateSkip = updatecheck.Skip

// NewModel creates a new TUI model.
func NewModel(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string, pluginMgr ...*plugins.Manager) Model {
	// projectRoot is already resolved — load config from its .grove directly
	// instead of re-discovering it via FindRoot's git spawns (#142).
	cfg, cfgErr := config.LoadFromGroveDir(filepath.Join(projectRoot, ".grove"))
	if cfgErr != nil {
		tuilog.Printf("warning: failed to load config: %v", cfgErr)
	}
	keys := DefaultKeyMap()

	s := GroveSpinner()

	var configCompact, prefCompact *bool
	if cfg != nil {
		configCompact = cfg.TUI.CompactList
	}
	pins := pinSet{}
	if prefs := loadUIPrefs(projectRoot); prefs != nil {
		prefCompact = prefs.CompactMode
		for slot, alias := range prefs.Pins {
			pins[slot] = alias
		}
	}
	compact := startupCompact(configCompact, prefCompact)

	l := newWorktreeList(rowDelegate{TwoLine: !compact, Secondary: listSecondary(cfg)})

	h := help.New()

	var pm *plugins.Manager
	if len(pluginMgr) > 0 {
		pm = pluginMgr[0]
	}

	// Read update-check cache once at startup. Empty strings if no update is
	// available (which is the common case). Reading from disk here instead of
	// on every render keeps the hot path allocation-free.
	//
	// Skip (GROVE_NO_UPDATE_NOTIFIER, GROVE_AGENT_MODE, etc.) hides the header
	// badge, matching the CLI box and `grove version`. `g u` is an explicit
	// request, so it still reports the cache (spec 12).
	latest, latestURL, _ := updatecheck.CachedRelease(version.Version)
	notify := !updateSkip(false, version.Version)

	return Model{
		worktreeMgr:         mgr,
		stateMgr:            stateMgr,
		pluginMgr:           pm,
		projectRoot:         projectRoot,
		projectName:         mgr.GetProjectName(),
		cfg:                 cfg,
		cfgLoadErr:          cfgErr,
		keys:                keys,
		list:                l,
		compactMode:         compact,
		pins:                pins,
		now:                 time.Now,
		lastActive:          lastWorktreeName(stateMgr),
		spinner:             s,
		help:                h,
		toast:               NewToastModel(),
		helpFooter:          NewHelpFooter(),
		helpOverlay:         NewHelpOverlay(),
		updateOverlay:       NewUpdateOverlay(),
		updateLatestVersion: latest,
		updateLatestURL:     latestURL,
		updateNotify:        notify,
		loading:             true,
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
	tuilog.Printf("Init: loading=%v ready=%v activeView=%d", m.loading, m.ready, m.activeView())
	cmds := []tea.Cmd{m.spinner.Tick, m.fetchWorktrees}
	if m.activeView() == ViewPRs {
		cmds = append(cmds, m.fetchPRsCmd)
	}
	if m.activeView() == ViewIssues {
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
	case detailMetricsLoadedMsg:
		return m.handleDetailMetricsLoaded(msg)
	case agentPollTickMsg:
		return m.handleAgentPollTick()
	case agentPollMsg:
		return m.handleAgentPoll(msg)
	case prLookupMsg:
		return m.handlePRLookup(msg)
	case worktreeDeletedMsg:
		return m.handleWorktreeDeleted(msg)
	case creationLogMsg:
		return m.handleCreationLog(msg)
	case creationDoneMsg:
		return m.handleCreationDone(msg)
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
	case checkoutCompleteMsg:
		return m.handleCheckoutComplete(msg)
	case spinner.TickMsg:
		return m.handleSpinnerTick(msg)
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	case tea.PasteMsg:
		if m.activeView() == ViewBranchForm && m.branchForm != nil {
			return m, m.branchForm.paste(msg)
		}
		if m.activeView() == ViewOmni && m.omni != nil {
			return m, m.typeOmni(msg)
		}
		return m.handleDefault(msg)
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
	if m.branchForm != nil {
		m.branchForm.setWidth(m.width, m.height)
	}
	if m.omni != nil {
		w, _ := omniFrameSize(m.width, m.height)
		m.omni.Input.SetWidth(max(4, w-8))
	}
	tuilog.Printf("WindowSizeMsg: %dx%d, list size: %dx%d", msg.Width, msg.Height, m.list.Width(), m.list.Height())
	if !wasReady && m.cfgLoadErr != nil {
		m.toast.Show(NewToast("Config load failed: "+m.cfgLoadErr.Error(), ToastWarning))
		m.cfgLoadErr = nil
	}
	if m.activeView() == ViewHelp {
		m.helpOverlay.Open(m.helpOverlay.ForView, m.width, m.height)
	}
	m.reconcileDetailWithWidth()
	return m, nil
}

// reconcileDetailWithWidth keeps the detail mode right for the width tier:
// pane focus exists only at 110+, the full-screen sheet only below it.
func (m *Model) reconcileDetailWithWidth() {
	wide := isWideLayout(m.width)
	if !wide && m.detailFocused {
		m.detailFocused = false
		m.refreshDetailForFocus()
	}
	if m.layoutView() == ViewDetailSheet {
		if wide && m.activeView() == ViewDetailSheet {
			m.popView()
			m.updateDetailContent()
			return
		}
		m.refreshDetailSheet()
	}
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
	if m.sortMode != SortByName || m.sortReversed {
		listItems = sortWorktreeItems(listItems, m.sortMode, m.sortReversed)
	}
	// Capture SetItems' cmd: with a filter applied it re-runs the filter, and
	// dropping it left the list showing no items until the user re-filtered (B38).
	setItemsCmd := m.list.SetItems(listItems)
	// Prune only here, after a successful fetch: a failed fetch must never
	// wipe pins (spec 8.3).
	if len(m.pins) > 0 {
		aliases := map[string]bool{}
		for _, item := range msg.items {
			aliases[item.ShortName] = true
		}
		if pruned := m.pins.prune(aliases); len(pruned) != len(m.pins) {
			m.changePins(func(p pinSet) pinSet { return p.prune(aliases) })
		}
	}
	if !m.agentAwareSet {
		// Decided by the first successful fetch, even one that found no
		// backend, so the column never appears mid-session.
		m.agentAware, m.agentAwareSet = msg.backend == "herdr", true
	}
	// A poll started before this fetch would overwrite fresher sessions.
	m.pollGen++
	var pollCmd tea.Cmd
	if m.agentAware && !m.polling {
		m.polling = true
		pollCmd = agentPollTick()
	}
	m.refreshDelegate()
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

	// Refresh worktree-branch maps for browser views that were constructed
	// before worktrees finished loading (the `grove prs`/`grove issues` direct
	// entry points build these states with an empty list). Without this, the
	// "✓ worktree" badge and the "worktree exists" prompt never fire.
	if m.prState != nil {
		m.prState.WorktreeBranches = m.worktreeBranchMap()
	}
	if m.issueState != nil {
		m.issueState.WorktreeBranches = m.worktreeBranchMap()
	}

	var branches []string
	for _, li := range listItems {
		if item, ok := li.(WorktreeItem); ok {
			branches = append(branches, item.Branch)
		}
	}

	// Bump the generation BEFORE dispatching the lazy fetch. Any in-flight
	// metrics from a previous fetch will arrive with an older gen and get
	// dropped by handleDetailMetricsLoaded.
	m.detailMetricsGen++
	return m, tea.Batch(
		setItemsCmd,
		pollCmd,
		lookupPRsCmd(branches),
		fetchDetailMetricsCmd(m.detailMetricsGen, msg.items, ResolveDefaultBranch(m.cfg)),
	)
}

func (m Model) handleDetailMetricsLoaded(msg detailMetricsLoadedMsg) (tea.Model, tea.Cmd) {
	// Drop results from a stale fetch (a newer FetchWorktrees was dispatched
	// after we kicked off this metrics load).
	if msg.gen != m.detailMetricsGen || msg.metrics == nil {
		return m, nil
	}

	listItems := m.list.Items()
	for i, li := range listItems {
		item, ok := li.(WorktreeItem)
		if !ok {
			continue
		}
		if metrics, found := msg.metrics[item.Path]; found {
			item.CommitCount = metrics.CommitCount
			item.StashCount = metrics.StashCount
			listItems[i] = item
		}
	}
	// Return SetItems' cmd so an active filter is re-applied instead of
	// blanking the list (B38).
	cmd := m.list.SetItems(listItems)
	m.updateDetailContent()
	return m, cmd
}

func (m Model) handlePRLookup(msg prLookupMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
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
		// Return SetItems' cmd so an active filter is re-applied (B38).
		cmd = m.list.SetItems(listItems)
		m.updateDetailContent()
	}
	return m, cmd
}

func (m Model) handleWorktreeDeleted(msg worktreeDeletedMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	m.popToRoot()
	m.deleteState = nil
	if msg.err == nil && m.pins.slotOf(msg.name) != "" {
		name := msg.name
		m.changePins(func(p pinSet) pinSet { return p.drop(name) })
	}
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
	m.popToRoot()
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

func (m Model) handlePRsFetched(msg prsFetchedMsg) (tea.Model, tea.Cmd) {
	// Read and clear the pending PR now, so a failed fetch or a browser
	// closed before it returned cannot move a later, unrelated open.
	pending := m.pendingPR
	m.pendingPR = 0
	if msg.err == nil {
		m.prCache = msg.prs
	}
	if m.prState != nil {
		m.prState.Loading = false
		if msg.err != nil {
			m.prState.Error = msg.err.Error()
		} else {
			m.prState.PRs = msg.prs
			for i, pr := range msg.prs {
				if pending != 0 && pr.Number == pending {
					m.prState.Cursor = i
				}
			}
		}
	}
	return m, nil
}

func (m Model) handleIssuesFetched(msg issuesFetchedMsg) (tea.Model, tea.Cmd) {
	pending := m.pendingIssue
	m.pendingIssue = 0
	if msg.err == nil {
		m.issueCache = msg.issues
	}
	if m.issueState != nil {
		m.issueState.Loading = false
		if msg.err != nil {
			m.issueState.Error = msg.err.Error()
		} else {
			m.issueState.Issues = msg.issues
			for i, is := range msg.issues {
				if pending != 0 && is.Number == pending {
					m.issueState.Cursor = i
				}
			}
		}
	}
	return m, nil
}

func (m Model) handleBulkDeleteDone(msg bulkDeleteDoneMsg) (tea.Model, tea.Cmd) {
	m.popToRoot()
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
	// The form ignores a result for any other worktree, so a stale check from
	// a form canceled earlier cannot mark this one clean.
	if m.branchForm != nil {
		m.branchForm.setWIP(msg.path, msg.hasWIP, msg.files, msg.err)
	}
	return m, nil
}

// finishFormWithError shows a failed copy or switch in the form, which stays
// open so the user can change something and try again. The failed command
// may already have stashed or captured changes, so the old check is stale:
// enter waits for a fresh one.
func (m Model) finishFormWithError(err error) (tea.Model, tea.Cmd) {
	s := m.branchForm
	if s == nil {
		return m, nil
	}
	if s.Log != nil {
		s.Log.SetDone(err)
	}
	s.setCreatingDone(err.Error())
	if s.Source == nil {
		return m, nil
	}
	s.wipDone = false
	return m, wipCheckCmd(s.Source.Path)
}

func (m Model) handleForkComplete(msg forkCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil && msg.name == "" {
		return m.finishFormWithError(msg.err)
	}
	m.popToRoot()
	m.branchForm = nil
	m.pendingSelect = msg.name
	if msg.err != nil {
		m.toast.Show(NewToast(fmt.Sprintf("Copied to %q (warning: %s)", msg.name, msg.err), ToastWarning))
	} else {
		m.toast.Show(NewToast(fmt.Sprintf("Copied to %q", msg.name), ToastSuccess))
	}
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
	m.popToRoot()
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
		m.popToRoot()
		m.configState = nil
	}
	m.toast.Show(NewToast("Configuration saved", ToastSuccess))
	if cfg, err := config.Load(); err != nil {
		tuilog.Printf("error: config reload after save failed: %v", err)
		m.toast.Show(NewToast("Saved but reload failed — restart TUI to apply", ToastWarning))
	} else {
		m.cfg = cfg
		m.refreshDelegate() // tui.list_secondary and density settings
	}
	return m, nil
}

func (m Model) handleRenameComplete(msg renameCompleteMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	m.popToRoot()
	m.renameState = nil
	if msg.err != nil {
		m.toast.Show(NewToast(fmt.Sprintf("Rename failed: %s", msg.err), ToastError))
	} else {
		m.toast.Show(NewToast(fmt.Sprintf("Renamed %q to %q", msg.oldName, msg.newName), ToastSuccess))
		if m.pins.slotOf(msg.oldName) != "" {
			old, renamed := msg.oldName, msg.newName
			m.changePins(func(p pinSet) pinSet { return p.rename(old, renamed) })
		}
		m.pendingSelect = msg.newName
		cmds = append(cmds, m.spinner.Tick)
	}
	cmds = append(cmds, m.fetchWorktrees)
	return m, tea.Batch(cmds...)
}

func (m Model) handleCheckoutComplete(msg checkoutCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m.finishFormWithError(msg.err)
	}
	m.popToRoot()
	m.branchForm = nil
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
	if m.activeView() != ViewDashboard {
		return m.handleKey(msg)
	}

	if m.list.FilterState() == list.Filtering {
		m.pendingG = false
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
	if m.layoutView() == ViewConfig && m.configState != nil && m.configState.Form != nil {
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
		if m.branchForm != nil {
			return m.branchForm, "", func() { m.branchForm = nil }
		}
	}
	return nil, "", nil
}

func (m *Model) updateLayout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	headerHeight := 1 // status bar
	footerHeight := m.helpFooter.CompactHeight(m.layoutView(), m.width-4) + m.stripHeight()
	available := m.height - headerHeight - footerHeight

	contentWidth := m.width - 2 // body padding (1 char each side)

	listHeaderHeight := 0

	useSideBySide := isWideLayout(m.width)
	if useSideBySide {
		// List gets 60% of content width, detail gets the remainder
		listWidth := contentWidth * 60 / 100
		dividerWidth := 3 // " │ "
		detailWidth := contentWidth - listWidth - dividerWidth
		m.list.SetSize(listWidth, available-listHeaderHeight)
		m.detail.SetWidth(detailWidth)
		m.detail.SetHeight(available)
	} else {
		// The list takes everything but the selection line. The detail
		// viewport is sized for the full-screen sheet.
		m.list.SetSize(contentWidth, max(1, available-1))
		m.detail.SetWidth(contentWidth)
		m.detail.SetHeight(max(3, available))
	}

	m.refreshDelegate()

	m.help.SetWidth(contentWidth)
}

// toggleCompactMode switches between V1 (compact) and V2 (two-line) delegates.
func (m *Model) toggleCompactMode() {
	m.compactMode = !m.compactMode

	// Persist preference (best-effort, don't block on errors)
	newVal := m.compactMode
	if err := updateUIPrefs(m.projectRoot, func(p *UIPrefs) { p.CompactMode = &newVal }); err != nil {
		tuilog.Printf("warning: failed to save UI prefs: %v", err)
	}

	m.refreshDelegate()
	m.updateLayout()
	m.updateDetailContent()
}

// startupCompact decides the starting density: a saved preference, else the
// tui.compact_list config, else one-line rows (spec 5.4).
func startupCompact(config, pref *bool) bool {
	switch {
	case pref != nil:
		return *pref
	case config != nil:
		return *config
	}
	return true
}

// listSecondary is the text after the alias: "branch" (default), "dir", or
// "off" (tui.list_secondary).
func listSecondary(cfg *config.Config) string {
	if cfg == nil || cfg.TUI.ListSecondary == "" {
		return "branch"
	}
	return cfg.TUI.ListSecondary
}

// rowDelegate is the dashboard row delegate for the current settings.
func (m *Model) rowDelegate() rowDelegate {
	pinOf := map[string]string{}
	for slot, alias := range m.pins {
		pinOf[alias] = slot
	}
	return rowDelegate{
		TwoLine:    !m.compactMode,
		Pins:       len(m.pins) > 0,
		PinOf:      pinOf,
		LastActive: m.lastActive,
		AgentAware: m.agentAware,
		Secondary:  listSecondary(m.cfg),
		Now:        m.now,
	}
}

// refreshDelegate installs the row delegate for the current settings.
func (m *Model) refreshDelegate() {
	m.list.SetDelegate(m.rowDelegate())
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
	return (m.branchForm != nil && m.branchForm.Busy) ||
		(m.syncState != nil && m.syncState.Syncing) ||
		(m.deleteState != nil && m.deleteState.Deleting) ||
		(m.renameState != nil && m.renameState.Renaming)
}

// isTrackerAnimating reports whether PR or issue state is loading/creating.
func (m Model) isTrackerAnimating() bool {
	return (m.prState != nil && m.prState.Loading) ||
		(m.issueState != nil && m.issueState.Loading)
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
	// A pending g or m owns the next key, before help or any other
	// dashboard key sees it.
	if m.activeView() == ViewDashboard && !m.detailFocused {
		if m.pendingG {
			if updated, cmd, handled := m.handleGLayerKey(msg); handled {
				return updated, cmd
			}
		}
		if m.pendingM {
			if updated, cmd, handled := m.handlePinStripKey(msg); handled {
				return updated, cmd
			}
		}
	}

	// Help and upgrade frames take every key. esc closes either, and ? also
	// closes help; both return to the frame beneath.
	switch m.activeView() {
	case ViewHelp:
		if key.Matches(msg, m.keys.Help) || key.Matches(msg, m.keys.Escape) {
			m.popView()
			return m, nil
		}
		var cmd tea.Cmd
		m.helpOverlay, cmd = m.helpOverlay.Update(msg)
		return m, cmd
	case ViewUpdate:
		if key.Matches(msg, m.keys.Escape) {
			m.popView()
		}
		return m, nil
	}

	// ? opens help for views without text input
	// Views with active text inputs (Create, Rename, Checkout, Fork) pass ? through
	if key.Matches(msg, m.keys.Help) && !m.viewHasTextInput() {
		if m.activeView() == ViewDashboard && !m.detailFocused {
			// On the dashboard, ? is omni scoped to actions (spec 9.1).
			return m.openOmni(omniScopeActions)
		}
		m.helpOverlay.Open(m.activeView(), m.width, m.height)
		m.pushView(ViewHelp)
		return m, nil
	}

	switch m.activeView() {
	case ViewDelete:
		return m.handleDeleteKey(msg)
	case ViewBranchForm:
		return m.handleBranchFormKey(msg)
	case ViewBulk:
		return m.handleBulkKey(msg)
	case ViewPRs:
		return m.handlePRKey(msg)
	case ViewIssues:
		return m.handleIssueKey(msg)
	case ViewSync:
		return m.handleSyncKey(msg)
	case ViewConfig:
		return m.handleConfigKey(msg)
	case ViewRename:
		return m.handleRenameKey(msg)
	case ViewOmni:
		return m.handleOmniKey(msg)
	case ViewMessages:
		return m.handleMessagesKey(msg)
	case ViewDetailSheet:
		return m.handleDetailSheetKey(msg)
	case ViewDashboard:
		return m.handleDashboardKey(msg)
	}
	return m, nil
}

func (m Model) handleDashboardKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.detailFocused {
		return m.handleDashboardDetailKey(msg)
	}
	// The action table decides (spec 15.4). Keys it gives the list, and
	// keys it does not know, go to the list.
	k := msg.String()
	a, ok := findAction("", k)
	if !ok || a.Run == nil {
		return m.routeToList(msg)
	}
	if a.Enabled != nil && !a.Enabled(m) {
		return m, nil
	}
	return a.Run(m, k)
}

// handleGLayerKey takes the key after g (spec 6.3): an action from the g
// strip, or anything else, which only closes the strip. ctrl+c still quits.
func (m Model) handleGLayerKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	k := msg.String()
	m.pendingG = false
	m.updateLayout()
	if k == "ctrl+c" {
		return m, nil, false
	}
	if a, ok := findAction("g", k); ok && a.Run != nil {
		if a.Enabled != nil && !a.Enabled(m) {
			return m, nil, true
		}
		next, cmd := a.Run(m, k)
		return next.(Model), cmd, true
	}
	return m, nil, true
}

// halfPage is half the rows the list shows per page, at least 1.
func (m Model) halfPage() int {
	return max(1, m.list.Paginator.PerPage/2)
}

// moveCursorTo selects visible row i, clamped to the list.
func (m *Model) moveCursorTo(i int) {
	n := len(m.list.VisibleItems())
	if n == 0 {
		return
	}
	i = max(0, min(i, n-1))
	if i != m.list.Index() {
		m.list.Select(i)
		m.updateDetailContent()
	}
}

func (m Model) handleDashboardDetailKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Escape) || key.Matches(msg, m.keys.Tab) || msg.String() == "h" {
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
	return m.openBranchForm(branchFormPrefill{})
}

// worktreeNamePattern returns the configured worktree naming pattern for
// directory-name previews. Empty when config is unavailable — interpolation
// falls back to the default pattern.
func (m Model) worktreeNamePattern() string {
	if m.cfg == nil {
		return ""
	}
	return m.cfg.Naming.Pattern
}

func (m Model) handleDashboardDeleteKey() (tea.Model, tea.Cmd) {
	item, ok := m.selectedItem()
	if ok && !item.IsMain && !item.IsProtected {
		m.pushView(ViewDelete)
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
	return m.openCopyForm(item)
}

func (m Model) handleDashboardRenameKey() (tea.Model, tea.Cmd) {
	item, ok := m.selectedItem()
	if ok && !item.IsMain && !item.IsProtected {
		m.pushView(ViewRename)
		m.renameState = NewRenameState(&item)
		return m, m.renameState.Input.Focus()
	}
	return m, nil
}

func (m Model) handleDashboardCheckoutKey() (tea.Model, tea.Cmd) {
	item, ok := m.selectedItem()
	if ok && !item.IsMain && !item.IsProtected {
		return m.openSwitchForm(item)
	}
	return m, nil
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
		m.popView()
		return m, nil
	}

	if m.deleteState.Deleting {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Confirm):
		if m.deleteState.Item == nil {
			m.popView()
			m.deleteState = nil
			return m, nil
		}
		name := m.deleteState.Item.ShortName
		m.deleteState.Deleting = true
		return m, tea.Batch(m.spinner.Tick, deleteWorktreeCmd(m.worktreeMgr, m.stateMgr, m.cfg, m.projectRoot, name, m.deleteState.DeleteBranch))

	case key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.Deny):
		m.popView()
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
		m.popView()
		return m, nil
	}

	if m.renameState.Renaming {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Escape):
		m.popView()
		m.renameState = nil
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		newName := m.renameState.Input.Value()
		if newName == "" {
			m.renameState.Error = "name cannot be empty"
			return m, nil
		}
		if errMsg := ValidateWorktreeName(newName); errMsg != "" {
			m.renameState.Error = errMsg
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

func (m *Model) applySortToList() tea.Cmd {
	sorted := sortWorktreeItems(m.list.Items(), m.sortMode, m.sortReversed)
	cmd := m.list.SetItems(sorted)
	m.refreshDelegate()
	m.updateDetailContent()
	return cmd
}

func (m Model) enterPRView() (tea.Model, tea.Cmd) {
	branches := m.worktreeBranchMap()
	m.pushView(ViewPRs)
	m.prState = &PRViewState{
		Loading:          true,
		WorktreeBranches: branches,
		FilterInput:      newFilterInput(""),
	}
	return m, tea.Batch(m.spinner.Tick, m.fetchPRsCmd)
}

func (m Model) enterIssueView() (tea.Model, tea.Cmd) {
	branches := m.worktreeBranchMap()
	m.pushView(ViewIssues)
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

	m.pushView(ViewBulk)
	m.bulkState = &BulkState{
		Items:    deletable,
		Selected: make([]bool, len(deletable)),
	}
	return m, nil
}

func (m Model) handleBulkKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.bulkState == nil {
		m.popView()
		return m, nil
	}

	s := m.bulkState

	if s.Deleting {
		// Ignore input while deleting
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Escape):
		m.popView()
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
			result := deleteWorktreeCmd(m.worktreeMgr, m.stateMgr, m.cfg, m.projectRoot, item.ShortName, false)()
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
	if LayoutBreakpoint(m.width) == LayoutGuard {
		// Nothing else is drawn; all state is kept for when it widens.
		return "grove needs ≥44 columns"
	}
	if m.loading {
		return m.viewLoading()
	}
	if len(m.list.Items()) == 0 {
		return m.viewEmpty()
	}

	return m.withToast(m.renderFrame(len(m.views) - 1))
}

// withToast draws the current toast over the top-right corner of a finished
// screen, above any overlay.
func (m Model) withToast(screen string) string {
	if m.toast == nil || m.toast.Current == nil {
		return screen
	}
	box := m.toast.View(m.width)
	return composeLayerAt(screen, box, m.width, m.height, m.width-lipgloss.Width(box), 0)
}

// renderFrame renders frame i of the view stack; -1 is the dashboard root.
// Overlay frames draw over the frame beneath them, so an overlay opened
// from the PR browser shows the PR browser behind it.
func (m Model) renderFrame(i int) string {
	v := ViewDashboard
	if i >= 0 {
		v = m.views[i].view
	}
	switch v {
	case ViewDashboard:
		return m.renderDashboard()
	case ViewPRs:
		if m.prState != nil {
			return m.renderPRPanel()
		}
		return m.renderFrame(i - 1)
	case ViewIssues:
		if m.issueState != nil {
			return m.renderIssuePanel()
		}
		return m.renderFrame(i - 1)
	case ViewDetailSheet:
		return m.renderDetailSheet()
	}
	overlay := m.renderOverlay(v)
	if overlay == "" {
		return m.renderFrame(i - 1)
	}
	if v == ViewBranchForm || v == ViewOmni {
		return composeOverlayAt(m.renderFrame(i-1), overlay, m.width, m.height, formTop)
	}
	return composeOverlay(m.renderFrame(i-1), overlay, m.width, m.height)
}

// renderOverlay returns the panel for an overlay view, or "" when its state
// is gone.
func (m Model) renderOverlay(v ActiveView) string {
	switch v {
	case ViewDelete:
		if m.deleteState != nil {
			return renderDeleteV2(m.deleteState, m.width)
		}
	case ViewBranchForm:
		if m.branchForm != nil {
			return renderBranchForm(m.branchForm, m.width, m.height, m.spinner.View())
		}
	case ViewOmni:
		if m.omni != nil {
			return renderOmni(m.omni, m.width, m.height)
		}
	case ViewMessages:
		return m.renderMessages()
	case ViewBulk:
		if m.bulkState != nil {
			return renderBulk(m.bulkState)
		}
	case ViewSync:
		if m.syncState != nil {
			return renderSync(m.syncState, m.width)
		}
	case ViewConfig:
		if m.configState != nil {
			return renderConfig(m.configState, m.width)
		}
	case ViewRename:
		if m.renameState != nil {
			return renderRename(m.renameState, m.width)
		}
	case ViewHelp:
		return m.helpOverlay.View(m.width, m.height)
	case ViewUpdate:
		if m.updateOverlay != nil {
			return m.updateOverlay.View(m.width, m.height)
		}
	}
	return ""
}

func (m Model) viewNotReady() string {
	tuilog.Printf("View: not ready")
	if m.activeView() == ViewPRs && m.prState != nil {
		return m.spinner.View() + " Loading PRs..."
	}
	if m.activeView() == ViewIssues && m.issueState != nil {
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

// updateAvailable reports whether the cached release info shows a newer
// grove version is available than the running binary.
//
// String inequality is sufficient because the only path that populates
// updateLatestVersion is NewModel → updatecheck.CachedRelease, which already
// applies CompareSemver and only returns a value when the cache is strictly
// newer. Re-running CompareSemver here would be redundant AND would break
// dev/test builds where version.Version is unparseable as semver.
func (m Model) updateAvailable() bool {
	return m.updateLatestVersion != "" && m.updateLatestVersion != version.Version
}

// viewHasTextInput returns true if the active view has a focused text input
// that should receive ? as a character rather than triggering help.
func (m Model) viewHasTextInput() bool {
	switch m.activeView() {
	case ViewBranchForm, ViewRename, ViewOmni:
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
	if strip := m.renderOpenStrip(); strip != "" {
		footer = strip + "\n" + footer
	}

	body = lipgloss.NewStyle().Padding(0, 1).Render(body)
	body = m.clampBodyToHeight(body, footer)
	if body == "" {
		return lipgloss.JoinVertical(lipgloss.Left, statusBar, footer)
	}
	return lipgloss.JoinVertical(lipgloss.Left, statusBar, body, footer)
}

// buildDashboardHeader populates the header from the current worktree list state.
func (m Model) buildDashboardHeader() Header {
	h := Header{
		ProjectName:   m.projectName,
		WorktreeCount: len(m.list.Items()),
		SortLabel:     m.sortMode.String(),
		SortReversed:  m.sortReversed,
		Cursor:        m.list.Index() + 1,
		Total:         len(m.list.VisibleItems()),
	}
	if m.updateNotify && m.updateAvailable() {
		h.Upgrade = m.updateLatestVersion
	}
	if m.agentAware {
		for _, li := range m.list.Items() {
			if item, ok := li.(WorktreeItem); ok {
				switch item.AgentStatus {
				case mux.AgentBlocked:
					h.Blocked++
				case mux.AgentDone:
					h.Done++
				}
			}
		}
	}
	return h
}

// renderDashboardBody renders the list and detail panels in side-by-side or stacked layout.
func (m Model) renderDashboardBody() string {
	if isWideLayout(m.width) {
		return m.renderSideBySideBody()
	}
	return m.renderStackedBody()
}

func (m Model) renderSideBySideBody() string {
	listWidth := m.list.Width()
	listContent := lipgloss.NewStyle().Width(listWidth).Render(m.list.View())
	divider := renderVerticalDivider(m.list.Height(), Colors.SurfaceDim)
	detailView := m.renderDetailPanel()
	return lipgloss.JoinHorizontal(lipgloss.Top, listContent, divider, detailView)
}

func (m Model) renderStackedBody() string {
	bodyWidth := m.width - 2
	return lipgloss.JoinVertical(lipgloss.Left, m.list.View(), m.renderSelectionLine(bodyWidth))
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
	return "" // no room: the body gives way so the screen never overflows
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

	title := " " + Styles.DetailTitle.Render(item.ShortName) + Styles.TextMuted.Render(" · focused") + " "
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
	hints = append(hints, Hint{"tab", "list"}, Hint{"esc", "list"})
	return hints
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
	m.pushView(ViewPRs)
	m.prState = &PRViewState{
		Loading:          true,
		WorktreeBranches: m.worktreeBranchMap(),
		FilterInput:      newFilterInput(""),
	}
	return m
}

// ConfigureForIssues configures the model to start directly in the issue browser view.
func (m Model) ConfigureForIssues() Model {
	m.pushView(ViewIssues)
	m.issueState = &IssueViewState{
		Loading:          true,
		FilterInput:      newFilterInput(""),
		WorktreeBranches: m.worktreeBranchMap(),
	}
	return m
}

// RunPRs starts the TUI directly in the PR browser view.
func RunPRs(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string, pluginMgr ...*plugins.Manager) (string, bool, error) {
	tuilog.Init()
	defer tuilog.Close()

	maybeRefreshUpdateCache()
	model := NewModel(mgr, stateMgr, projectRoot, pluginMgr...).ConfigureForPRs()
	return runModel(model)
}

// RunIssues starts the TUI directly in the issue browser view.
func RunIssues(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string, pluginMgr ...*plugins.Manager) (string, bool, error) {
	tuilog.Init()
	defer tuilog.Close()

	maybeRefreshUpdateCache()
	model := NewModel(mgr, stateMgr, projectRoot, pluginMgr...).ConfigureForIssues()
	return runModel(model)
}

// Run starts the TUI and returns the path to switch to (if any) and whether to force docker up.
func Run(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot string, pluginMgr ...*plugins.Manager) (string, bool, error) {
	tuilog.Init()
	defer tuilog.Close()

	maybeRefreshUpdateCache()
	model := NewModel(mgr, stateMgr, projectRoot, pluginMgr...)
	return runModel(model)
}

// maybeRefreshUpdateCache triggers an async refresh of the update-check cache
// from the TUI startup path, honoring the same Skip rules as the CLI surface
// (CI, GROVE_AGENT_MODE, GROVE_NO_UPDATE_NOTIFIER, non-TTY, dev/unknown
// versions). The 24h interval inside RefreshAsync prevents repeated network
// hits.
func maybeRefreshUpdateCache() {
	if updatecheck.Skip(false, version.Version) {
		return
	}
	updatecheck.RefreshAsync()
}

// handleTmuxSwitch creates/switches to the multiplexer session for the target
// worktree. Returns true if a session switch happened (caller should skip cd).
func (m *Model) handleTmuxSwitch(switchPath string) bool {
	tmuxMode := "auto"
	if m.cfg != nil && m.cfg.Tmux.Mode != "" {
		tmuxMode = m.cfg.Tmux.Mode
	}

	mx := muxFor(m.cfg)
	if tmuxMode == "off" || sessionsSuppressed(m.cfg) || !mx.Available() || m.switchToDisplayName == "" {
		return false
	}

	target := muxTargetFor(m.projectName, m.projectRoot, m.switchToDisplayName, switchPath)
	tuilog.Printf("handleTmuxSwitch: session=%q displayName=%q", target.Name, m.switchToDisplayName)

	// Store current session as last before switching
	if mx.Inside() {
		if currentSession, err := mx.Current(); err == nil {
			_ = mux.StoreLastSession(currentSession)
		}
	}

	// Create session if it doesn't exist
	exists, err := mx.Exists(target)
	if err != nil {
		tuilog.Printf("warning: failed to check session %q: %v", target.Name, err)
		return false
	}
	if !exists {
		if err := mx.Ensure(target); err != nil {
			// A target the backend declines (herdr with no usable server, a
			// path it cannot open, or a worktree already open in another
			// session) has no session, so returning false is the answer: the
			// caller falls back to changing directory. When the backend says
			// why is worth knowing, say it — this runs after the dashboard has
			// exited and restored the terminal, so stderr is visible.
			if !mux.ErrUnmanaged(err) {
				tuilog.Printf("warning: failed to create session %q: %v", target.Name, err)
			} else if hint := mux.DegradedHint(err); hint != "" {
				cli.Warning(cli.NewStderr(), "%s", hint)
			}
			return false
		}
	}

	// Switch or attach
	if mx.Inside() {
		if err := mx.Switch(target); err != nil {
			tuilog.Printf("warning: failed to switch session: %v", err)
			return false
		}
		return true
	}

	var controlMode *bool
	if m.cfg != nil {
		controlMode = m.cfg.Tmux.ControlMode
	}
	if cm, ok := mx.(mux.ControlModer); ok && cm.UseControlMode(controlMode) {
		if err := cm.AttachControlMode(target); err != nil {
			tuilog.Printf("warning: failed to attach session: %v", err)
			return false
		}
		return true
	}
	if err := mx.Attach(target); err != nil {
		tuilog.Printf("warning: failed to attach session: %v", err)
		return false
	}
	return true
}

func runModel(model Model) (string, bool, error) {
	tuilog.Printf("runModel: projectRoot=%s activeView=%d", model.projectRoot, model.activeView())

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
			// Route through cli.CdDirective: it prefers a pre-existing
			// GROVE_CD_FILE (never recreating a stale one — B27 hardening) and
			// falls back to a cd: line on stdout under GROVE_SHELL; a cd-file
			// write failure degrades to that fallback instead of failing
			// runModel. When the stdout fallback is the channel in play, print
			// a leading newline first so the directive lands on its own line,
			// separated from any bubbletea alt-screen exit escape codes that
			// may precede it on stdout.
			if os.Getenv("GROVE_CD_FILE") == "" && cli.IsShellIntegration() {
				fmt.Println()
			}
			if cli.CdDirective(switchPath) {
				tuilog.Printf("runModel: emitted cd directive for switchTo=%q", switchPath)
			} else {
				tuilog.Printf("runModel: switchTo=%q but no usable GROVE_CD_FILE or GROVE_SHELL — cannot switch", switchPath)
			}
		}
	}

	return switchPath, m.SwitchForceUp(), nil
}
