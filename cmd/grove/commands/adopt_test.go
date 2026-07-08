package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAdoptTarget_UsesCwdWhenNoArg(t *testing.T) {
	tmpDir := t.TempDir()
	got, err := resolveAdoptTarget(tmpDir, []string{})
	if err != nil {
		t.Fatalf("resolveAdoptTarget: %v", err)
	}
	expected, _ := filepath.EvalSymlinks(tmpDir)
	if expected == "" {
		expected = tmpDir
	}
	if got != expected {
		t.Errorf("got %q want %q", got, expected)
	}
}

func TestResolveAdoptTarget_UsesArgWhenProvided(t *testing.T) {
	tmpDir := t.TempDir()
	other := filepath.Join(tmpDir, "other")
	if err := os.MkdirAll(other, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got, err := resolveAdoptTarget(tmpDir, []string{other})
	if err != nil {
		t.Fatalf("resolveAdoptTarget: %v", err)
	}
	expected, _ := filepath.EvalSymlinks(other)
	if expected == "" {
		expected = other
	}
	if got != expected {
		t.Errorf("got %q want %q", got, expected)
	}
}

func TestResolveAdoptTarget_ErrorsOnNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := resolveAdoptTarget(tmpDir, []string{filepath.Join(tmpDir, "nope")})
	if err == nil {
		t.Errorf("expected error for nonexistent path")
	}
}

func TestGitBranchAt_DetachedHEAD(t *testing.T) {
	// Set up a real git repo, make a commit, then detach HEAD.
	dir := t.TempDir()
	runAdoptGit(t, dir, "init")
	runAdoptGit(t, dir, "commit", "--allow-empty", "-m", "init")

	// Detach HEAD by checking out the commit hash directly.
	hashOut, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	hash := strings.TrimSpace(string(hashOut))
	runAdoptGit(t, dir, "checkout", hash)

	_, gotErr := gitBranchAt(dir)
	if gotErr == nil {
		t.Fatal("expected error for detached HEAD, got nil")
	}
	if !strings.Contains(gotErr.Error(), "detached HEAD") {
		t.Errorf("error %q does not mention detached HEAD", gotErr.Error())
	}
}

func TestGitBranchAt_NamedBranch(t *testing.T) {
	dir := t.TempDir()
	runAdoptGit(t, dir, "init")
	runAdoptGit(t, dir, "commit", "--allow-empty", "-m", "init")

	branch, err := gitBranchAt(dir)
	if err != nil {
		t.Fatalf("gitBranchAt: %v", err)
	}
	// The init branch is typically "main" or "master" — just verify it's not empty or "HEAD".
	if branch == "" || branch == "HEAD" {
		t.Errorf("got branch %q, want a real branch name", branch)
	}
}

// runAdoptGit runs a git command in dir and fatals the test on failure.
func runAdoptGit(t *testing.T, dir string, args ...string) {
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

func TestAdopt_StripProjectPrefixForName(t *testing.T) {
	tests := []struct {
		name        string
		dirBase     string
		projectName string
		want        string
	}{
		{"strips matching prefix", "grove-feature", "grove", "feature"},
		{"no prefix when project doesn't match", "myproj-feature", "grove", "myproj-feature"},
		{"no prefix when name equals project", "grove", "grove", "grove"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dirBase
			if prefix := tt.projectName + "-"; strings.HasPrefix(got, prefix) {
				got = strings.TrimPrefix(got, prefix)
			}
			if got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestGitCommonDirAt_DistinguishesRepositories(t *testing.T) {
	base := t.TempDir()

	repo := filepath.Join(base, "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runAdoptGit(t, repo, "init")
	runAdoptGit(t, repo, "commit", "--allow-empty", "-m", "init")

	// Linked worktree of the same repository must share the common dir.
	wt := filepath.Join(base, "repo-wt")
	runAdoptGit(t, repo, "worktree", "add", wt)

	repoCommon, err := gitCommonDirAt(repo)
	if err != nil {
		t.Fatalf("gitCommonDirAt(repo): %v", err)
	}
	wtCommon, err := gitCommonDirAt(wt)
	if err != nil {
		t.Fatalf("gitCommonDirAt(wt): %v", err)
	}
	if repoCommon != wtCommon {
		t.Errorf("worktree common dir %q != repo common dir %q", wtCommon, repoCommon)
	}

	// A subdirectory of the worktree also resolves to the same repo.
	sub := filepath.Join(wt, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	subCommon, err := gitCommonDirAt(sub)
	if err != nil {
		t.Fatalf("gitCommonDirAt(sub): %v", err)
	}
	if subCommon != repoCommon {
		t.Errorf("subdir common dir %q != repo common dir %q", subCommon, repoCommon)
	}

	// An unrelated repository must NOT match — this is the membership check
	// grove adopt relies on to reject foreign repos.
	other := filepath.Join(base, "other")
	if err := os.MkdirAll(other, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runAdoptGit(t, other, "init")
	runAdoptGit(t, other, "commit", "--allow-empty", "-m", "init")

	otherCommon, err := gitCommonDirAt(other)
	if err != nil {
		t.Fatalf("gitCommonDirAt(other): %v", err)
	}
	if otherCommon == repoCommon {
		t.Errorf("unrelated repo shares common dir %q — membership check would pass foreign repos", otherCommon)
	}
}
