package docker

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/hooks"
)

// fakeStrategy records strategy method invocations for hook handler tests.
type fakeStrategy struct {
	upCalled    int
	upErr       error
	runCalled   int
	runService  string
	runCommand  string
	runEnv      []string
	runErr      error
	execCalled  int
	execService string
	execCommand string
	execEnv     []string
	execErr     error
}

func (f *fakeStrategy) OnPreSwitch(_ *hooks.Context) error  { return nil }
func (f *fakeStrategy) OnPostSwitch(_ *hooks.Context) error { return nil }
func (f *fakeStrategy) OnPostCreate(_ *hooks.Context) error { return nil }
func (f *fakeStrategy) Up(_ string, _ bool) error {
	f.upCalled++
	return f.upErr
}
func (f *fakeStrategy) Down(_ string) error                   { return nil }
func (f *fakeStrategy) Logs(_ string, _ string, _ bool) error { return nil }
func (f *fakeStrategy) Restart(_ string, _ string) error      { return nil }
func (f *fakeStrategy) Run(_ string, service, command string, hookEnv []string) error {
	f.runCalled++
	f.runService = service
	f.runCommand = command
	f.runEnv = hookEnv
	return f.runErr
}
func (f *fakeStrategy) Exec(_ string, service, command string, hookEnv []string) error {
	f.execCalled++
	f.execService = service
	f.execCommand = command
	f.execEnv = hookEnv
	return f.execErr
}

func newFakePlugin() (*Plugin, *fakeStrategy) {
	fs := &fakeStrategy{}
	return &Plugin{enabled: true, strategy: fs}, fs
}

func TestComposeHandler_RunMode_CallsRun(t *testing.T) {
	p, fs := newFakePlugin()
	var buf bytes.Buffer
	action := &hooks.HookAction{Type: "docker:compose", Service: "web", Command: "bundle install"}
	ctx := &hooks.ExecutionContext{NewPath: "/tmp/wt", Output: &buf}
	vars := &hooks.Variables{}

	if err := p.composeHandler(action, ctx, vars); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.runCalled != 1 {
		t.Errorf("expected Run called once, got %d", fs.runCalled)
	}
	if fs.upCalled != 0 {
		t.Errorf("Up should NOT be called for mode=run; got %d", fs.upCalled)
	}
	if fs.runService != "web" || fs.runCommand != "bundle install" {
		t.Errorf("wrong call args: service=%q cmd=%q", fs.runService, fs.runCommand)
	}
	if !strings.Contains(buf.String(), "service: web") {
		t.Errorf("expected status line, got %q", buf.String())
	}
}

// TestComposeHandler_InterpolatesByReference guards B13 for docker:compose:
// a hostile value in {{.branch}} (grove checks branches out from untrusted
// PRs) must reach the container as environment, never spliced into the
// `bash -cil` command where it could execute.
func TestComposeHandler_InterpolatesByReference(t *testing.T) {
	for _, mode := range []string{"run", "exec"} {
		t.Run(mode, func(t *testing.T) {
			p, fs := newFakePlugin()
			var buf bytes.Buffer
			action := &hooks.HookAction{
				Type:    "docker:compose",
				Service: "app",
				Command: "deploy {{.branch}}",
				Mode:    mode,
			}
			ctx := &hooks.ExecutionContext{NewPath: "/tmp/wt", Output: &buf}
			vars := &hooks.Variables{Branch: `x$(touch /tmp/pwned)`}

			if err := p.composeHandler(action, ctx, vars); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotCmd := fs.runCommand
			gotEnv := fs.runEnv
			if mode == "exec" {
				gotCmd = fs.execCommand
				gotEnv = fs.execEnv
			}
			if strings.Contains(gotCmd, "touch") {
				t.Errorf("raw branch value leaked into container command: %q", gotCmd)
			}
			if !strings.Contains(gotCmd, "${GROVE_HOOK_branch}") {
				t.Errorf("command did not reference the hook env var: %q", gotCmd)
			}
			wantEnv := `GROVE_HOOK_branch=x$(touch /tmp/pwned)`
			found := false
			for _, e := range gotEnv {
				if e == wantEnv {
					found = true
				}
			}
			if !found {
				t.Errorf("hook value not passed as container env %q; got %v", wantEnv, gotEnv)
			}
		})
	}
}

// TestDockerEnvArgs_ValueIsOneArgv confirms a value with shell metacharacters
// becomes a single `-e KEY=VALUE` argv element (handed to docker, never a
// shell), so it cannot be re-parsed and injected.
func TestDockerEnvArgs_ValueIsOneArgv(t *testing.T) {
	got := dockerEnvArgs([]string{`GROVE_HOOK_branch=x$(id);rm -rf /`})
	want := []string{"-e", `GROVE_HOOK_branch=x$(id);rm -rf /`}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("dockerEnvArgs = %v, want %v", got, want)
	}
	if dockerEnvArgs(nil) != nil {
		t.Errorf("dockerEnvArgs(nil) should be nil")
	}
}

func TestComposeHandler_ExecMode_CallsExecOnly(t *testing.T) {
	// After hook-ordering inversion (plugin Go hooks fire before config
	// hooks), the compose handler no longer self-Ups for mode=exec. The
	// container is presumed up by the time this runs.
	p, fs := newFakePlugin()
	action := &hooks.HookAction{Type: "docker:compose", Service: "app", Command: "rails db:migrate", Mode: "exec"}
	ctx := &hooks.ExecutionContext{NewPath: "/tmp/wt"}

	if err := p.composeHandler(action, ctx, &hooks.Variables{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.upCalled != 0 {
		t.Errorf("Up should NOT be called from the handler; got %d", fs.upCalled)
	}
	if fs.execCalled != 1 {
		t.Errorf("Exec should be called once; got %d", fs.execCalled)
	}
	if fs.runCalled != 0 {
		t.Errorf("Run should NOT be called for mode=exec; got %d", fs.runCalled)
	}
}

func TestComposeHandler_ExecMode_NotRunningIncludesActionableHint(t *testing.T) {
	p, fs := newFakePlugin()
	fs.execErr = errors.New("service is not running")
	action := &hooks.HookAction{Type: "docker:compose", Service: "app", Command: "true", Mode: "exec"}
	ctx := &hooks.ExecutionContext{NewPath: "/tmp/wt"}

	err := p.composeHandler(action, ctx, &hooks.Variables{})
	if err == nil {
		t.Fatal("expected error when Exec fails")
	}
	if !strings.Contains(err.Error(), "stack up") {
		t.Fatalf("expected hint about starting the stack, got %v", err)
	}
}

func TestComposeHandler_NoComposeFileHintsDockerExec(t *testing.T) {
	p, fs := newFakePlugin()
	fs.runErr = ErrNoComposeFile
	action := &hooks.HookAction{Type: "docker:compose", Service: "app", Command: "true"}
	ctx := &hooks.ExecutionContext{NewPath: "/tmp/wt"}

	err := p.composeHandler(action, ctx, &hooks.Variables{})
	if err == nil {
		t.Fatal("expected error when no compose file")
	}
	if !strings.Contains(err.Error(), `docker:exec`) {
		t.Fatalf("expected hint suggesting docker:exec, got %v", err)
	}
}

func TestComposeHandler_DisabledPlugin(t *testing.T) {
	p := &Plugin{enabled: false}
	err := p.composeHandler(
		&hooks.HookAction{Service: "app", Command: "x"},
		&hooks.ExecutionContext{},
		&hooks.Variables{},
	)
	if err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("expected disabled-plugin error, got %v", err)
	}
}

func TestComposeHandler_RequiresService(t *testing.T) {
	p, _ := newFakePlugin()
	err := p.composeHandler(
		&hooks.HookAction{Command: "x"},
		&hooks.ExecutionContext{NewPath: "/tmp/wt"},
		&hooks.Variables{},
	)
	if err == nil || !strings.Contains(err.Error(), "service") {
		t.Fatalf("expected 'service required' error, got %v", err)
	}
}

func TestComposeHandler_RequiresCommand(t *testing.T) {
	p, _ := newFakePlugin()
	err := p.composeHandler(
		&hooks.HookAction{Service: "app"},
		&hooks.ExecutionContext{NewPath: "/tmp/wt"},
		&hooks.Variables{},
	)
	if err == nil || !strings.Contains(err.Error(), "command") {
		t.Fatalf("expected 'command required' error, got %v", err)
	}
}

func TestComposeHandler_InvalidMode(t *testing.T) {
	p, _ := newFakePlugin()
	err := p.composeHandler(
		&hooks.HookAction{Service: "app", Command: "x", Mode: "weird"},
		&hooks.ExecutionContext{NewPath: "/tmp/wt"},
		&hooks.Variables{},
	)
	if err == nil || !strings.Contains(err.Error(), "weird") {
		t.Fatalf("expected mode-validation error, got %v", err)
	}
}

func TestComposeHandler_RunError(t *testing.T) {
	// fakeStrategy.runErr is defined but was never exercised. Verify that a
	// strategy-level run error propagates through composeHandler.
	p, fs := newFakePlugin()
	syntheticErr := errors.New("bundle install failed with exit 1")
	fs.runErr = syntheticErr
	action := &hooks.HookAction{Type: "docker:compose", Service: "web", Command: "bundle install"}
	ctx := &hooks.ExecutionContext{NewPath: "/tmp/wt"}

	err := p.composeHandler(action, ctx, &hooks.Variables{})
	if err == nil {
		t.Fatal("expected error when Run fails, got nil")
	}
	if !strings.Contains(err.Error(), "web") {
		t.Errorf("error should reference the service name, got %v", err)
	}
}

func TestComposeHandler_ExecError(t *testing.T) {
	// Symmetric test for exec mode: strategy-level exec error must propagate.
	p, fs := newFakePlugin()
	syntheticErr := errors.New("rails db:migrate failed")
	fs.execErr = syntheticErr
	action := &hooks.HookAction{Type: "docker:compose", Service: "app", Command: "rails db:migrate", Mode: "exec"}
	ctx := &hooks.ExecutionContext{NewPath: "/tmp/wt"}

	err := p.composeHandler(action, ctx, &hooks.Variables{})
	if err == nil {
		t.Fatal("expected error when Exec fails, got nil")
	}
	if !strings.Contains(err.Error(), "app") {
		t.Errorf("error should reference the service name, got %v", err)
	}
}

func TestComposeHandler_VariableInterpolation(t *testing.T) {
	// Interpolation applies to service AND command (both can be per-worktree).
	// The service name is a literal argv element to docker (safe), so it is
	// substituted directly; the command is executed by `bash -cil` inside the
	// container, so it is rewritten to a GROVE_HOOK_* reference with the value
	// carried as container env (B13).
	p, fs := newFakePlugin()
	action := &hooks.HookAction{Service: "{{.worktree}}-app", Command: "echo {{.worktree}}"}
	ctx := &hooks.ExecutionContext{NewPath: "/tmp/wt"}
	vars := &hooks.Variables{Worktree: "feature-x"}

	if err := p.composeHandler(action, ctx, vars); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.runService != "feature-x-app" {
		t.Errorf("expected interpolated service, got %q", fs.runService)
	}
	if fs.runCommand != `echo "${GROVE_HOOK_worktree}"` {
		t.Errorf("expected reference-rewritten command, got %q", fs.runCommand)
	}
	wantEnv := "GROVE_HOOK_worktree=feature-x"
	found := false
	for _, e := range fs.runEnv {
		if e == wantEnv {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %q in container env, got %v", wantEnv, fs.runEnv)
	}
}
