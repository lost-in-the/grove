package docker

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/plugins"
)

func TestClassifyStack(t *testing.T) {
	run := func(n string) ServiceStatus { return ServiceStatus{Name: n, Status: ServiceRunning} }
	exitedErr := func(n string) ServiceStatus { return ServiceStatus{Name: n, Status: ServiceExitedError} }
	exitedOK := func(n string) ServiceStatus { return ServiceStatus{Name: n, Status: ServiceExitedClean} }
	unknown := func(n string) ServiceStatus { return ServiceStatus{Name: n, Status: ServiceUnknown} }

	tests := []struct {
		name        string
		statuses    []ServiceStatus
		probeErr    error
		services    []string
		nonBlocking []string
		want        StackState
		wantRunning int
		wantBlock   []string
	}{
		{name: "probe error is unknown, not stopped", probeErr: errors.New("timeout"), want: StackUnknown},
		{name: "no containers", want: StackStopped},
		{name: "stack-wide when no services configured", statuses: []ServiceStatus{run("app")}, want: StackRunning, wantRunning: 1},
		{
			name:     "shared infra alone does not count as running",
			statuses: []ServiceStatus{run("db"), run("redis"), exitedOK("app")},
			services: []string{"app"},
			want:     StackStopped,
		},
		{
			name:        "configured service running",
			statuses:    []ServiceStatus{run("db"), run("app")},
			services:    []string{"app"},
			want:        StackRunning,
			wantRunning: 1,
		},
		{
			name:     "configured service absent from ps is stopped",
			statuses: []ServiceStatus{run("db")},
			services: []string{"app"},
			want:     StackStopped,
		},
		{
			name:        "unrelated service in error does not flip the verdict",
			statuses:    []ServiceStatus{run("app"), exitedErr("mailer"), unknown("cache")},
			services:    []string{"app"},
			want:        StackRunning,
			wantRunning: 1,
		},
		{
			name:        "configured service in error is degraded",
			statuses:    []ServiceStatus{run("web"), exitedErr("app")},
			services:    []string{"web", "app"},
			want:        StackDegraded,
			wantRunning: 1,
			wantBlock:   []string{"app"},
		},
		{
			name:        "non-blocking configured service may fail",
			statuses:    []ServiceStatus{run("app"), exitedErr("assets")},
			services:    []string{"app", "assets"},
			nonBlocking: []string{"assets"},
			want:        StackRunning,
			wantRunning: 1,
		},
		{
			name:     "all configured exited clean is stopped",
			statuses: []ServiceStatus{exitedOK("migrate"), exitedOK("seed")},
			services: []string{"migrate", "seed"},
			want:     StackStopped,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyStack(tt.statuses, tt.probeErr, tt.services, tt.nonBlocking)
			if got.State != tt.want {
				t.Fatalf("state = %v, want %v", got.State, tt.want)
			}
			if got.Running != tt.wantRunning {
				t.Errorf("running = %d, want %d", got.Running, tt.wantRunning)
			}
			if len(got.Blockers) != len(tt.wantBlock) {
				t.Errorf("blockers = %v, want %v", got.Blockers, tt.wantBlock)
			}
		})
	}
}

// stubProbe swaps the docker probe for the test and restores it afterwards.
func stubProbe(t *testing.T, statuses []ServiceStatus, err error) {
	t.Helper()
	orig := probeHealth
	probeHealth = func(string, string, []string, time.Duration) ([]ServiceStatus, error) {
		return statuses, err
	}
	t.Cleanup(func() { probeHealth = orig })
}

func agentEnabledCfg(t *testing.T, composeDir, envBody string, slotsJSON string) *config.Config {
	t.Helper()
	if err := os.WriteFile(filepath.Join(composeDir, ".env"), []byte(envBody), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(composeDir, "agent"), 0755); err != nil {
		t.Fatal(err)
	}
	if slotsJSON != "" {
		if err := os.WriteFile(filepath.Join(composeDir, "agent", ".slots.json"), []byte(slotsJSON), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := externalCfg(composeDir)
	enabled := true
	cfg.Plugins.Docker.External.Services = []string{"app"}
	cfg.Plugins.Docker.External.Agent = &config.AgentStackConfig{
		Enabled:      &enabled,
		TemplatePath: "agent/compose.yml",
	}
	return cfg
}

func TestExternalServiceInfo_AgentEnabledWithoutSlotFallsThroughToSharedStack(t *testing.T) {
	dir := t.TempDir()
	wt := filepath.Join(t.TempDir(), "myapp-feature")
	cfg := agentEnabledCfg(t, dir, "APP_DIR="+wt+"\n", "")
	stubProbe(t, []ServiceStatus{{Name: "app", Status: ServiceRunning}}, nil)

	info := CurrentServiceInfo(cfg, wt)
	if info == nil {
		t.Fatal("expected info")
	}
	if info.Slot != 0 {
		t.Errorf("slot = %d, want 0", info.Slot)
	}
	if info.State != StackRunning || !info.MatchesCurrent || info.ConfiguredPath != wt {
		t.Errorf("want running shared stack configured for worktree, got %+v", info)
	}
}

func TestExternalServiceInfo_AgentSlotReportsAgentStack(t *testing.T) {
	dir := t.TempDir()
	wt := filepath.Join(t.TempDir(), "myapp-feature")
	cfg := agentEnabledCfg(t, dir, "APP_DIR=/elsewhere/myapp-main\n", `[{"slot":2,"worktree":"myapp-feature"}]`)
	stubProbe(t, nil, errors.New("must not be called for an agent stack"))

	info := CurrentServiceInfo(cfg, wt)
	if info == nil || info.Slot != 2 || !info.IsRunning || !info.MatchesCurrent {
		t.Fatalf("want agent stack in slot 2, got %+v", info)
	}
}

func TestExternalServiceInfo_UnsetEnvVarIsNotConfigured(t *testing.T) {
	dir := t.TempDir()
	cfg := externalCfg(dir)
	stubProbe(t, nil, errors.New("must not be called when nothing is configured"))

	info := CurrentServiceInfo(cfg, "/work/myapp-feature")
	if info == nil {
		t.Fatal("expected info")
	}
	if info.ConfiguredPath != "" || info.MatchesCurrent || info.IsRunning {
		t.Errorf("want unconfigured, got %+v", info)
	}
}

func TestExternalServiceInfo_ProbeErrorIsUnknown(t *testing.T) {
	dir := t.TempDir()
	cfg := agentEnabledCfg(t, dir, "APP_DIR=/work/myapp-feature\n", "")
	stubProbe(t, nil, errors.New("timeout"))

	info := CurrentServiceInfo(cfg, "/work/myapp-feature")
	if info == nil || info.State != StackUnknown || info.IsRunning {
		t.Fatalf("want unknown, got %+v", info)
	}
}

func TestExternalServiceInfo_InfraOnlyIsNotRunning(t *testing.T) {
	dir := t.TempDir()
	cfg := agentEnabledCfg(t, dir, "APP_DIR=/work/myapp-feature\n", "")
	stubProbe(t, []ServiceStatus{
		{Name: "db", Status: ServiceRunning},
		{Name: "app", Status: ServiceExitedClean},
	}, nil)

	info := CurrentServiceInfo(cfg, "/work/myapp-feature")
	if info == nil || info.State != StackStopped {
		t.Fatalf("want stopped, got %+v", info)
	}
}

func TestExternalStatuses_AgentSlotWorktreeShowsAgentEntryAndSharedOnlyForConfigured(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	slotted := filepath.Join(root, "myapp-feature")
	configured := filepath.Join(root, "myapp-main")
	cfg := agentEnabledCfg(t, dir, "APP_DIR="+configured+"\n", `[{"slot":1,"worktree":"myapp-feature"}]`)
	stubProbe(t, []ServiceStatus{{Name: "app", Status: ServiceRunning}}, nil)

	s := newExternalStrategy(cfg)
	got := externalStatuses(s, []string{slotted, configured})

	if e := got[slotted]; e.Short != "#1" || e.Level != plugins.StatusActive {
		t.Errorf("slotted worktree: want agent entry #1, got %+v", e)
	}
	if e := got[configured]; e.Short != "up" {
		t.Errorf("configured worktree: want shared 'up', got %+v", e)
	}
}

func TestExternalStatuses_ProbeErrorReportsConfiguredUnknown(t *testing.T) {
	dir := t.TempDir()
	wt := filepath.Join(t.TempDir(), "myapp-feature")
	cfg := agentEnabledCfg(t, dir, "APP_DIR="+wt+"\n", "")
	stubProbe(t, nil, errors.New("timeout"))

	got := externalStatuses(newExternalStrategy(cfg), []string{wt})
	e := got[wt]
	if e.Short != "configured" || e.Level != plugins.StatusInfo {
		t.Fatalf("want configured/info, got %+v", e)
	}
	if e.Detail == "" {
		t.Error("detail should say the state is unknown")
	}
}

func stubLocalProbe(t *testing.T, count int, err error) {
	t.Helper()
	orig := localPsCount
	localPsCount = func(string) (int, error) { return count, err }
	t.Cleanup(func() { localPsCount = orig })
}

func localComposeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLocalServiceInfo_ProbeErrorIsUnknown(t *testing.T) {
	dir := localComposeDir(t)
	stubLocalProbe(t, 0, errors.New("timeout"))

	info := localServiceInfo(dir)
	if info == nil || info.State != StackUnknown || info.IsRunning {
		t.Fatalf("want unknown, got %+v", info)
	}
}

func TestLocalServiceInfo_CountsDecideRunning(t *testing.T) {
	dir := localComposeDir(t)
	stubLocalProbe(t, 2, nil)
	if info := localServiceInfo(dir); info.State != StackRunning || !info.IsRunning {
		t.Errorf("want running, got %+v", info)
	}
	stubLocalProbe(t, 0, nil)
	if info := localServiceInfo(dir); info.State != StackStopped {
		t.Errorf("want stopped, got %+v", info)
	}
}

func TestLocalStatuses_ProbeErrorSaysUnknown(t *testing.T) {
	dir := localComposeDir(t)
	stubLocalProbe(t, 0, errors.New("timeout"))

	got := localStatuses(nil, []string{dir})
	e := got[dir]
	if e.Short != "compose" || e.Level != plugins.StatusInfo {
		t.Fatalf("want compose/info, got %+v", e)
	}
	if !strings.Contains(e.Detail, "unknown") {
		t.Errorf("detail should say unknown, got %q", e.Detail)
	}
}
