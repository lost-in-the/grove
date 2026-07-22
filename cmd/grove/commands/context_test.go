package commands

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/grove"
)

func TestEmitDriftNotice_PrintsAdoptHint(t *testing.T) {
	var buf bytes.Buffer
	w := cli.NewWriter(&buf, false)

	emitDriftNotice(w, "drifted-wt", grove.ReasonDriftedWorktree)

	out := buf.String()
	if !strings.Contains(out, "grove adopt") {
		t.Errorf("expected 'grove adopt' hint, got: %s", out)
	}
	if !strings.Contains(out, "drifted-wt") {
		t.Errorf("expected worktree name in notice, got: %s", out)
	}
}

func TestEmitDriftNotice_SilentWhenRegistered(t *testing.T) {
	var buf bytes.Buffer
	w := cli.NewWriter(&buf, false)

	emitDriftNotice(w, "ok-wt", grove.ReasonRegistered)

	if buf.Len() != 0 {
		t.Errorf("expected no output for registered worktree, got: %s", buf.String())
	}
}

// TestDriftProbeDir_NormalizesSubdirToWorktreeTopLevel guards the drift check
// in RequireGroveContext: it must diagnose the worktree containing cwd, not
// cwd itself — otherwise every command run from <worktree>/sub/deep printed a
// spurious "this worktree (deep) wasn't created by grove" notice.
func TestDriftProbeDir_NormalizesSubdirToWorktreeTopLevel(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(base, "main")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git(repo, "init")
	git(repo, "commit", "--allow-empty", "-m", "init")

	// A registered worktree with a deep subdirectory.
	wt := filepath.Join(base, "main-feature")
	git(repo, "worktree", "add", wt, "-b", "feature")
	sub := filepath.Join(wt, "sub", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	// From a subdirectory the probe dir is the worktree top level.
	if got := driftProbeDir(sub); got != wt {
		t.Errorf("driftProbeDir(subdir) = %q, want worktree root %q", got, wt)
	}
	// From the worktree root it is the identity.
	if got := driftProbeDir(wt); got != wt {
		t.Errorf("driftProbeDir(worktree root) = %q, want %q", got, wt)
	}
	// Outside any repo it falls back to the directory itself.
	outside := filepath.Join(base, "not-a-repo")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := driftProbeDir(outside); got != outside {
		t.Errorf("driftProbeDir(non-repo) = %q, want %q", got, outside)
	}

	// End to end: with the worktree registered in state.json, the normalized
	// probe dir reports registered — the spurious notice never fires from a
	// subdirectory. The raw subdir is exactly the misdiagnosis being fixed.
	groveDir := filepath.Join(repo, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	state := fmt.Sprintf(`{"worktrees":{"feature":{"path":%q}}}`, wt)
	if err := os.WriteFile(filepath.Join(groveDir, "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := grove.DiagnoseDrift(driftProbeDir(sub), repo); got != grove.ReasonRegistered {
		t.Errorf("DiagnoseDrift(probe dir) = %v, want ReasonRegistered", got)
	}
	if got := grove.DiagnoseDrift(sub, repo); got != grove.ReasonDriftedWorktree {
		t.Errorf("DiagnoseDrift(raw subdir) = %v, want ReasonDriftedWorktree (the misdiagnosis driftProbeDir prevents)", got)
	}
}
