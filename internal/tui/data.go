package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/plugins"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/tuilog"
	"github.com/lost-in-the/grove/internal/worktree"
)

// PRInfo holds lightweight PR metadata for display in the detail panel.
type PRInfo struct {
	Number         int
	Title          string
	URL            string
	ReviewDecision string // "APPROVED", "CHANGES_REQUESTED", "REVIEW_REQUIRED", ""
}

// RecentCommit holds a short SHA and subject for display.
type RecentCommit struct {
	SHA     string // 7-char short hash
	Message string // first line of commit message
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
	TmuxStatus     string         // "attached", "detached", "none"
	HasRemote      bool           // true if branch has upstream tracking
	TrackingBranch string         // e.g., "origin/feat/ux-polish" (empty if no upstream)
	AheadCount     int            // commits ahead of upstream
	BehindCount    int            // commits behind upstream
	CommitCount    int            // commits ahead of default branch
	RecentCommits  []RecentCommit // last 3 commits
	StashCount     int            // number of stashes in this worktree
	AssociatedPR   *PRInfo        // PR linked to this branch (nil if none)
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

// getUpstreamInfo returns upstream tracking info in a single git call.
// If no upstream is configured, hasRemote is false and counts are 0.
func getUpstreamInfo(worktreePath string) (ahead, behind int, hasRemote bool, trackingBranch string) {
	out, err := cmdexec.Output(context.TODO(), "git", []string{"rev-list", "--count", "--left-right", "@{upstream}...HEAD"}, worktreePath, cmdexec.GitLocal)
	if err != nil {
		return 0, 0, false, ""
	}

	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) != 2 {
		return 0, 0, false, ""
	}

	b, _ := strconv.Atoi(parts[0])
	a, _ := strconv.Atoi(parts[1])

	// Get the tracking branch name
	tbOut, tbErr := cmdexec.Output(context.TODO(), "git", []string{"rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"}, worktreePath, cmdexec.GitLocal)
	if tbErr == nil {
		trackingBranch = strings.TrimSpace(string(tbOut))
	}

	return a, b, true, trackingBranch
}

// getCommitCountAhead returns the number of commits a worktree branch is ahead
// of the default branch. Returns 0 if on the default branch or on error.
func getCommitCountAhead(worktreePath, defaultBranch string) int {
	out, err := cmdexec.Output(context.TODO(), "git", []string{"rev-list", "--count", defaultBranch + "..HEAD"}, worktreePath, cmdexec.GitLocal)
	if err != nil {
		return 0
	}
	count, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return count
}

// getRecentCommits returns the last n commits (SHA + subject) for a worktree.
func getRecentCommits(worktreePath string, n int) []RecentCommit {
	out, err := cmdexec.Output(context.TODO(), "git", []string{"log", fmt.Sprintf("-%d", n), "--format=%h %s"}, worktreePath, cmdexec.GitLocal)
	if err != nil {
		return nil
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	commits := make([]RecentCommit, 0, len(lines))
	for _, line := range lines {
		if sha, msg, ok := strings.Cut(line, " "); ok {
			commits = append(commits, RecentCommit{SHA: sha, Message: msg})
		}
	}
	return commits
}

// getStashCount returns the number of stashes in a worktree.
func getStashCount(worktreePath string) int {
	out, err := cmdexec.Output(context.TODO(), "git", []string{"stash", "list"}, worktreePath, cmdexec.GitLocal)
	if err != nil {
		return 0
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return 0
	}
	return len(strings.Split(raw, "\n"))
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
}

func loadFetchContext(mgr *worktree.Manager, stateMgr *state.Manager) fetchContext {
	fc := fetchContext{
		projectName:   mgr.GetProjectName(),
		defaultBranch: "main",
		stateMgr:      stateMgr,
		mgr:           mgr,
	}

	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		tuilog.Printf("warning: failed to load config for protection checks: %v", cfgErr)
	}
	fc.cfg = cfg
	if cfg != nil && cfg.DefaultBranch != "" {
		fc.defaultBranch = cfg.DefaultBranch
	}

	currentTree, currentErr := mgr.GetCurrent()
	if currentErr != nil {
		tuilog.Printf("warning: failed to get current worktree: %v", currentErr)
	}
	if currentTree != nil {
		fc.currentPath = currentTree.Path
	}

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

func (fc *fetchContext) enrichGitInfo(item *WorktreeItem, treePath, branch string, isDirty bool) {
	shortHash, message, age, err := fc.mgr.GetCommitInfo(treePath)
	if err != nil {
		tuilog.Printf("warning: failed to get commit info for %q: %v", treePath, err)
	} else {
		item.Commit = shortHash
		item.CommitMessage = message
		item.CommitAge = age
	}

	if isDirty {
		dirtyFiles, err := fc.mgr.GetDirtyFiles(treePath)
		if err != nil {
			tuilog.Printf("warning: failed to get dirty files for %q: %v", treePath, err)
		} else if dirtyFiles != "" {
			for _, f := range strings.Split(dirtyFiles, "\n") {
				if f != "" {
					item.DirtyFiles = append(item.DirtyFiles, f)
				}
			}
		}
	}

	item.AheadCount, item.BehindCount, item.HasRemote, item.TrackingBranch = getUpstreamInfo(treePath)

	if branch != fc.defaultBranch {
		item.CommitCount = getCommitCountAhead(treePath, fc.defaultBranch)
	}

	item.RecentCommits = getRecentCommits(treePath, 3)
	item.StashCount = getStashCount(treePath)
}

// FetchWorktrees gathers all enriched worktree data for display.
// pluginMgr is optional — pass nil to skip plugin status collection.
func FetchWorktrees(mgr *worktree.Manager, stateMgr *state.Manager, pluginMgr ...*plugins.Manager) ([]WorktreeItem, error) {
	trees, err := mgr.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	fc := loadFetchContext(mgr, stateMgr)
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
			go func(item *WorktreeItem, treePath, branch string, isDirty bool) {
				defer wg.Done()
				fc.enrichGitInfo(item, treePath, branch, isDirty)
			}(item, tree.Path, tree.Branch, tree.IsDirty)
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
