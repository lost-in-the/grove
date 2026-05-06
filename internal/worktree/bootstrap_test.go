package worktree

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
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
	}

	if err := BootstrapWorktree(stateMgr, cfg, opts, nil); err != nil {
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
	}

	if err := BootstrapWorktree(stateMgr, cfg, opts, nil); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := BootstrapWorktree(stateMgr, cfg, opts, nil); err != nil {
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
			err := BootstrapWorktree(nil, nil, tt.opts, nil)
			if err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

// TestBootstrapWorktree_HookFailureSurfacesToWriter verifies that a failing
// post-create hook writes a warning to the supplied writer without requiring
// GROVE_LOG=1 to be set.
func TestBootstrapWorktree_HookFailureSurfacesToWriter(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	groveDir := filepath.Join(mainDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatalf("mkdir grove: %v", err)
	}
	wtPath := filepath.Join(tmpDir, "feature")
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}

	// Write a hooks.toml with a required command that always fails.
	hooksToml := `[hooks]
[[hooks.post_create]]
type = "command"
command = "false"
required = true
`
	if err := os.WriteFile(filepath.Join(groveDir, "hooks.toml"), []byte(hooksToml), 0644); err != nil {
		t.Fatalf("write hooks.toml: %v", err)
	}

	// hooks.NewExecutor() uses cwd to find .grove/hooks.toml, so chdir to mainDir.
	t.Chdir(mainDir)

	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		t.Fatalf("state mgr: %v", err)
	}

	var buf bytes.Buffer
	w := cli.NewWriter(&buf, false)

	cfg := config.LoadDefaults()
	opts := BootstrapOpts{
		Name:         "feature",
		Branch:       "feature",
		WorktreePath: wtPath,
		MainPath:     mainDir,
		ProjectName:  "test-proj",
	}

	// BootstrapWorktree must return nil (hook failures are non-fatal).
	if err := BootstrapWorktree(stateMgr, cfg, opts, w); err != nil {
		t.Fatalf("BootstrapWorktree returned unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "post-create hook failed") {
		t.Errorf("expected warning about post-create hook failure on writer, got: %q", got)
	}
}
