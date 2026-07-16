//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
	"github.com/lost-in-the/grove/internal/worktree"
)

// TestRmForce_NodeModules exercises force removal of a worktree that contains
// an untracked directory (e.g. a node_modules left by a post-create hook).
// git's own `worktree remove --force` handles this; the os.RemoveAll fallback
// in worktree.Manager.Remove covers the rarer cases where even that fails.
func TestRmForce_NodeModules(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := testhelper.SetupRailsFixture(t)

	mgr, err := worktree.NewManager(repo)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create a worktree.
	if err := mgr.Create("feat", "feat-branch"); err != nil {
		t.Fatalf("Create worktree: %v", err)
	}

	projectName := mgr.GetProjectName()
	fullName := projectName + "-feat"

	wt, err := mgr.Find(fullName)
	if err != nil || wt == nil {
		t.Fatalf("Find worktree %q: err=%v wt=%v", fullName, err, wt)
	}

	// Plant node_modules to simulate a post-create hook artifact.
	nodeModules := filepath.Join(wt.Path, "node_modules", "some-pkg")
	if err := os.MkdirAll(nodeModules, 0755); err != nil {
		t.Fatalf("MkdirAll node_modules: %v", err)
	}
	indexJS := filepath.Join(nodeModules, "index.js")
	if err := os.WriteFile(indexJS, []byte("module.exports={}"), 0644); err != nil {
		t.Fatalf("WriteFile index.js: %v", err)
	}

	// Force removal must succeed even though node_modules is present.
	if err := mgr.Remove(fullName, true); err != nil {
		t.Fatalf("Remove with node_modules: %v", err)
	}

	// Worktree directory must be gone.
	if _, statErr := os.Stat(wt.Path); !os.IsNotExist(statErr) {
		t.Errorf("worktree directory still exists at %s", wt.Path)
	}

	// git must no longer list it.
	wtAfter, _ := mgr.Find(fullName)
	if wtAfter != nil {
		t.Errorf("worktree %q should not be found after Remove()", fullName)
	}
}
