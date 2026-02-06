package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetCurrentCommit_Normal(t *testing.T) {
	// Create a temporary git repo with a commit
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	// Create a file and commit
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	commit, err := getCurrentCommit(dir)
	if err != nil {
		t.Fatalf("getCurrentCommit() error: %v", err)
	}
	if len(commit) != 7 {
		t.Errorf("expected 7-char commit hash, got %q (len %d)", commit, len(commit))
	}
}

func TestGetCurrentCommit_InvalidRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := getCurrentCommit(dir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestSyncCmd(t *testing.T) {
	if syncCmd == nil {
		t.Fatal("syncCmd is nil")
	}

	if syncCmd.Use != "sync [name]" {
		t.Errorf("syncCmd.Use = %v, want 'sync [name]'", syncCmd.Use)
	}

	if syncCmd.RunE == nil {
		t.Error("syncCmd.RunE is nil")
	}
}
