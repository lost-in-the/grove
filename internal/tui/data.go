package tui

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

// WorktreeItem holds enriched worktree data for the TUI.
type WorktreeItem struct {
	ShortName     string
	FullName      string
	Path          string
	Branch        string
	Commit        string
	CommitMessage string
	CommitAge     string
	IsDirty       bool
	DirtyFiles    []string
	IsMain        bool
	IsCurrent     bool
	IsEnvironment bool
	IsProtected   bool
	IsPrunable    bool
	TmuxStatus    string // "attached", "detached", "none"
	AheadCount    int    // commits ahead of upstream
	BehindCount   int    // commits behind upstream
	LastAccessed  time.Time
}

// list.Item interface implementation for bubbles/list.
func (w WorktreeItem) Title() string       { return w.ShortName }
func (w WorktreeItem) Description() string { return w.Branch }
func (w WorktreeItem) FilterValue() string { return w.ShortName + " " + w.Branch }

// StatusText returns a display string for git status.
func (w *WorktreeItem) StatusText() string {
	if w.IsPrunable {
		return Theme.StatusStale.Render("✗ stale")
	}
	if w.IsDirty {
		return Theme.StatusDirty.Render("● dirty")
	}
	return Theme.StatusClean.Render("✓ clean")
}

// TmuxText returns a display string for tmux status.
func (w *WorktreeItem) TmuxText() string {
	switch w.TmuxStatus {
	case "attached":
		return Theme.TmuxBadge.Render("⬡ attached")
	case "detached":
		return Theme.TmuxBadge.Render("⬡ tmux")
	default:
		return ""
	}
}

// AgeText returns a compact age string from CommitAge.
func (w *WorktreeItem) AgeText() string {
	if w.CommitAge == "" {
		return ""
	}
	return Theme.DetailDim.Render(w.CommitAge)
}

// FetchWorktrees gathers all enriched worktree data for display.
func FetchWorktrees(mgr *worktree.Manager, stateMgr *state.Manager) ([]WorktreeItem, error) {
	trees, err := mgr.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	projectName := mgr.GetProjectName()

	// Load config for protection checks
	cfg, _ := config.Load()

	// Current worktree
	currentTree, _ := mgr.GetCurrent()
	var currentPath string
	if currentTree != nil {
		currentPath = currentTree.Path
	}

	// Tmux sessions
	var sessions map[string]*tmux.Session
	if tmux.IsTmuxAvailable() {
		sessionList, err := tmux.ListSessions()
		if err == nil {
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
				if err == nil {
					item.Commit = shortHash
					item.CommitMessage = message
					item.CommitAge = age
				}

				if isDirty {
					dirtyFiles, err := mgr.GetDirtyFiles(treePath)
					if err == nil && dirtyFiles != "" {
						for _, f := range strings.Split(dirtyFiles, "\n") {
							if f != "" {
								item.DirtyFiles = append(item.DirtyFiles, f)
							}
						}
					}
				}

				if isCurrent {
					ahead, behind := getUpstreamCounts(treePath)
					item.AheadCount = ahead
					item.BehindCount = behind
				}
			}(item, tree.Path, tree.IsDirty, item.IsCurrent)
		}
	}
	wg.Wait()

	return items, nil
}

// getUpstreamCounts returns (ahead, behind) commit counts relative to the
// upstream tracking branch. Returns (0, 0) when there is no upstream or on
// any error (detached HEAD, local-only branch, etc.).
func getUpstreamCounts(worktreePath string) (ahead, behind int) {
	cmd := exec.Command("git", "rev-list", "--count", "--left-right", "@{upstream}...HEAD")
	cmd.Dir = worktreePath
	out, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) != 2 {
		return 0, 0
	}

	b, _ := strconv.Atoi(parts[0])
	a, _ := strconv.Atoi(parts[1])
	return a, b
}
