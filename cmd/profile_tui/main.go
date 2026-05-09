// cmd/profile_tui exercises the TUI fetch hot path against a real grove
// project and reports timing for each phase.
//
// Usage: go run ./cmd/profile_tui /path/to/grove/project
//
// Output reports:
//   - per-phase timing for the FetchWorktrees call path
//   - drilldown timings for List(), GetCurrent(), tmux, and plugin status
//
// Use to validate performance changes against a real, populated repo
// (e.g. one with many worktrees) since unit tests run on tiny synthetic
// repos that hide cumulative subprocess cost.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/plugins"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/tui"
	"github.com/lost-in-the/grove/internal/worktree"
	docker "github.com/lost-in-the/grove/plugins/docker"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: profile_tui <project-dir>")
		os.Exit(2)
	}
	dir := os.Args[1]
	if err := os.Chdir(dir); err != nil {
		fail("chdir: %v", err)
	}

	t0 := time.Now()
	groveDir, err := grove.IsGroveProject()
	if err != nil || groveDir == "" {
		fail("not a grove project: %v", err)
	}
	projectRoot := grove.MustProjectRoot(groveDir)
	step(t0, "detected grove project")

	t1 := time.Now()
	mgr, err := worktree.NewManager(projectRoot)
	if err != nil {
		fail("worktree.NewManager: %v", err)
	}
	step(t1, "worktree.NewManager")

	t2 := time.Now()
	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		fail("state.NewManager: %v", err)
	}
	step(t2, "state.NewManager")

	t3 := time.Now()
	cfg, _ := config.LoadFromGroveDir(groveDir)
	step(t3, "config.LoadFromGroveDir")

	pluginMgr := plugins.NewManager(cfg)
	dockerPlugin := docker.New()
	if err := dockerPlugin.Init(cfg); err == nil {
		_ = pluginMgr.Register(dockerPlugin)
	}

	t4 := time.Now()
	items, err := tui.FetchWorktrees(mgr, stateMgr, pluginMgr)
	if err != nil {
		fail("tui.FetchWorktrees: %v", err)
	}
	step(t4, fmt.Sprintf("tui.FetchWorktrees -> %d items", len(items)))

	fmt.Printf("\nTotal: %dms\n", time.Since(t0).Milliseconds())

	fmt.Println("\n--- drilldown ---")
	mgr2, _ := worktree.NewManager(projectRoot)

	t5 := time.Now()
	trees, _ := mgr2.List()
	step(t5, fmt.Sprintf("mgr.List() -> %d trees", len(trees)))

	t6 := time.Now()
	_, _ = mgr2.GetCurrent()
	step(t6, "mgr.GetCurrent()")

	t7 := time.Now()
	_ = tmux.IsTmuxAvailable()
	step(t7, "tmux.IsTmuxAvailable")

	t8 := time.Now()
	sessions, _ := tmux.ListSessions()
	step(t8, fmt.Sprintf("tmux.ListSessions -> %d sessions", len(sessions)))

	t9 := time.Now()
	paths := make([]string, len(trees))
	for i, t := range trees {
		paths[i] = t.Path
	}
	statuses := pluginMgr.CollectStatuses(paths)
	step(t9, fmt.Sprintf("pluginMgr.CollectStatuses -> %d statuses", len(statuses)))
}

func step(t time.Time, label string) {
	fmt.Printf("[%6dms] %s\n", time.Since(t).Milliseconds(), label)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
