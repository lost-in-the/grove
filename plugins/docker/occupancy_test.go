package docker

import (
	"os"
	"os/exec"
	"testing"
)

func TestParseAgentSlot(t *testing.T) {
	tests := []struct {
		name     string
		project  string
		wantSlot int
		wantOK   bool
	}{
		{"numbered slot", "myapp-agent-1", 1, true},
		{"numbered slot double digit", "myapp-agent-12", 12, true},
		{"ephemeral is not a slot", "myapp-agent-ephemeral", 0, false},
		{"unrelated project", "myapp-main", 0, false},
		{"project name itself contains agent", "my-agent-app-agent-3", 3, true},
		{"zero slot rejected", "myapp-agent-0", 0, false},
		{"negative-looking suffix rejected", "myapp-agent--1", 0, false},
		{"empty suffix rejected", "myapp-agent-", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slot, ok := parseAgentSlot(tt.project)
			if ok != tt.wantOK || slot != tt.wantSlot {
				t.Errorf("parseAgentSlot(%q) = (%d, %v), want (%d, %v)", tt.project, slot, ok, tt.wantSlot, tt.wantOK)
			}
		})
	}
}

func TestParseStackOccupants(t *testing.T) {
	composePath := "/stack/shared-infra"

	out := "myapp-agent-1|/stack/shared-infra\n" +
		"otherapp-agent-2|/stack/shared-infra\n" +
		"myapp-agent-ephemeral|/stack/shared-infra\n" +
		"unrelated-project|/some/other/path\n" +
		"\n"

	got := parseStackOccupants(out, composePath)

	want := map[int]string{
		1: "myapp-agent-1",
		2: "otherapp-agent-2",
	}
	if len(got) != len(want) {
		t.Fatalf("parseStackOccupants() = %v, want %v", got, want)
	}
	for slot, project := range want {
		if got[slot] != project {
			t.Errorf("slot %d = %q, want %q", slot, got[slot], project)
		}
	}
}

func TestParseStackOccupants_MatchesTrailingSlashVariants(t *testing.T) {
	out := "myapp-agent-1|/stack/shared-infra/\n"
	got := parseStackOccupants(out, "/stack/shared-infra")
	if got[1] != "myapp-agent-1" {
		t.Errorf("expected slot 1 to match despite trailing slash, got %v", got)
	}
}

func TestParseStackOccupants_IgnoresMalformedLines(t *testing.T) {
	out := "no-pipe-here\n|\nmyapp-agent-1|\n"
	got := parseStackOccupants(out, "/stack/shared-infra")
	if len(got) != 0 {
		t.Errorf("expected no occupants from malformed lines, got %v", got)
	}
}

func TestForeignOccupants(t *testing.T) {
	occupants := map[int]string{
		1: "myapp-agent-1",    // this project's own slot 1
		2: "otherapp-agent-2", // a different project's slot 2
	}
	got := foreignOccupants(occupants, "myapp")
	if len(got) != 1 {
		t.Fatalf("foreignOccupants() = %v, want exactly one foreign entry", got)
	}
	if got[2] != "otherapp-agent-2" {
		t.Errorf("foreignOccupants()[2] = %q, want %q", got[2], "otherapp-agent-2")
	}
}

func TestForeignOccupants_EmptyWhenAllOwnedByCaller(t *testing.T) {
	occupants := map[int]string{
		1: "myapp-agent-1",
		2: "myapp-agent-2",
	}
	got := foreignOccupants(occupants, "myapp")
	if len(got) != 0 {
		t.Errorf("foreignOccupants() = %v, want empty", got)
	}
}

func TestForeignOccupants_NilInputReturnsNil(t *testing.T) {
	if got := foreignOccupants(nil, "myapp"); got != nil {
		t.Errorf("foreignOccupants(nil, ...) = %v, want nil", got)
	}
}

// TestStackOccupants_CrossProjectConflict exercises stackOccupants end to end
// against a fake `docker` binary on PATH (matching the pattern used in
// hook_docker_exec_test.go), simulating two grove projects — "myapp" and
// "otherapp" — both mounting the same shared external compose path. It
// confirms the cross-project slot is detected via the working_dir label
// correlation, independent of either project's own compose project name.
func TestStackOccupants_CrossProjectConflict(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	composePath := t.TempDir()

	fakeDockerDir := t.TempDir()
	fakeDockerScript := `#!/bin/sh
echo 'myapp-agent-1|` + composePath + `'
echo 'otherapp-agent-2|` + composePath + `'
echo 'unrelated-thing|/somewhere/else'
exit 0
`
	fakeDockerPath := fakeDockerDir + "/docker"
	if err := os.WriteFile(fakeDockerPath, []byte(fakeDockerScript), 0755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeDockerDir+":"+origPath)

	occupants, err := stackOccupants(composePath)
	if err != nil {
		t.Fatalf("stackOccupants() error = %v", err)
	}

	if occupants[1] != "myapp-agent-1" {
		t.Errorf("occupants[1] = %q, want %q", occupants[1], "myapp-agent-1")
	}
	if occupants[2] != "otherapp-agent-2" {
		t.Errorf("occupants[2] = %q, want %q", occupants[2], "otherapp-agent-2")
	}

	foreign := foreignOccupants(occupants, "myapp")
	if len(foreign) != 1 || foreign[2] != "otherapp-agent-2" {
		t.Errorf("foreignOccupants() from myapp's perspective = %v, want only slot 2 held by otherapp-agent-2", foreign)
	}
}
