package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/worktree"
)

// loadFetchContext must resolve config from the manager's project root, not
// from the process cwd — the dashboard refresh loop runs with an arbitrary
// cwd, and cwd-based discovery both spawned git per refresh and could pick up
// the wrong project's config (#142).
func TestLoadFetchContext_ConfigFromManagerRoot(t *testing.T) {
	root := t.TempDir()
	groveDir := filepath.Join(root, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatalf("mkdir .grove: %v", err)
	}
	if err := os.WriteFile(filepath.Join(groveDir, "config.toml"),
		[]byte("default_base_branch = \"trunk\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	// Isolate from the developer's real global config.
	t.Setenv("GROVE_CONFIG", filepath.Join(root, "no-such-global.toml"))

	mgr, err := worktree.NewManager(root)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// The test runs with cwd inside grove's own repo; before the fix,
	// config.Load() would discover THAT project's config instead of root's.
	fc := loadFetchContext(mgr, nil)

	if fc.cfg == nil {
		t.Fatal("fetchContext.cfg should be loaded")
	}
	if fc.defaultBranch != "trunk" {
		t.Errorf("defaultBranch = %q, want %q (config must come from the manager's root, not cwd)", fc.defaultBranch, "trunk")
	}
}
