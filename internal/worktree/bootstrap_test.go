package worktree

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/state"
)

func TestBootstrapWorktree_RegistersInState(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	groveDir := filepath.Join(mainDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	wtPath := filepath.Join(tmpDir, "feature")
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		t.Fatalf("state mgr: %v", err)
	}

	cfg := config.LoadDefaults()
	opts := BootstrapOpts{
		Name:         "feature",
		Branch:       "feature",
		WorktreePath: wtPath,
		MainPath:     mainDir,
		ProjectName:  "test-proj",
		Now:          time.Now(),
	}

	if err := BootstrapWorktree(stateMgr, cfg, opts); err != nil {
		t.Fatalf("BootstrapWorktree: %v", err)
	}

	got, err := stateMgr.GetWorktree("feature")
	if err != nil {
		t.Fatalf("GetWorktree: %v", err)
	}
	if got.Path != wtPath {
		t.Errorf("Path: got %q want %q", got.Path, wtPath)
	}
	if got.Branch != "feature" {
		t.Errorf("Branch: got %q want feature", got.Branch)
	}
}

func TestBootstrapWorktree_IdempotentOnSecondCall(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	groveDir := filepath.Join(mainDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	wtPath := filepath.Join(tmpDir, "feature")
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		t.Fatalf("state mgr: %v", err)
	}
	cfg := config.LoadDefaults()
	opts := BootstrapOpts{
		Name:         "feature",
		Branch:       "feature",
		WorktreePath: wtPath,
		MainPath:     mainDir,
		ProjectName:  "test-proj",
		Now:          time.Now(),
	}

	if err := BootstrapWorktree(stateMgr, cfg, opts); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := BootstrapWorktree(stateMgr, cfg, opts); err != nil {
		t.Fatalf("second call should not error: %v", err)
	}
}

func TestBootstrapWorktree_RejectsEmptyPaths(t *testing.T) {
	tests := []struct {
		name string
		opts BootstrapOpts
	}{
		{"empty WorktreePath", BootstrapOpts{MainPath: "/tmp/main"}},
		{"empty MainPath", BootstrapOpts{WorktreePath: "/tmp/wt"}},
		{"both empty", BootstrapOpts{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := BootstrapWorktree(nil, nil, tt.opts)
			if err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}
