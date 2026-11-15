package worktree

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/state"
)

// bootstrapFixture lays out a main worktree with one real source file and a
// config that asks for that file plus one that does not exist.
func bootstrapFixture(t *testing.T) (mainDir, wtPath string, cfg *config.Config) {
	t.Helper()
	root := t.TempDir()
	mainDir = filepath.Join(root, "main")
	wtPath = filepath.Join(root, "feature")
	for _, d := range []string{filepath.Join(mainDir, ".grove"), filepath.Join(mainDir, "config"), wtPath} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(mainDir, "config", "present.key"), []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg = config.LoadDefaults()
	cfg.Plugins.Docker.External = &config.ExternalComposeConfig{
		CopyFiles: []string{"config/secret.key", "config/present.key"},
	}
	return mainDir, wtPath, cfg
}

func TestBootstrapWorktree_FileSetupFailureIsReturned(t *testing.T) {
	mainDir, wtPath, cfg := bootstrapFixture(t)
	stateMgr, err := state.NewManager(filepath.Join(mainDir, ".grove"))
	if err != nil {
		t.Fatal(err)
	}

	err = BootstrapWorktree(stateMgr, cfg, BootstrapOpts{
		Name: "feature", Branch: "feature", WorktreePath: wtPath, MainPath: mainDir, ProjectName: "myapp",
	}, nil)

	if !errors.Is(err, ErrFileSetupIncomplete) {
		t.Fatalf("err = %v, want ErrFileSetupIncomplete", err)
	}
	var fe *FileSetupError
	if !errors.As(err, &fe) || len(fe.Failed) != 1 || fe.Failed[0].Path != "config/secret.key" {
		t.Fatalf("errors.As: failed = %+v, want only config/secret.key", fe)
	}
	if errors.Is(err, ErrRequiredHookFailed) {
		t.Error("a file setup failure must not read as a required hook failure")
	}
	if _, statErr := os.Stat(filepath.Join(wtPath, "config", "present.key")); statErr != nil {
		t.Errorf("the entry that could be copied was not copied: %v", statErr)
	}
	if ws, _ := stateMgr.GetWorktree("feature"); ws == nil {
		t.Error("worktree was not registered in state")
	}
}

func TestBootstrap_ReportsEveryEntry(t *testing.T) {
	mainDir, wtPath, cfg := bootstrapFixture(t)
	res, err := Bootstrap(nil, cfg, BootstrapOpts{
		Name: "feature", Branch: "feature", WorktreePath: wtPath, MainPath: mainDir,
	}, nil)
	if !errors.Is(err, ErrFileSetupIncomplete) {
		t.Fatalf("err = %v, want ErrFileSetupIncomplete", err)
	}
	if len(res.Files) != 2 || res.Files[0].Status != FileFailed || res.Files[1].Status != FileCopied {
		t.Errorf("Files = %+v, want [failed copied]", res.Files)
	}
}

func TestBootstrapWorktree_RunsFileSetupWhenStateSaveFails(t *testing.T) {
	mainDir, wtPath, cfg := bootstrapFixture(t)
	groveDir := filepath.Join(mainDir, ".grove")
	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		t.Fatal(err)
	}
	// Replace .grove with a regular file so the state write fails.
	if err := os.RemoveAll(groveDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(groveDir, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	err = BootstrapWorktree(stateMgr, cfg, BootstrapOpts{
		Name: "feature", Branch: "feature", WorktreePath: wtPath, MainPath: mainDir,
	}, nil)

	if !errors.Is(err, ErrStateRegistration) {
		t.Errorf("err = %v, want it to match ErrStateRegistration", err)
	}
	if !errors.Is(err, ErrFileSetupIncomplete) {
		t.Errorf("err = %v, want it to also match ErrFileSetupIncomplete (config/secret.key is missing)", err)
	}
	if _, statErr := os.Stat(filepath.Join(wtPath, "config", "present.key")); statErr != nil {
		t.Errorf("file setup did not run after the state write failed: %v", statErr)
	}
}
