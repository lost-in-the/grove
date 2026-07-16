package tui

import (
	"strings"
	"sync"
	"time"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/plugins"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/tuilog"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/internal/worktreeinfo"
)

// PRInfo holds lightweight PR metadata for display in the detail panel.
type PRInfo struct {
	Number         int
	Title          string
	URL            string
	ReviewDecision string // "APPROVED", "CHANGES_REQUESTED", "REVIEW_REQUIRED", ""
}

// WorktreeItem holds enriched worktree data for the TUI.
type WorktreeItem struct {
	ShortName      string
	FullName       string
	Path           string
	Branch         string
	Commit         string
	CommitMessage  string
	CommitAge      string
	IsDirty        bool
	DirtyFiles     []string
	IsMain         bool
	IsCurrent      bool
	IsEnvironment  bool
	IsProtected    bool
	IsPrunable     bool
	TmuxStatus     string                      // "attached", "detached", "none"
	HasRemote      bool                        // true if branch has upstream tracking
	TrackingBranch string                      // e.g., "origin/feat/ux-polish" (empty if no upstream)
	AheadCount     int                         // commits ahead of upstream
	BehindCount    int                         // commits behind upstream
	CommitCount    int                         // commits ahead of default branch
	RecentCommits  []worktreeinfo.RecentCommit // last 3 commits
	StashCount     int                         // number of stashes in this worktree
	AssociatedPR   *PRInfo                     // PR linked to this branch (nil if none)
	LastAccessed   time.Time
	PluginStatuses []plugins.StatusEntry // status entries from plugins
}

// list.Item interface implementation for bubbles/list.
func (w WorktreeItem) Title() string       { return w.ShortName }
func (w WorktreeItem) Description() string { return w.Branch }
func (w WorktreeItem) FilterValue() string { return w.ShortName + " " + w.Branch }

// displayName returns the name used for tmux session naming.
// Matches worktree.Worktree.DisplayName() logic.
func (w WorktreeItem) displayName() string {
	if w.IsMain {
		return "root"
	}
	return w.ShortName
}

// StatusText returns a display string for git status.
func (w *WorktreeItem) StatusText() string {
	if w.IsPrunable {
		return Styles.StatusStale.Render("✗ stale")
	}
	if w.IsDirty {
		return Styles.StatusDirty.Render("● dirty")
	}
	return Styles.StatusClean.Render("✓ clean")
}

// TmuxText returns a display string for tmux status.
func (w *WorktreeItem) TmuxText() string {
	switch w.TmuxStatus {
	case "attached":
		return Styles.TmuxBadge.Render("⬡ attached")
	case "detached":
		return Styles.TmuxBadge.Render("⬡ tmux")
	default:
		return ""
	}
}

// AgeText returns a compact age string from CommitAge.
func (w *WorktreeItem) AgeText() string {
	if w.CommitAge == "" {
		return ""
	}
	return Styles.DetailDim.Render(w.CommitAge)
}

// fetchContext holds preloaded data used when enriching worktree items.
type fetchContext struct {
	projectName   string
	defaultBranch string
	currentPath   string
	sessions      map[string]*tmux.Session
	cfg           *config.Config
	stateMgr      *state.Manager
	mgr           *worktree.Manager
	upstreams     map[string]worktreeinfo.BranchUpstream // keyed by branch name
}

// ResolveDefaultBranch returns the default branch name to use for "ahead of
// default" calculations: cfg.DefaultBranch when set, otherwise "main". Single
// source of truth for both the bulk fetcher and the lazy metrics dispatcher.
func ResolveDefaultBranch(cfg *config.Config) string {
	if cfg != nil && cfg.DefaultBranch != "" {
		return cfg.DefaultBranch
	}
	return "main"
}

func loadFetchContext(mgr *worktree.Manager, stateMgr *state.Manager) fetchContext {
	fc := fetchContext{
		projectName: mgr.GetProjectName(),
		stateMgr:    stateMgr,
		mgr:         mgr,
	}

	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		tuilog.Printf("warning: failed to load config for protection checks: %v", cfgErr)
	}
	fc.cfg = cfg
	fc.defaultBranch = ResolveDefaultBranch(cfg)

	// Use CurrentPath (single git rev-parse) rather than GetCurrent, which
	// would re-run List() with N parallel git status calls — already done by
	// the caller.
	currentPath, currentErr := mgr.CurrentPath()
	if currentErr != nil {
		tuilog.Printf("warning: failed to get current worktree path: %v", currentErr)
	}
	fc.currentPath = currentPath

	if tmux.IsTmuxAvailable() {
		sessionList, err := tmux.ListSessions()
		if err != nil {
			tuilog.Printf("warning: failed to list tmux sessions: %v", err)
		} else {
			fc.sessions = make(map[string]*tmux.Session, len(sessionList))
			for _, s := range sessionList {
				fc.sessions[s.Name] = s
			}
		}
	}

	return fc
}

func tmuxStatusForSession(s *tmux.Session) string {
	if s.Attached {
		return "attached"
	}
	return "detached"
}

func (fc *fetchContext) setTmuxStatus(item *WorktreeItem, tree worktree.Worktree) {
	if fc.sessions == nil {
		return
	}
	sessionName := worktree.TmuxSessionName(fc.projectName, tree.ShortName)
	if s, ok := fc.sessions[sessionName]; ok {
		item.TmuxStatus = tmuxStatusForSession(s)
	} else if s, ok := fc.sessions[tree.Name]; ok {
		item.TmuxStatus = tmuxStatusForSession(s)
	}
}

func (fc *fetchContext) setStateInfo(item *WorktreeItem, shortName string) {
	if fc.stateMgr == nil {
		return
	}
	isEnv, _ := fc.stateMgr.IsEnvironment(shortName)
	item.IsEnvironment = isEnv

	ws, _ := fc.stateMgr.GetWorktree(shortName)
	if ws != nil {
		item.LastAccessed = ws.LastAccessedAt
	}
}

func (fc *fetchContext) enrichGitInfo(item *WorktreeItem, tree *worktree.Worktree) {
	branch := tree.Branch

	// One git log call returns both the head commit info and recent commits.
	commitInfo, recents := worktreeinfo.HeadAndRecentCommits(tree.Path, 3)
	item.Commit = commitInfo.ShortHash
	item.CommitMessage = commitInfo.Message
	item.CommitAge = commitInfo.Age
	item.RecentCommits = recents

	// Dirty file list was populated by Manager.List(); split it here.
	if tree.DirtyFiles != "" {
		for _, f := range strings.Split(tree.DirtyFiles, "\n") {
			if f != "" {
				item.DirtyFiles = append(item.DirtyFiles, f)
			}
		}
	}

	if up, ok := fc.upstreams[branch]; ok {
		item.AheadCount = up.Ahead
		item.BehindCount = up.Behind
		item.HasRemote = up.HasRemote
		item.TrackingBranch = up.Tracking
	}

	// CommitCount (ahead of default branch) and StashCount are detail-panel
	// only — defer their per-worktree git calls to FetchDetailMetrics so the
	// dashboard renders without paying for ~2N extra git invocations.
}

// DetailMetrics holds the per-worktree numbers that only appear in the detail
// panel: commits ahead of the default branch, and stash count. Computed
// lazily after the dashboard has rendered.
type DetailMetrics struct {
	CommitCount int
	StashCount  int
}

// FetchDetailMetrics loads the per-worktree detail-panel numbers (CommitCount,
// StashCount) for every non-prunable item in parallel. Result is keyed by
// worktree path. Skips worktrees on the default branch for CommitCount.
//
// Run this AFTER the initial FetchWorktrees so the dashboard paint isn't
// blocked on N extra git calls. Takes the existing items rather than
// re-listing — the caller already paid for List() once.
func FetchDetailMetrics(items []WorktreeItem, defaultBranch string) map[string]DetailMetrics {
	result := make(map[string]DetailMetrics, len(items))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, item := range items {
		if item.IsPrunable {
			continue
		}
		wg.Add(1)
		go func(path, branch string) {
			defer wg.Done()
			m := DetailMetrics{
				StashCount: worktreeinfo.StashCount(path),
			}
			if branch != defaultBranch {
				m.CommitCount = worktreeinfo.CommitCountAhead(path, defaultBranch)
			}
			mu.Lock()
			result[path] = m
			mu.Unlock()
		}(item.Path, item.Branch)
	}
	wg.Wait()
	return result
}

// FetchWorktrees gathers all enriched worktree data for display.
// pluginMgr is optional — pass nil to skip plugin status collection.
func FetchWorktrees(mgr *worktree.Manager, stateMgr *state.Manager, pluginMgr ...*plugins.Manager) ([]WorktreeItem, error) {
	trees, err := mgr.List()
	if err != nil {
		// Manager.List already wraps with "failed to list worktrees".
		return nil, err
	}

	fc := loadFetchContext(mgr, stateMgr)

	// Batch upstream tracking info for every local branch in one git call,
	// instead of 2 calls per worktree.
	upstreams, upErr := worktreeinfo.AllBranchUpstreams(mgr.GetRepoRoot())
	if upErr != nil {
		tuilog.Printf("warning: failed to load upstream info: %v", upErr)
	}
	fc.upstreams = upstreams

	items := make([]WorktreeItem, len(trees))

	var wg sync.WaitGroup
	for i, tree := range trees {
		item := &items[i]
		item.ShortName = tree.DisplayName()
		item.FullName = tree.Name
		item.Path = tree.Path
		item.Branch = tree.Branch
		item.IsDirty = tree.IsDirty
		item.IsMain = tree.IsMain
		item.IsCurrent = tree.Path == fc.currentPath
		item.IsPrunable = tree.IsPrunable
		item.TmuxStatus = "none"

		fc.setTmuxStatus(item, *tree)
		fc.setStateInfo(item, tree.ShortName)

		if fc.cfg != nil {
			item.IsProtected = fc.cfg.IsProtected(tree.ShortName)
		}

		if !tree.IsPrunable {
			wg.Add(1)
			go func(item *WorktreeItem, tree *worktree.Worktree) {
				defer wg.Done()
				fc.enrichGitInfo(item, tree)
			}(item, tree)
		}
	}

	var pluginStatuses map[string][]plugins.StatusEntry
	var pm *plugins.Manager
	if len(pluginMgr) > 0 {
		pm = pluginMgr[0]
	}
	if pm != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			paths := make([]string, len(trees))
			for i, t := range trees {
				paths[i] = t.Path
			}
			pluginStatuses = pm.CollectStatuses(paths)
		}()
	}

	wg.Wait()

	if pluginStatuses != nil {
		for i := range items {
			if entries, ok := pluginStatuses[items[i].Path]; ok {
				items[i].PluginStatuses = entries
			}
		}
	}

	return items, nil
}
