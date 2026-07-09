package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/worktree"
)

func TestRenderContextSummary(t *testing.T) {
	tests := []struct {
		name     string
		state    *CreateState
		width    int
		wantStrs []string
	}{
		{
			name: "shows name and full name",
			state: &CreateState{
				Name:        "feature-auth",
				ProjectName: "acupoll",
			},
			width:    60,
			wantStrs: []string{"feature-auth", "acupoll-feature-auth"},
		},
		{
			name: "shows branch when set",
			state: &CreateState{
				Name:        "feature-auth",
				ProjectName: "acupoll",
				BaseBranch:  "develop",
			},
			width:    60,
			wantStrs: []string{"feature-auth", "acupoll-feature-auth", "develop"},
		},
		{
			name: "empty name produces summary header",
			state: &CreateState{
				Name:        "",
				ProjectName: "acupoll",
			},
			width:    60,
			wantStrs: []string{"Summary"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderContextSummary(tt.state, tt.width)
			for _, want := range tt.wantStrs {
				if !strings.Contains(got, want) {
					t.Errorf("renderContextSummary() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestRenderCreateBranchChoiceV2(t *testing.T) {
	state := &CreateState{
		Step:              CreateStepBranchChoice,
		Branches:          []string{"main", "develop"},
		BranchFilterInput: newBranchFilterInput(),
		BranchNameInput:   newBranchNameInput(),
	}
	got := renderCreateBranchChoiceV2(state, 80)

	for _, want := range []string{"Select an existing branch", "Create a new branch", "Branch"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderCreateBranchChoiceV2() missing %q in output:\n%s", want, got)
		}
	}
}

func TestRenderCreateBranchSelectV2(t *testing.T) {
	tests := []struct {
		name     string
		state    *CreateState
		width    int
		wantStrs []string
	}{
		{
			name: "shows branch list",
			state: &CreateState{
				Step:              CreateStepBranchSelect,
				Branches:          []string{"main", "develop", "feature/auth"},
				BranchFilterInput: newBranchFilterInput(),
			},
			width:    80,
			wantStrs: []string{"main", "develop", "Branch"},
		},
		{
			name: "shows filter when active",
			state: func() *CreateState {
				bfi := newBranchFilterInput()
				bfi.SetValue("feat")
				return &CreateState{
					Step:              CreateStepBranchSelect,
					Branches:          []string{"main", "develop", "feature/auth"},
					BranchFilterInput: bfi,
					BranchFilterMode:  BranchFilterOn,
				}
			}(),
			width:    80,
			wantStrs: []string{"feature/auth", "Filter"},
		},
		{
			name: "shows stepper labels",
			state: &CreateState{
				Step:              CreateStepBranchSelect,
				Branches:          []string{"main"},
				BranchFilterInput: newBranchFilterInput(),
			},
			width:    80,
			wantStrs: []string{"Branch", "Name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderCreateBranchSelectV2(tt.state, tt.width)
			for _, want := range tt.wantStrs {
				if !strings.Contains(got, want) {
					t.Errorf("renderCreateBranchSelectV2() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestRenderCreateBranchCreateV2(t *testing.T) {
	state := &CreateState{
		Step:            CreateStepBranchCreate,
		BranchNameInput: newBranchNameInput(),
	}
	got := renderCreateBranchCreateV2(state, 80)

	for _, want := range []string{"Enter a name", "Branch"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderCreateBranchCreateV2() missing %q in output:\n%s", want, got)
		}
	}
}

func TestRenderCreateNameV2(t *testing.T) {
	tests := []struct {
		name     string
		state    *CreateState
		width    int
		wantStrs []string
	}{
		{
			name:     "name step shows stepper and input",
			state:    createStateWithName("test", "acupoll"),
			width:    80,
			wantStrs: []string{"Name", "test", "acupoll-test"},
		},
		{
			name: "name step shows error",
			state: func() *CreateState {
				s := createStateWithName("bad name!", "acupoll")
				s.Error = "invalid characters"
				return s
			}(),
			width:    80,
			wantStrs: []string{"invalid characters"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderCreateNameV2(tt.state, tt.width)
			for _, want := range tt.wantStrs {
				if !strings.Contains(got, want) {
					t.Errorf("renderCreateNameV2() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestRenderCreateV2Dispatch(t *testing.T) {
	tests := []struct {
		name     string
		state    *CreateState
		wantStrs []string
	}{
		{
			name: "dispatches to name step",
			state: func() *CreateState {
				s := createStateWithName("", "proj")
				return s
			}(),
			wantStrs: []string{"Name"},
		},
		{
			name: "dispatches to branch choice step",
			state: &CreateState{
				Step:              CreateStepBranchChoice,
				Branches:          []string{"main", "develop"},
				BranchFilterInput: newBranchFilterInput(),
				BranchNameInput:   newBranchNameInput(),
			},
			wantStrs: []string{"Select an existing branch"},
		},
		{
			name: "dispatches to branch select step",
			state: &CreateState{
				Step:              CreateStepBranchSelect,
				Branches:          []string{"main", "develop"},
				BranchFilterInput: newBranchFilterInput(),
			},
			wantStrs: []string{"Branch", "main"},
		},
		{
			name: "dispatches to creating spinner",
			state: &CreateState{
				Creating: true,
				Name:     "feat",
			},
			wantStrs: []string{"Creating", "feat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderCreateV2(tt.state, 80, "⠋")
			for _, want := range tt.wantStrs {
				if !strings.Contains(got, want) {
					t.Errorf("renderCreateV2() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestBackspacePreservesValues(t *testing.T) {
	s := createStateWithName("my-feature", "acupoll")
	s.BaseBranch = "develop"

	// Simulate going back to branch choice step
	s.Step = CreateStepBranchChoice

	if s.Name != "my-feature" {
		t.Errorf("Name not preserved: got %q, want %q", s.Name, "my-feature")
	}
	if s.ProjectName != "acupoll" {
		t.Errorf("ProjectName not preserved: got %q, want %q", s.ProjectName, "acupoll")
	}
	if s.BaseBranch != "develop" {
		t.Errorf("BaseBranch not preserved: got %q, want %q", s.BaseBranch, "develop")
	}

	// Navigate forward again — values should still be there
	s.Step = CreateStepName
	view := renderCreateNameV2(s, 80)
	if !strings.Contains(view, "my-feature") {
		t.Errorf("name step should show preserved name, got:\n%s", view)
	}
}

// TestCreateWorktreeCmd_RemoteBranch_UsesCreateFromBranch is a regression test
// for commit ca7ca08. Before that fix, the TUI used CreateFromExisting for the
// "from existing branch" flow, which only works with local refs. The fix
// switched to CreateFromBranch, which fetches from origin first.
//
// This test verifies that when baseBranch is non-empty, createWorktreeCmd
// dispatches through the CreateFromBranch path by inspecting the first log
// line sent on the streaming channel — it must include the base branch name.
// (The createFn will fail with no real repo; we only care about the log line.)
func TestCreateWorktreeCmd_RemoteBranch_UsesCreateFromBranch(t *testing.T) {
	// Build a minimal git repo so Manager.prepareWorktreePath can resolve paths,
	// then confirm the first streaming log line names the base branch.
	dir := t.TempDir()

	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "commit.gpgsign", "false"},
		{"config", "user.name", "Test"},
		{"config", "user.email", "t@t.com"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	// Create an initial commit so the repo is valid.
	f := filepath.Join(dir, "README")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	mgr, err := worktree.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	const baseBranch = "origin/feature-remote-only"
	cmd := createWorktreeCmd(mgr, nil, dir, "remote-wt", baseBranch, "", "")
	msg := cmd()

	// The first message must be a creationLogMsg (log line before createFn runs)
	// or a creationDoneMsg (if it failed quickly). Either way the log line or
	// error context must reference the base branch, proving CreateFromBranch
	// was called rather than CreateFromExisting.
	switch m := msg.(type) {
	case creationLogMsg:
		if !strings.Contains(m.line, baseBranch) {
			t.Errorf("first log line = %q, want it to contain base branch %q", m.line, baseBranch)
		}
	case creationDoneMsg:
		// Creation failed (expected — no remote). Verify the log lines
		// that were sent named the base branch. We can infer this from
		// the source being "create" (not a silent skip).
		if m.source != "create" {
			t.Errorf("creationDoneMsg.source = %q, want %q", m.source, "create")
		}
		// The error should be about the branch fetch/create failing, not a nil
		// error that would indicate a silent no-op.
		if m.err == nil {
			t.Error("expected non-nil error from create with nonexistent remote branch")
		}
	default:
		t.Fatalf("unexpected message type %T: %v", msg, msg)
	}

	// streamingCreateCmd runs createFn (the actual `git worktree add`) in a
	// background goroutine and returns after the first log line. Drain the
	// remaining events until the done message so that goroutine's git writes
	// complete before t.TempDir() cleanup runs — otherwise an in-flight
	// worktree add races RemoveAll on .git and CI flakes with
	// "directory not empty".
	for {
		lm, ok := msg.(creationLogMsg)
		if !ok {
			break // creationDoneMsg — the creation goroutine has finished
		}
		msg = readCreationLog(lm.ch, "create")()
	}
}

// createStateWithName creates a CreateState at the name step with a pre-set name.
func createStateWithName(name, projectName string) *CreateState {
	ni := newNameInput("")
	ni.SetValue(name)
	return &CreateState{
		Step:        CreateStepName,
		Name:        name,
		ProjectName: projectName,
		NameInput:   ni,
	}
}
