package commands

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/plugins/docker"
)

func TestDescribeServices(t *testing.T) {
	name := func(p string) string { return filepath.Base(p) }
	feature := "/work/myapp-feature"
	main := "/work/myapp-main"

	tests := []struct {
		name      string
		info      docker.ServiceInfo
		wantText  string
		wantLevel cli.StatusLevel
		wantState string
		wantHint  bool
	}{
		{
			name:      "shared stack running for this worktree",
			info:      docker.ServiceInfo{Shared: true, ConfiguredPath: feature, State: docker.StackRunning, IsRunning: true, MatchesCurrent: true},
			wantText:  "running, on myapp-feature",
			wantLevel: cli.StatusActive,
			wantState: "running",
			wantHint:  true,
		},
		{
			name:      "shared stack running for another worktree",
			info:      docker.ServiceInfo{Shared: true, ConfiguredPath: main, State: docker.StackRunning, IsRunning: true},
			wantText:  "running, on myapp-main (not here)",
			wantLevel: cli.StatusWarning,
			wantState: "running",
			wantHint:  true,
		},
		{
			name:      "stopped",
			info:      docker.ServiceInfo{Shared: true, ConfiguredPath: feature, State: docker.StackStopped, MatchesCurrent: true},
			wantText:  "not running, on myapp-feature",
			wantLevel: cli.StatusError,
			wantState: "stopped",
		},
		{
			name:      "unknown is not reported as not running",
			info:      docker.ServiceInfo{Shared: true, ConfiguredPath: feature, State: docker.StackUnknown, MatchesCurrent: true},
			wantText:  "state unknown (docker did not answer), on myapp-feature",
			wantLevel: cli.StatusWarning,
			wantState: "unknown",
			wantHint:  true,
		},
		{
			name:      "degraded names the failing services",
			info:      docker.ServiceInfo{Shared: true, ConfiguredPath: feature, State: docker.StackDegraded, Blockers: []string{"app", "worker"}, MatchesCurrent: true},
			wantText:  "degraded (app, worker failed), on myapp-feature",
			wantLevel: cli.StatusWarning,
			wantState: "degraded",
			wantHint:  true,
		},
		{
			name:      "unset env var prints no empty name",
			info:      docker.ServiceInfo{Shared: true},
			wantText:  "not on any worktree",
			wantLevel: cli.StatusWarning,
			wantState: "not_configured",
		},
		{
			name:      "agent stack",
			info:      docker.ServiceInfo{Shared: false, ConfiguredPath: feature, State: docker.StackRunning, IsRunning: true, MatchesCurrent: true, Slot: 2},
			wantText:  "agent stack #2",
			wantLevel: cli.StatusActive,
			wantState: "agent",
		},
		{
			name:      "local compose running",
			info:      docker.ServiceInfo{ConfiguredPath: feature, State: docker.StackRunning, IsRunning: true, MatchesCurrent: true},
			wantText:  "running",
			wantLevel: cli.StatusActive,
			wantState: "running",
		},
		{
			name:      "local compose unknown",
			info:      docker.ServiceInfo{ConfiguredPath: feature, State: docker.StackUnknown, MatchesCurrent: true},
			wantText:  "state unknown (docker did not answer)",
			wantLevel: cli.StatusWarning,
			wantState: "unknown",
		},
		{
			name:      "local compose stopped",
			info:      docker.ServiceInfo{ConfiguredPath: feature, State: docker.StackStopped, MatchesCurrent: true},
			wantText:  "not running",
			wantLevel: cli.StatusError,
			wantState: "stopped",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := describeServices(&tt.info, name)
			if got.Text != tt.wantText {
				t.Errorf("text = %q, want %q", got.Text, tt.wantText)
			}
			if got.Level != tt.wantLevel {
				t.Errorf("level = %v, want %v", got.Level, tt.wantLevel)
			}
			if got.State != tt.wantState {
				t.Errorf("state = %q, want %q", got.State, tt.wantState)
			}
			if (got.Hint != "") != tt.wantHint {
				t.Errorf("hint = %q, wantHint %v", got.Hint, tt.wantHint)
			}
		})
	}
}

func TestMountCheckHintWording(t *testing.T) {
	want := "This is the env file setting. Run 'grove here --check-mount' to see what the containers have mounted."
	if mountCheckHint != want {
		t.Errorf("hint = %q", mountCheckHint)
	}
}

func TestWhichServiceOutput_JSONFieldNames(t *testing.T) {
	raw, err := json.Marshal(whichServiceOutput{State: "running", StackOn: "feature", MatchesCurrent: true, Slot: 0})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	if m["stack_on"] != "feature" {
		t.Errorf("stack_on = %v, want feature (got %s)", m["stack_on"], raw)
	}
	if _, old := m["configured_for"]; old {
		t.Errorf("configured_for should be gone, got %s", raw)
	}
}
