package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LeahArmstrong/grove-cli/internal/cmdexec"
	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/plugins"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/tuilog"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

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
	TmuxStatus     string // "attached", "detached", "none"
	HasRemote      bool   // true if branch has upstream tracking
	AheadCount     int    // commits ahead of upstream
	BehindCount    int    // commits behind upstream
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

// SyncStatusText returns a compact sync status string for the list view.
func (w *WorktreeItem) SyncStatusText() string {
	if !w.HasRemote {
		return "⚠ no remote"
	}
	if w.AheadCount == 0 && w.BehindCount == 0 {
		return "✓ synced"
	}
	var parts []string
	if w.AheadCount > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", w.AheadCount))
	}
	if w.BehindCount > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", w.BehindCount))
	}
	return strings.Join(parts, " ")
}

// getUpstreamInfo returns upstream tracking info in a single git call.
// If no upstream is configured, hasRemote is false and counts are 0.
func getUpstreamInfo(worktreePath string) (ahead, behind int, hasRemote bool) {
	out, err := cmdexec.Output(context.TODO(), "git", []string{"rev-list", "--count", "--left-right", "@{upstream}...HEAD"}, worktreePath, cmdexec.GitLocal)
	if err != nil {
		return 0, 0, false
	}

	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) != 2 {
		return 0, 0, false
	}

	b, _ := strconv.Atoi(parts[0])
	a, _ := strconv.Atoi(parts[1])
	return a, b, true
}

// FetchWorktrees gathers all enriched worktree data for display.
// pluginMgr is optional — pass nil to skip plugin status collection.
func FetchWorktrees(mgr *worktree.Manager, stateMgr *state.Manager, pluginMgr ...*plugins.Manager) ([]WorktreeItem, error) {
	trees, err := mgr.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	projectName := mgr.GetProjectName()

	// Load config for protection checks
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		tuilog.Printf("warning: failed to load config for protection checks: %v", cfgErr)
	}

	// Current worktree
	currentTree, currentErr := mgr.GetCurrent()
	if currentErr != nil {
		tuilog.Printf("warning: failed to get current worktree: %v", currentErr)
	}
	var currentPath string
	if currentTree != nil {
		currentPath = currentTree.Path
	}

	// Tmux sessions
	var sessions map[string]*tmux.Session
	if tmux.IsTmuxAvailable() {
		sessionList, err := tmux.ListSessions()
		if err != nil {
			tuilog.Printf("warning: failed to list tmux sessions: %v", err)
		} else {
			sessions = make(map[string]*tmux.Session, len(sessionList))
			for _, s := range sessionList {
				sessions[s.Name] = s
			}
		}
	}

	items := make([]WorktreeItem, len(trees))

	// Enrich worktrees in parallel for performance (git calls per tree)
	var wg sync.WaitGroup
	for i, tree := range trees {
		item := &items[i]
		item.ShortName = tree.DisplayName()
		item.FullName = tree.Name
		item.Path = tree.Path
		item.Branch = tree.Branch
		item.IsDirty = tree.IsDirty
		item.IsMain = tree.IsMain
		item.IsCurrent = tree.Path == currentPath
		item.IsPrunable = tree.IsPrunable
		item.TmuxStatus = "none"

		// Tmux status (no git call, fast)
		if sessions != nil {
			sessionName := worktree.TmuxSessionName(projectName, tree.ShortName)
			if s, ok := sessions[sessionName]; ok {
				if s.Attached {
					item.TmuxStatus = "attached"
				} else {
					item.TmuxStatus = "detached"
				}
			} else if s, ok := sessions[tree.Name]; ok {
				if s.Attached {
					item.TmuxStatus = "attached"
				} else {
					item.TmuxStatus = "detached"
				}
			}
		}

		// State info (no git call, fast)
		if stateMgr != nil {
			isEnv, _ := stateMgr.IsEnvironment(tree.ShortName)
			item.IsEnvironment = isEnv

			ws, _ := stateMgr.GetWorktree(tree.ShortName)
			if ws != nil {
				item.LastAccessed = ws.LastAccessedAt
			}
		}

		// Protection check (no git call, fast)
		if cfg != nil {
			item.IsProtected = cfg.IsProtected(tree.ShortName)
		}

		// Parallel: commit info + dirty files + upstream
		if !tree.IsPrunable {
			wg.Add(1)
			go func(item *WorktreeItem, treePath string, isDirty, isCurrent bool) {
				defer wg.Done()

				shortHash, message, age, err := mgr.GetCommitInfo(treePath)
				if err != nil {
					tuilog.Printf("warning: failed to get commit info for %q: %v", treePath, err)
				} else {
					item.Commit = shortHash
					item.CommitMessage = message
					item.CommitAge = age
				}

				if isDirty {
					dirtyFiles, err := mgr.GetDirtyFiles(treePath)
					if err != nil {
						tuilog.Printf("warning: failed to get dirty files for %q: %v", treePath, err)
					}
					if err == nil && dirtyFiles != "" {
						var files []string
						for _, f := range strings.Split(dirtyFiles, "\n") {
							if f != "" {
								files = append(files, f)
							}
						}
						item.DirtyFiles = files
					}
				}

				ahead, behind, hasRemote := getUpstreamInfo(treePath)
				item.HasRemote = hasRemote
				item.AheadCount = ahead
				item.BehindCount = behind
			}(item, tree.Path, tree.IsDirty, item.IsCurrent)
		}
	}

	// Plugin statuses — run in parallel with git enrichment
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

	// Attach plugin statuses to items
	if pluginStatuses != nil {
		for i := range items {
			if entries, ok := pluginStatuses[items[i].Path]; ok {
				items[i].PluginStatuses = entries
			}
		}
	}

	return items, nil
}
