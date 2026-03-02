package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupWIPTestRepo initializes a temporary git repo with one tracked file and an initial commit.
func setupWIPTestRepo(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init")
	run("git", "config", "--local", "user.name", "test")
	run("git", "config", "--local", "user.email", "test@test")
	run("git", "config", "--local", "commit.gpgsign", "false")

	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("original"), 0644); err != nil {
		t.Fatalf("failed to write tracked.txt: %v", err)
	}
	run("git", "add", "tracked.txt")
	run("git", "commit", "-m", "init")

	return dir
}

func gitAdd(t *testing.T, dir, file string) {
	t.Helper()
	cmd := exec.Command("git", "add", file)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add %q failed: %v\n%s", file, err, out)
	}
}

func TestHasWIP(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  bool
	}{
		{
			name:  "clean repo returns false",
			setup: func(t *testing.T, dir string) {},
			want:  false,
		},
		{
			name: "modified tracked file returns true",
			setup: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("modified"), 0644); err != nil {
					t.Fatalf("failed to modify file: %v", err)
				}
			},
			want: true,
		},
		{
			name: "new untracked file returns true",
			setup: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644); err != nil {
					t.Fatalf("failed to create file: %v", err)
				}
			},
			want: true,
		},
		{
			name: "staged file returns true",
			setup: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged"), 0644); err != nil {
					t.Fatalf("failed to create file: %v", err)
				}
				gitAdd(t, dir, "staged.txt")
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupWIPTestRepo(t)
			tt.setup(t, dir)

			h := NewWIPHandler(dir)
			got, err := h.HasWIP()
			if err != nil {
				t.Fatalf("HasWIP() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("HasWIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListWIPFiles(t *testing.T) {
	t.Run("clean returns empty", func(t *testing.T) {
		dir := setupWIPTestRepo(t)
		h := NewWIPHandler(dir)
		files, err := h.ListWIPFiles()
		if err != nil {
			t.Fatalf("ListWIPFiles() error = %v", err)
		}
		if len(files) != 0 {
			t.Errorf("clean repo should return no files, got %v", files)
		}
	})

	t.Run("multiple changes listed", func(t *testing.T) {
		dir := setupWIPTestRepo(t)

		if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("modified"), 0644); err != nil {
			t.Fatalf("failed to modify tracked.txt: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644); err != nil {
			t.Fatalf("failed to create new.txt: %v", err)
		}

		h := NewWIPHandler(dir)
		files, err := h.ListWIPFiles()
		if err != nil {
			t.Fatalf("ListWIPFiles() error = %v", err)
		}
		if len(files) < 2 {
			t.Errorf("expected at least 2 files, got %d: %v", len(files), files)
		}
	})
}

func TestStashAndPop(t *testing.T) {
	dir := setupWIPTestRepo(t)

	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("wip changes"), 0644); err != nil {
		t.Fatalf("failed to modify tracked.txt: %v", err)
	}

	h := NewWIPHandler(dir)

	hasWIP, err := h.HasWIP()
	if err != nil {
		t.Fatalf("HasWIP() before stash: %v", err)
	}
	if !hasWIP {
		t.Fatal("expected WIP before stash")
	}

	if err := h.Stash("test stash"); err != nil {
		t.Fatalf("Stash() error = %v", err)
	}

	hasWIP, err = h.HasWIP()
	if err != nil {
		t.Fatalf("HasWIP() after stash: %v", err)
	}
	if hasWIP {
		t.Error("expected no WIP after stash")
	}

	if err := h.PopStash(); err != nil {
		t.Fatalf("PopStash() error = %v", err)
	}

	hasWIP, err = h.HasWIP()
	if err != nil {
		t.Fatalf("HasWIP() after pop: %v", err)
	}
	if !hasWIP {
		t.Error("expected WIP restored after pop stash")
	}
}

func TestHasStagedChanges(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  bool
	}{
		{
			name:  "clean returns false",
			setup: func(t *testing.T, dir string) {},
			want:  false,
		},
		{
			name: "staged file returns true",
			setup: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged"), 0644); err != nil {
					t.Fatalf("failed to create file: %v", err)
				}
				gitAdd(t, dir, "staged.txt")
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupWIPTestRepo(t)
			tt.setup(t, dir)

			h := NewWIPHandler(dir)
			got, err := h.HasStagedChanges()
			if err != nil {
				t.Fatalf("HasStagedChanges() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("HasStagedChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasUnstagedChanges(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  bool
	}{
		{
			name:  "clean returns false",
			setup: func(t *testing.T, dir string) {},
			want:  false,
		},
		{
			name: "modified tracked file returns true",
			setup: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("modified"), 0644); err != nil {
					t.Fatalf("failed to modify tracked.txt: %v", err)
				}
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupWIPTestRepo(t)
			tt.setup(t, dir)

			h := NewWIPHandler(dir)
			got, err := h.HasUnstagedChanges()
			if err != nil {
				t.Fatalf("HasUnstagedChanges() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("HasUnstagedChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyPatch_Empty(t *testing.T) {
	dir := setupWIPTestRepo(t)
	h := NewWIPHandler(dir)

	if err := h.ApplyPatch(nil); err != nil {
		t.Errorf("ApplyPatch(nil) expected nil error, got %v", err)
	}
	if err := h.ApplyPatch([]byte{}); err != nil {
		t.Errorf("ApplyPatch([]) expected nil error, got %v", err)
	}
}

func TestCreateAndApplyPatch(t *testing.T) {
	dir := setupWIPTestRepo(t)

	// Modify the tracked file to create WIP
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("patched content"), 0644); err != nil {
		t.Fatalf("failed to write tracked.txt: %v", err)
	}

	h := NewWIPHandler(dir)

	// Create patch from the WIP changes
	patch, err := h.CreatePatch()
	if err != nil {
		t.Fatalf("CreatePatch() error = %v", err)
	}
	if len(patch) == 0 {
		t.Fatal("CreatePatch() returned empty patch, expected non-empty")
	}

	// Discard working tree changes to simulate a clean state
	discardCmd := exec.Command("git", "-C", dir, "checkout", "--", ".")
	if out, err := discardCmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -- . failed: %v\n%s", err, out)
	}

	// Verify the file is back to its original content
	content, err := os.ReadFile(filepath.Join(dir, "tracked.txt"))
	if err != nil {
		t.Fatalf("failed to read tracked.txt after discard: %v", err)
	}
	if string(content) != "original" {
		t.Fatalf("expected original content after discard, got %q", content)
	}

	// Apply the patch and verify the changes are restored
	if err := h.ApplyPatch(patch); err != nil {
		t.Fatalf("ApplyPatch() error = %v", err)
	}

	content, err = os.ReadFile(filepath.Join(dir, "tracked.txt"))
	if err != nil {
		t.Fatalf("failed to read tracked.txt after patch: %v", err)
	}
	if string(content) != "patched content" {
		t.Errorf("ApplyPatch() restored %q, want %q", content, "patched content")
	}
}

func TestHasUntrackedFiles(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  bool
	}{
		{
			name:  "clean returns false",
			setup: func(t *testing.T, dir string) {},
			want:  false,
		},
		{
			name: "new untracked file returns true",
			setup: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644); err != nil {
					t.Fatalf("failed to create file: %v", err)
				}
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupWIPTestRepo(t)
			tt.setup(t, dir)

			h := NewWIPHandler(dir)
			got, err := h.HasUntrackedFiles()
			if err != nil {
				t.Fatalf("HasUntrackedFiles() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("HasUntrackedFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}
