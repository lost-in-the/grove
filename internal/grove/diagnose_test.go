package grove

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDiagnoseNoGrove(t *testing.T) {
	// Set up a git repo with no .grove
	dir := t.TempDir()
	runGit(t, dir, "init")

	result := DiagnoseNoGrove(dir)

	if result.Reason != ReasonNoGroveDir {
		t.Errorf("expected ReasonNoGroveDir, got %v", result.Reason)
	}
}

func TestDiagnoseNoGrove_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	result := DiagnoseNoGrove(dir)

	if result.Reason != ReasonNotGitRepo {
		t.Errorf("expected ReasonNotGitRepo, got %v", result.Reason)
	}
}

func TestDiagnoseNoGrove_WorktreeMainMissingGrove(t *testing.T) {
	// Set up main repo with no .grove, then create a worktree
	mainDir := t.TempDir()
	runGit(t, mainDir, "init")
	runGit(t, mainDir, "commit", "--allow-empty", "-m", "init")

	wtDir := filepath.Join(t.TempDir(), "worktree")
	runGit(t, mainDir, "worktree", "add", wtDir, "-b", "test-branch")

	result := DiagnoseNoGrove(wtDir)

	if result.Reason != ReasonMainWorktreeMissingGrove {
		t.Errorf("expected ReasonMainWorktreeMissingGrove, got %v", result.Reason)
	}
	// Resolve symlinks for comparison (macOS /var → /private/var)
	wantMain, _ := filepath.EvalSymlinks(mainDir)
	if result.MainWorktreePath != wantMain {
		t.Errorf("expected main path %s, got %s", wantMain, result.MainWorktreePath)
	}
}

func TestDiagnoseDrift_WorktreeNotInState(t *testing.T) {
	// Set up a main repo with a .grove dir, then a worktree that isn't in state.
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	if err := os.MkdirAll(filepath.Join(mainDir, ".grove"), 0755); err != nil {
		t.Fatalf("mkdir main/.grove: %v", err)
	}
	// Touch a state file with no worktrees registered.
	stateContent := `{"project": "test", "worktrees": {}}`
	if err := os.WriteFile(filepath.Join(mainDir, ".grove", "state.json"), []byte(stateContent), 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}

	worktreePath := filepath.Join(tmpDir, "drifted-wt")
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	got := DiagnoseDrift(worktreePath, mainDir)
	if got != ReasonDriftedWorktree {
		t.Errorf("expected ReasonDriftedWorktree, got %v", got)
	}
}

func TestDiagnoseDrift_WorktreeInState(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	if err := os.MkdirAll(filepath.Join(mainDir, ".grove"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	worktreePath := filepath.Join(tmpDir, "registered-wt")
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stateContent := `{"project": "test", "worktrees": {"registered-wt": {"path": "` + worktreePath + `", "branch": "main"}}}`
	if err := os.WriteFile(filepath.Join(mainDir, ".grove", "state.json"), []byte(stateContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := DiagnoseDrift(worktreePath, mainDir)
	if got != ReasonRegistered {
		t.Errorf("expected ReasonRegistered, got %v", got)
	}
}

func TestDiagnoseDrift_AtMainWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	if err := os.MkdirAll(filepath.Join(mainDir, ".grove"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got := DiagnoseDrift(mainDir, mainDir)
	if got != ReasonRegistered {
		t.Errorf("expected ReasonRegistered for main worktree, got %v", got)
	}
}

func TestIsWorktreeInState(t *testing.T) {
	tests := []struct {
		name         string
		stateData    []byte
		worktreePath string
		want         bool
	}{
		{
			name:         "present",
			stateData:    []byte(`{"worktrees":{"feature":{"path":"/repos/proj-feature","branch":"feature"}}}`),
			worktreePath: "/repos/proj-feature",
			want:         true,
		},
		{
			name:         "absent",
			stateData:    []byte(`{"worktrees":{"feature":{"path":"/repos/proj-feature","branch":"feature"}}}`),
			worktreePath: "/repos/proj-other",
			want:         false,
		},
		{
			name:         "prefix collision: /foo vs /foo-bar",
			stateData:    []byte(`{"worktrees":{"foo":{"path":"/repos/proj-foo","branch":"foo"}}}`),
			worktreePath: "/repos/proj-foo-bar",
			want:         false,
		},
		{
			name:         "nil data returns false",
			stateData:    nil,
			worktreePath: "/repos/proj-feature",
			want:         false,
		},
		{
			name:         "empty data returns false",
			stateData:    []byte{},
			worktreePath: "/repos/proj-feature",
			want:         false,
		},
		{
			name:         "malformed JSON returns false",
			stateData:    []byte(`not json at all`),
			worktreePath: "/repos/proj-feature",
			want:         false,
		},
		{
			name:         "path with backslash characters",
			stateData:    []byte(`{"worktrees":{"wt":{"path":"/repos/back\\slash","branch":"main"}}}`),
			worktreePath: `/repos/back\slash`,
			want:         true,
		},
		{
			name:         "path with non-ASCII characters",
			stateData:    []byte(`{"worktrees":{"wt":{"path":"/repos/кириллица","branch":"main"}}}`),
			worktreePath: "/repos/кириллица",
			want:         true,
		},
		{
			name: "path absent even when it appears as substring of another field",
			// "/repos/proj" appears inside the branch field value "/repos/proj-extra"
			// but must NOT match worktreePath "/repos/proj".
			stateData:    []byte(`{"worktrees":{"wt":{"path":"/repos/proj-other","branch":"/repos/proj"}}}`),
			worktreePath: "/repos/proj",
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWorktreeInState(tt.stateData, tt.worktreePath)
			if got != tt.want {
				t.Errorf("IsWorktreeInState(%q) = %v, want %v", tt.worktreePath, got, tt.want)
			}
		})
	}
}

func TestIsWorktreeInState_SymmetricSymlinkResolution(t *testing.T) {
	// state.json may have been written with the unresolved form of a path
	// (e.g. /var/folders/... on macOS) while the caller passes the resolved
	// form (/private/var/folders/...). The helper must match either way.
	tmpDir := t.TempDir()

	// Create a real directory (the resolved path) and a symlink that points
	// at it (the unresolved path).
	realDir := filepath.Join(tmpDir, "real")
	if err := os.MkdirAll(realDir, 0755); err != nil {
		t.Fatalf("mkdir real: %v", err)
	}
	linkDir := filepath.Join(tmpDir, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// state.json stores the symlinked (unresolved) path.
	stateData := []byte(`{"worktrees":{"wt":{"path":"` + linkDir + `","branch":"main"}}}`)

	// Caller passes the resolved real path — should still match.
	if !IsWorktreeInState(stateData, realDir) {
		t.Errorf("IsWorktreeInState(stored=symlink, caller=real) = false, want true")
	}

	// Inverse: state.json stores the real path, caller passes the symlinked
	// form. EvalSymlinks(linkDir) returns realDir, so this must also match.
	stateData2 := []byte(`{"worktrees":{"wt":{"path":"` + realDir + `","branch":"main"}}}`)
	if !IsWorktreeInState(stateData2, linkDir) {
		t.Errorf("IsWorktreeInState(stored=real, caller=symlink) = false, want true")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
