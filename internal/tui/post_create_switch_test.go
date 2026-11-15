package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/worktree"
)

// The dashboard stays open after creating or forking a worktree and only
// selects it in the list, so post-create hooks must not treat the new
// worktree as the one the user moved into.
func TestDashboardCreate_PostCreateHookIsNonSwitching(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	repo := filepath.Join(root, "myapp")
	groveDir := filepath.Join(repo, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mgr, err := worktree.NewManager(repo)
	if err != nil {
		t.Fatal(err)
	}
	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		t.Fatal(err)
	}
	wtPath := mgr.PathForName("feature")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	var got *hooks.Context
	hooks.Register(hooks.EventPostCreate, func(c *hooks.Context) error {
		if c.WorktreePath == wtPath {
			got = c
		}
		return nil
	})

	ch := make(chan creationEvent, 100)
	go func() {
		for range ch {
		}
	}()
	defer close(ch)

	cfg := &config.Config{}
	wt := &worktree.Worktree{Name: filepath.Base(wtPath), Path: wtPath, Branch: "feature", ShortName: "feature"}
	runPostCreateStreaming(ch, mgr, stateMgr, cfg, repo, "feature", wt)

	if got == nil {
		t.Fatal("post-create hook did not fire")
	}
	if got.WillSwitch {
		t.Error("dashboard create reported WillSwitch=true, but the dashboard does not move the user")
	}
}
