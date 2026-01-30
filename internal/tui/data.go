package tui

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
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

	items := make([]WorktreeItem, 0, len(trees))
	for _, tree := range trees {
		item := WorktreeItem{
			ShortName:  tree.DisplayName(),
			FullName:   tree.Name,
			Path:       tree.Path,
			Branch:     tree.Branch,
			IsDirty:    tree.IsDirty,
			IsMain:     tree.IsMain,
			IsCurrent:  tree.Path == currentPath,
			IsPrunable: tree.IsPrunable,
			TmuxStatus: "none",
		}

		// Enrich with commit info (List() doesn't populate these)
		if !tree.IsPrunable {
			shortHash, message, age, err := mgr.GetCommitInfo(tree.Path)
			if err == nil {
				item.Commit = shortHash
				item.CommitMessage = message
				item.CommitAge = age
			}
		}

		// Dirty files
		if tree.IsDirty && !tree.IsPrunable {
			dirtyFiles, err := mgr.GetDirtyFiles(tree.Path)
			if err == nil && dirtyFiles != "" {
				for _, f := range strings.Split(dirtyFiles, "\n") {
					if f != "" {
						item.DirtyFiles = append(item.DirtyFiles, f)
					}
				}
			}
		}

		// Tmux status
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

		// Upstream tracking status (ahead/behind).
		// Currently only fetched for the active worktree to keep startup
		// fast (~20-30ms per git call). A future configuration option
		// (e.g. tui.fetch_all_upstream = true) could expand this to fetch
		// for every worktree, ideally in parallel goroutines.
		if item.IsCurrent && !tree.IsPrunable {
			ahead, behind := getUpstreamCounts(tree.Path)
			item.AheadCount = ahead
			item.BehindCount = behind
		}

		// State info
		if stateMgr != nil {
			isEnv, _ := stateMgr.IsEnvironment(tree.ShortName)
			item.IsEnvironment = isEnv

			ws, _ := stateMgr.GetWorktree(tree.ShortName)
			if ws != nil {
				item.LastAccessed = ws.LastAccessedAt
			}
		}

		// Protection check
		if cfg != nil {
			item.IsProtected = cfg.IsProtected(tree.ShortName)
		}

		items = append(items, item)
	}

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
