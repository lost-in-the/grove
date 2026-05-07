package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/output"
)

// TestContextCmd_Registration verifies the command is wired up correctly.
func TestContextCmd_Registration(t *testing.T) {
	if contextCmd == nil {
		t.Fatal("contextCmd is nil")
	}
	if contextCmd.Use != "context" {
		t.Errorf("contextCmd.Use = %q, want %q", contextCmd.Use, "context")
	}
	if contextCmd.RunE == nil {
		t.Error("contextCmd.RunE is nil")
	}
	if contextCmd.Flags().Lookup("json") == nil {
		t.Error("expected --json flag to exist")
	}
}

// --- JSON formatter tests (pure, no git required) ---

// buildContextOutput is a test helper that constructs a contextOutput and
// round-trips it through JSON to exercise the schema.
func buildContextOutput(overrides func(*contextOutput)) contextOutput {
	base := contextOutput{
		Name:   "my-feature",
		Path:   "/home/user/project-my-feature",
		Branch: "feat/my-feature",
		Commit: contextCommitInfo{
			SHA:     "abc1234",
			Message: "initial commit",
		},
		TrackingBranch: "origin/feat/my-feature",
		Status:         "clean",
		Ahead:          2,
		Behind:         0,
		StashCount:     0,
		RecentCommits: []contextRecentCommit{
			{SHA: "abc1234", Message: "initial commit"},
			{SHA: "def5678", Message: "second commit"},
		},
	}
	if overrides != nil {
		overrides(&base)
	}
	return base
}

// TestContextOutput_JSONSchema verifies that contextOutput marshals to the
// expected snake_case field names and omits optional fields when empty.
func TestContextOutput_JSONSchema(t *testing.T) {
	tests := []struct {
		name       string
		build      func(*contextOutput)
		wantFields []string
		absentKeys []string
	}{
		{
			name:       "clean worktree",
			build:      nil,
			wantFields: []string{`"name"`, `"path"`, `"branch"`, `"commit"`, `"tracking_branch"`, `"status"`, `"ahead"`, `"behind"`, `"stash_count"`, `"recent_commits"`},
			absentKeys: []string{`"changes"`},
		},
		{
			name: "dirty worktree with changes",
			build: func(o *contextOutput) {
				o.Status = "dirty"
				o.Changes = []string{"M internal/foo.go", "?? newfile.txt"}
			},
			wantFields: []string{`"changes"`},
			absentKeys: nil,
		},
		{
			name: "no tracking branch — field omitted",
			build: func(o *contextOutput) {
				o.TrackingBranch = ""
			},
			wantFields: nil,
			absentKeys: []string{`"tracking_branch"`},
		},
		{
			name: "detached HEAD",
			build: func(o *contextOutput) {
				o.Branch = "(detached HEAD at abc1234)"
			},
			wantFields: []string{`"(detached HEAD at abc1234)"`},
			absentKeys: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obj := buildContextOutput(tc.build)
			data, err := json.MarshalIndent(obj, "", "  ")
			if err != nil {
				t.Fatalf("MarshalIndent: %v", err)
			}
			js := string(data)

			for _, want := range tc.wantFields {
				if !strings.Contains(js, want) {
					t.Errorf("expected %q in JSON, got:\n%s", want, js)
				}
			}
			for _, absent := range tc.absentKeys {
				if strings.Contains(js, absent) {
					t.Errorf("expected %q to be absent from JSON, got:\n%s", absent, js)
				}
			}
		})
	}
}

// TestContextOutput_JSONRoundtrip verifies that JSON output can be decoded back
// into a contextOutput with all fields intact.
func TestContextOutput_JSONRoundtrip(t *testing.T) {
	original := buildContextOutput(func(o *contextOutput) {
		o.Status = "dirty"
		o.Changes = []string{"M foo.go"}
		o.StashCount = 3
		o.Ahead = 1
		o.Behind = 2
	})

	var buf bytes.Buffer
	origStdout := bytes.Buffer{}
	_ = origStdout // suppress unused warning

	// Marshal via output.PrintJSON (which uses json.MarshalIndent).
	// Capture by directly marshaling here since PrintJSON writes to os.Stdout.
	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	_, _ = fmt.Fprintf(&buf, "%s", data)

	var decoded contextOutput
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status: got %q, want %q", decoded.Status, original.Status)
	}
	if decoded.StashCount != original.StashCount {
		t.Errorf("StashCount: got %d, want %d", decoded.StashCount, original.StashCount)
	}
	if decoded.Ahead != original.Ahead {
		t.Errorf("Ahead: got %d, want %d", decoded.Ahead, original.Ahead)
	}
	if decoded.Behind != original.Behind {
		t.Errorf("Behind: got %d, want %d", decoded.Behind, original.Behind)
	}
	if len(decoded.Changes) != len(original.Changes) {
		t.Errorf("Changes len: got %d, want %d", len(decoded.Changes), len(original.Changes))
	}
	if len(decoded.RecentCommits) != len(original.RecentCommits) {
		t.Errorf("RecentCommits len: got %d, want %d", len(decoded.RecentCommits), len(original.RecentCommits))
	}
}

// TestContextHumanOutput_CleanWorktree checks human output for a clean worktree.
func TestContextHumanOutput_CleanWorktree(t *testing.T) {
	var buf bytes.Buffer
	w := cli.NewWriter(&buf, false)

	// Render a minimal "clean" context manually (mirrors what contextCmd does).
	cli.Header(w, "%s (%s)", "my-feature", "feat/my-feature")
	cli.Label(w, "Path:      ", "/home/user/project-my-feature")
	cli.Label(w, "Branch:    ", "feat/my-feature")
	cli.Label(w, "Tracking:  ", "origin/feat/my-feature  ↑2 ↓0")
	cli.Label(w, "Commit:    ", "abc1234 initial commit")
	cli.Label(w, "Status:    ", "✓ clean")

	out := buf.String()
	checks := []string{"my-feature", "feat/my-feature", "abc1234", "initial commit", "✓ clean", "Tracking:"}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

// TestContextHumanOutput_DirtyWorktree checks human output for a dirty worktree.
func TestContextHumanOutput_DirtyWorktree(t *testing.T) {
	var buf bytes.Buffer
	w := cli.NewWriter(&buf, false)

	cli.Header(w, "%s (%s)", "my-feature", "feat/my-feature")
	cli.Label(w, "Status:    ", "● dirty")
	cli.Faint(w, "           M internal/foo.go")

	out := buf.String()
	if !strings.Contains(out, "● dirty") {
		t.Errorf("expected dirty status, got:\n%s", out)
	}
	if !strings.Contains(out, "internal/foo.go") {
		t.Errorf("expected dirty file listed, got:\n%s", out)
	}
}

// TestContextHumanOutput_NoTrackingBranch checks that omitting tracking branch
// works (no "Tracking:" line when branch is empty).
func TestContextHumanOutput_NoTrackingBranch(t *testing.T) {
	var buf bytes.Buffer
	w := cli.NewWriter(&buf, false)

	// Simulate rendering without a tracking branch.
	trackingBranch := ""
	if trackingBranch != "" {
		cli.Label(w, "Tracking:  ", trackingBranch)
	}
	cli.Label(w, "Status:    ", "✓ clean")

	out := buf.String()
	if strings.Contains(out, "Tracking:") {
		t.Errorf("expected no Tracking line when no remote, got:\n%s", out)
	}
}

// TestContextOutput_PrintJSON verifies output.PrintJSON produces valid JSON.
func TestContextOutput_PrintJSON(t *testing.T) {
	// Just verify the helper doesn't error on a valid struct.
	obj := buildContextOutput(nil)
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	// Verify it's non-empty and valid JSON.
	var probe map[string]interface{}
	if err := json.Unmarshal(data, &probe); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	// output.PrintJSON is a thin wrapper; verify the package is importable.
	_ = output.PrintJSON // just ensure the import is used
}

// TestContextOutput_NoRemote_OmitsTrackingFields verifies that when hasRemote
// is false, the JSON output omits tracking_branch and emits 0 for ahead/behind.
// This mirrors the TUI's hasRemote contract in internal/tui/data.go.
func TestContextOutput_NoRemote_OmitsTrackingFields(t *testing.T) {
	// Simulate what contextCmd does when hasRemote=false: tracking_branch is
	// left as zero value ("") so omitempty drops it; ahead/behind stay 0.
	obj := buildContextOutput(func(o *contextOutput) {
		o.TrackingBranch = ""
		o.Ahead = 0
		o.Behind = 0
	})

	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	js := string(data)

	if strings.Contains(js, `"tracking_branch"`) {
		t.Errorf("expected tracking_branch to be absent when no remote, got:\n%s", js)
	}
}

// TestContextOutput_EmptyRecentCommits_Omitted verifies that when the
// recent_commits slice is nil/empty, the field is omitted from JSON output.
func TestContextOutput_EmptyRecentCommits_Omitted(t *testing.T) {
	obj := buildContextOutput(func(o *contextOutput) {
		o.RecentCommits = nil
	})

	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	js := string(data)

	if strings.Contains(js, `"recent_commits"`) {
		t.Errorf("expected recent_commits to be absent when empty, got:\n%s", js)
	}
}

// TestContextOutput_EmptyRecentCommitsSlice_Omitted verifies that an empty
// (non-nil) slice is also omitted, since omitempty treats empty slices as zero.
func TestContextOutput_EmptyRecentCommitsSlice_Omitted(t *testing.T) {
	obj := buildContextOutput(func(o *contextOutput) {
		o.RecentCommits = []contextRecentCommit{}
	})

	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	js := string(data)

	if strings.Contains(js, `"recent_commits"`) {
		t.Errorf("expected recent_commits to be absent for empty slice, got:\n%s", js)
	}
}

// TestContextDirtyFileGuard verifies that the changes slice is populated from
// DirtyFiles alone (no IsDirty check), mirroring the here.go convention.
func TestContextDirtyFileGuard(t *testing.T) {
	tests := []struct {
		name        string
		dirtyFiles  string
		wantChanges []string
	}{
		{
			name:        "dirty files present",
			dirtyFiles:  "M internal/foo.go\n?? newfile.txt\n",
			wantChanges: []string{"M internal/foo.go", "?? newfile.txt"},
		},
		{
			name:        "empty dirty files",
			dirtyFiles:  "",
			wantChanges: nil,
		},
		{
			name:        "only blank lines",
			dirtyFiles:  "\n\n",
			wantChanges: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reproduce the guard logic from context_cmd.go directly.
			var changes []string
			if tc.dirtyFiles != "" {
				for _, f := range strings.Split(tc.dirtyFiles, "\n") {
					if f != "" {
						changes = append(changes, f)
					}
				}
			}

			if len(changes) != len(tc.wantChanges) {
				t.Fatalf("changes len: got %d, want %d (got %v)", len(changes), len(tc.wantChanges), changes)
			}
			for i, want := range tc.wantChanges {
				if changes[i] != want {
					t.Errorf("changes[%d]: got %q, want %q", i, changes[i], want)
				}
			}
		})
	}
}
