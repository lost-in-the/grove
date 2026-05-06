//go:build !race

package commands

// BenchmarkListWorktrees exercises the grove ls table-rendering path with N=20
// synthetic worktrees to net regressions against the <500ms target.
//
// This benchmark covers the pure-Go output formatting cost (cli.Table construction,
// width computation, row rendering) that runs after the worktree list is loaded.
// The git I/O layer (worktree.Manager.List, dirty checks) is not exercised here
// because it requires a fixture git repository with actual worktrees on disk.
//
// To add full end-to-end coverage: build a git repo in t.TempDir(), run
// "git worktree add" 20 times, call worktree.NewManager, and invoke lsCmd.RunE.
// That test would live in a file tagged //go:build integration.
//
// Run with:
//
//	go test ./cmd/grove/commands/ -bench=BenchmarkListWorktrees -benchmem -run=^$

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
)

func BenchmarkListWorktrees(b *testing.B) {
	const n = 20

	// Pre-build synthetic worktree rows (project-name, branch, status, tmux, path).
	type row struct {
		indicator, name, branch, status, tmux, path string
	}
	rows := make([]row, n)
	for i := range rows {
		rows[i] = row{
			indicator: "",
			name:      fmt.Sprintf("myapp-feature-%02d", i+1),
			branch:    fmt.Sprintf("feat/feature-%02d", i+1),
			status:    statusClean,
			tmux:      tmuxStatusDetached,
			path:      fmt.Sprintf("/home/user/projects/myapp-feature-%02d", i+1),
		}
	}
	// Mark one as current and one as dirty to exercise all code paths.
	rows[0].indicator = "●"
	rows[1].status = statusDirty
	rows[2].tmux = tmuxStatusAttached

	columns := []cli.Column{
		{Title: "", MinWidth: 2, MaxWidth: 2},
		{Title: "NAME", MaxWidth: 30},
		{Title: "BRANCH", MaxWidth: 25},
		{Title: "STATUS", MinWidth: 10},
		{Title: "TMUX", MinWidth: 12},
		{Title: "PATH"},
	}

	b.ResetTimer()
	for range b.N {
		var buf bytes.Buffer
		w := cli.NewWriter(&buf, false)
		tbl := cli.NewTable(w, columns...)
		for _, r := range rows {
			tbl.AddRow(r.indicator, r.name, r.branch, r.status, r.tmux, r.path)
		}
		tbl.Render()
	}
}
