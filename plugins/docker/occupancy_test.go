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

	out := "myapp-agent-1|/stack/shared-infra|running\n" +
		"otherapp-agent-2|/stack/shared-infra|running\n" +
		"myapp-agent-ephemeral|/stack/shared-infra|running\n" +
		"unrelated-project|/some/other/path|running\n" +
		"\n"

	got := parseStackOccupants(out, composePath)

	want := map[int]stackOccupant{
		1: {Project: "myapp-agent-1", Running: true},
		2: {Project: "otherapp-agent-2", Running: true},
	}
	if len(got) != len(want) {
		t.Fatalf("parseStackOccupants() = %v, want %v", got, want)
	}
	for slot, occ := range want {
		if got[slot] != occ {
			t.Errorf("slot %d = %+v, want %+v", slot, got[slot], occ)
		}
	}
}

func TestParseStackOccupants_MatchesTrailingSlashVariants(t *testing.T) {
	out := "myapp-agent-1|/stack/shared-infra/|running\n"
	got := parseStackOccupants(out, "/stack/shared-infra")
	if got[1].Project != "myapp-agent-1" {
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

// TestParseStackOccupants_ExitedContainerStillOccupiesSlot confirms a slot
// whose only container has exited (but hasn't been removed) still counts as
// occupied — allocation must keep excluding it, since docker still refuses a
// second container with the same pinned container_name (#147 follow-up).
func TestParseStackOccupants_ExitedContainerStillOccupiesSlot(t *testing.T) {
	out := "otherapp-agent-1|/stack/shared-infra|exited\n"
	got := parseStackOccupants(out, "/stack/shared-infra")
	occ, ok := got[1]
	if !ok {
		t.Fatalf("parseStackOccupants() = %v, want slot 1 present (occupied but stopped)", got)
	}
	if occ.Project != "otherapp-agent-1" || occ.Running {
		t.Errorf("slot 1 = %+v, want Project=otherapp-agent-1 Running=false", occ)
	}
}

// TestParseStackOccupants_MixedRunningAndExitedCountsAsRunning confirms a
// compose project with one running and one exited container (e.g. mid
// restart) is reported as Running — any live container is enough.
func TestParseStackOccupants_MixedRunningAndExitedCountsAsRunning(t *testing.T) {
	out := "myapp-agent-1|/stack/shared-infra|exited\n" +
		"myapp-agent-1|/stack/shared-infra|running\n"
	got := parseStackOccupants(out, "/stack/shared-infra")
	occ, ok := got[1]
	if !ok || !occ.Running {
		t.Errorf("slot 1 = %+v, want Running=true when any container in the project is running", occ)
	}

	// Order shouldn't matter — running-then-exited should also stay Running.
	out2 := "myapp-agent-1|/stack/shared-infra|running\n" +
		"myapp-agent-1|/stack/shared-infra|exited\n"
	got2 := parseStackOccupants(out2, "/stack/shared-infra")
	if occ2 := got2[1]; !occ2.Running {
		t.Errorf("slot 1 = %+v, want Running=true regardless of line order", occ2)
	}
}

func TestForeignOccupants(t *testing.T) {
	occupants := map[int]stackOccupant{
		1: {Project: "myapp-agent-1", Running: true},    // this project's own slot 1
		2: {Project: "otherapp-agent-2", Running: true}, // a different project's slot 2
	}
	got := foreignOccupants(occupants, "myapp")
	if len(got) != 1 {
		t.Fatalf("foreignOccupants() = %v, want exactly one foreign entry", got)
	}
	if got[2].Project != "otherapp-agent-2" {
		t.Errorf("foreignOccupants()[2] = %+v, want Project %q", got[2], "otherapp-agent-2")
	}
}

func TestForeignOccupants_EmptyWhenAllOwnedByCaller(t *testing.T) {
	occupants := map[int]stackOccupant{
		1: {Project: "myapp-agent-1", Running: true},
		2: {Project: "myapp-agent-2", Running: true},
	}
	got := foreignOccupants(occupants, "myapp")
	if len(got) != 0 {
		t.Errorf("foreignOccupants() = %v, want empty", got)
	}
}

// TestForeignOccupants_StoppedForeignSlotStillExcluded confirms a foreign
// slot held only by a stopped container is still reported by
// foreignOccupants — it must keep blocking allocation even though it isn't
// live (see the comment on foreignOccupants and its allocation call site).
func TestForeignOccupants_StoppedForeignSlotStillExcluded(t *testing.T) {
	occupants := map[int]stackOccupant{
		1: {Project: "otherapp-agent-1", Running: false},
	}
	got := foreignOccupants(occupants, "myapp")
	occ, ok := got[1]
	if !ok || occ.Running {
		t.Fatalf("foreignOccupants() = %v, want slot 1 present with Running=false", got)
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
echo 'myapp-agent-1|` + composePath + `|running'
echo 'otherapp-agent-2|` + composePath + `|running'
echo 'unrelated-thing|/somewhere/else|running'
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

	if occupants[1].Project != "myapp-agent-1" {
		t.Errorf("occupants[1] = %+v, want Project %q", occupants[1], "myapp-agent-1")
	}
	if occupants[2].Project != "otherapp-agent-2" {
		t.Errorf("occupants[2] = %+v, want Project %q", occupants[2], "otherapp-agent-2")
	}

	foreign := foreignOccupants(occupants, "myapp")
	if len(foreign) != 1 || foreign[2].Project != "otherapp-agent-2" {
		t.Errorf("foreignOccupants() from myapp's perspective = %v, want only slot 2 held by otherapp-agent-2", foreign)
	}
}
