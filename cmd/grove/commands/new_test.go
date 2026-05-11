package commands

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/cmdexec"
)

func TestNewCmd(t *testing.T) {
	if newCmd == nil {
		t.Fatal("newCmd is nil")
	}

	if newCmd.Use != "new [name]" {
		t.Errorf("newCmd.Use = %v, want 'new [name]'", newCmd.Use)
	}

	if newCmd.Short == "" {
		t.Error("newCmd.Short is empty")
	}

	if newCmd.RunE == nil {
		t.Error("newCmd.RunE is nil")
	}
}

func TestNewCmd_Flags(t *testing.T) {
	flags := newCmd.Flags()

	tests := []string{"json", "branch", "from", "mirror", "no-docker", "no-switch"}
	for _, name := range tests {
		if flags.Lookup(name) == nil {
			t.Errorf("expected --%s flag to exist", name)
		}
	}
}

func TestNewCmd_BranchFlag(t *testing.T) {
	flag := newCmd.Flags().Lookup("branch")
	if flag == nil {
		t.Fatal("newCmd missing --branch flag")
	}
	if flag.Shorthand != "b" {
		t.Errorf("--branch shorthand = %q, want %q", flag.Shorthand, "b")
	}
	if flag.DefValue != "" {
		t.Errorf("--branch should default to empty, got %q", flag.DefValue)
	}
}

func TestNewCmd_FromFlag(t *testing.T) {
	flag := newCmd.Flags().Lookup("from")
	if flag == nil {
		t.Fatal("newCmd missing --from flag")
	}
	if flag.Shorthand != "f" {
		t.Errorf("--from shorthand = %q, want %q", flag.Shorthand, "f")
	}
	if flag.DefValue != "" {
		t.Errorf("--from should default to empty, got %q", flag.DefValue)
	}
}

func TestNewCmd_FromSetsBaseRef(t *testing.T) {
	// Verify that --from flag exists and can accept a ref value
	flag := newCmd.Flags().Lookup("from")
	if flag == nil {
		t.Fatal("--from flag not found")
	}

	// The flag's usage should indicate it sets the base ref
	if !strings.Contains(flag.Usage, "ref") {
		t.Errorf("--from usage = %q, should mention 'ref'", flag.Usage)
	}
}

func TestNewCmd_BranchOverridesBranchName(t *testing.T) {
	// Verify that --branch flag exists and is used to override the branch name
	flag := newCmd.Flags().Lookup("branch")
	if flag == nil {
		t.Fatal("--branch flag not found")
	}

	// The flag's usage should indicate it overrides the branch name
	if !strings.Contains(strings.ToLower(flag.Usage), "branch") {
		t.Errorf("--branch usage = %q, should mention 'branch'", flag.Usage)
	}
}

func TestNewCmd_NoSwitchFlag(t *testing.T) {
	flag := newCmd.Flags().Lookup("no-switch")
	if flag == nil {
		t.Fatal("expected --no-switch flag to exist")
	}
	if flag.Shorthand != "n" {
		t.Errorf("--no-switch shorthand = %q, want %q", flag.Shorthand, "n")
	}
	if flag.DefValue != "false" {
		t.Errorf("--no-switch default = %q, want %q", flag.DefValue, "false")
	}
}

func TestNewAutoSwitchDefault(t *testing.T) {
	// Verify newNoSwitch defaults to false, meaning auto-switch is on by default
	if newNoSwitch {
		t.Error("newNoSwitch should default to false (auto-switch enabled)")
	}
}

func TestNewCmd_MirrorFromMutuallyExclusive(t *testing.T) {
	// --mirror and --from should be mutually exclusive
	// This is configured in init() via MarkFlagsMutuallyExclusive
	mirrorFlag := newCmd.Flags().Lookup("mirror")
	fromFlag := newCmd.Flags().Lookup("from")

	if mirrorFlag == nil {
		t.Fatal("--mirror flag not found")
	}
	if fromFlag == nil {
		t.Fatal("--from flag not found")
	}

	// Verify mutual exclusion is enforced by cobra by checking the
	// flag annotations that MarkFlagsMutuallyExclusive sets
	mirrorAnnotations := mirrorFlag.Annotations
	if mirrorAnnotations == nil {
		t.Fatal("--mirror flag has no annotations; MarkFlagsMutuallyExclusive not configured")
	}

	exclusiveGroup, ok := mirrorAnnotations["cobra_annotation_mutually_exclusive"]
	if !ok {
		t.Fatal("--mirror missing cobra_annotation_mutually_exclusive annotation")
	}

	// The annotation should list "from" as a mutually exclusive peer
	foundFrom := false
	for _, group := range exclusiveGroup {
		if strings.Contains(group, "from") {
			foundFrom = true
			break
		}
	}
	if !foundFrom {
		t.Error("--mirror should be mutually exclusive with --from")
	}
}

func TestNewCmd_MirrorBranchMutuallyExclusive(t *testing.T) {
	// --mirror and --branch should be mutually exclusive
	mirrorFlag := newCmd.Flags().Lookup("mirror")
	branchFlag := newCmd.Flags().Lookup("branch")

	if mirrorFlag == nil {
		t.Fatal("--mirror flag not found")
	}
	if branchFlag == nil {
		t.Fatal("--branch flag not found")
	}

	mirrorAnnotations := mirrorFlag.Annotations
	if mirrorAnnotations == nil {
		t.Fatal("--mirror flag has no annotations; MarkFlagsMutuallyExclusive not configured")
	}

	exclusiveGroup, ok := mirrorAnnotations["cobra_annotation_mutually_exclusive"]
	if !ok {
		t.Fatal("--mirror missing cobra_annotation_mutually_exclusive annotation")
	}

	foundBranch := false
	for _, group := range exclusiveGroup {
		if strings.Contains(group, "branch") {
			foundBranch = true
			break
		}
	}
	if !foundBranch {
		t.Error("--mirror should be mutually exclusive with --branch")
	}
}

func TestNewCmd_RequiresExactlyOneArg(t *testing.T) {
	if newCmd.Args == nil {
		t.Error("newCmd.Args should not be nil — should require exactly one argument")
	}
}

func TestNewCmd_HasSpawnAlias(t *testing.T) {
	found := false
	for _, alias := range newCmd.Aliases {
		if alias == "spawn" {
			found = true
			break
		}
	}
	if !found {
		t.Error("newCmd should have 'spawn' alias")
	}
}

func TestNewCmd_FromBranchFlag(t *testing.T) {
	flag := newCmd.Flags().Lookup("from-branch")
	if flag == nil {
		t.Fatal("newCmd missing --from-branch flag")
	}
	if flag.DefValue != "" {
		t.Errorf("--from-branch should default to empty, got %q", flag.DefValue)
	}
	if !strings.Contains(strings.ToLower(flag.Usage), "existing") {
		t.Errorf("--from-branch usage should mention 'existing' branch, got: %q", flag.Usage)
	}
}

func TestNewCmd_DirtyFlag(t *testing.T) {
	flag := newCmd.Flags().Lookup("dirty")
	if flag == nil {
		t.Fatal("newCmd missing --dirty flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--dirty should default to false, got %q", flag.DefValue)
	}
	if !strings.Contains(flag.Usage, "from-branch") {
		t.Errorf("--dirty usage should mention --from-branch requirement, got: %q", flag.Usage)
	}
}

// TestNewCmd_FromBranchMutuallyExclusive verifies --from-branch is mutually
// exclusive with --from, --branch, and --mirror (all attempt to create a new
// branch in different ways, so combining them is nonsense).
func TestNewCmd_FromBranchMutuallyExclusive(t *testing.T) {
	flag := newCmd.Flags().Lookup("from-branch")
	if flag == nil {
		t.Fatal("--from-branch flag not found")
	}
	annotations := flag.Annotations
	if annotations == nil {
		t.Fatal("--from-branch has no annotations; MarkFlagsMutuallyExclusive not configured")
	}
	group, ok := annotations["cobra_annotation_mutually_exclusive"]
	if !ok {
		t.Fatal("--from-branch missing cobra_annotation_mutually_exclusive annotation")
	}

	for _, peer := range []string{"from", "branch", "mirror"} {
		found := false
		for _, g := range group {
			if strings.Contains(g, peer) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("--from-branch should be mutually exclusive with --%s (annotations: %v)", peer, group)
		}
	}
}

// TestApplyDirtyPatch_RoundTrip exercises the diff-capture + diff-apply pair
// against a real git repo. It writes a baseline file, commits it, modifies it,
// captures `git diff HEAD`, then applies the patch in a freshly-built worktree
// directory and asserts the modification is reproduced. This is the core
// invariant of --dirty: whatever the user sees as uncommitted in their current
// worktree must appear identically in the new one.
func TestApplyDirtyPatch_RoundTrip(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	source := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = source
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-b", "main", ".")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "test")

	baseline := filepath.Join(source, "file.txt")
	if err := os.WriteFile(baseline, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "file.txt")
	runGit("commit", "-m", "baseline")

	// Modify the file but don't commit — this is what --dirty should carry.
	if err := os.WriteFile(baseline, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	patch, err := cmdexec.Output(context.Background(), "git", []string{"-C", source, "diff", "HEAD"}, "", cmdexec.GitLocal)
	if err != nil {
		t.Fatalf("capture diff: %v", err)
	}
	if len(patch) == 0 {
		t.Fatal("expected non-empty diff after modification")
	}

	// Build a separate "worktree" by adding via git worktree on a new branch.
	// applyDirtyPatch only needs a git working tree at the target path.
	target := filepath.Join(t.TempDir(), "wt")
	if out, err := exec.Command("git", "-C", source, "worktree", "add", "-b", "feature", target).CombinedOutput(); err != nil {
		t.Fatalf("create target worktree: %v\n%s", err, out)
	}

	if err := applyDirtyPatch(target, patch); err != nil {
		t.Fatalf("applyDirtyPatch: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "file.txt"))
	if err != nil {
		t.Fatalf("read target file: %v", err)
	}
	if string(got) != "hello\nworld\n" {
		t.Errorf("target file contents = %q, want %q", string(got), "hello\nworld\n")
	}
}
