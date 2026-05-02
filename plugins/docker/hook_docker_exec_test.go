package docker

import (
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/hooks"
)

func TestDockerExecHandler_RequiresContainer(t *testing.T) {
	p := &Plugin{enabled: true}
	err := p.dockerExecHandler(
		&hooks.HookAction{Command: "x"},
		&hooks.ExecutionContext{},
		&hooks.Variables{},
	)
	if err == nil || !strings.Contains(err.Error(), "container") {
		t.Fatalf("expected 'container required' error, got %v", err)
	}
}

func TestDockerExecHandler_RequiresCommand(t *testing.T) {
	p := &Plugin{enabled: true}
	err := p.dockerExecHandler(
		&hooks.HookAction{Container: "foo"},
		&hooks.ExecutionContext{},
		&hooks.Variables{},
	)
	if err == nil || !strings.Contains(err.Error(), "command") {
		t.Fatalf("expected 'command required' error, got %v", err)
	}
}

func TestDockerExecHandler_RejectsQuotedShell(t *testing.T) {
	// strings.Fields can't preserve quoted args. Reject the input rather than
	// silently shatter "sh -lc 'cd /app && true'" into broken tokens.
	p := &Plugin{enabled: true}
	err := p.dockerExecHandler(
		&hooks.HookAction{Container: "x", Command: "true", Shell: "sh -lc 'cd /app && true'"},
		&hooks.ExecutionContext{},
		&hooks.Variables{},
	)
	if err == nil || !strings.Contains(err.Error(), "no quotes") {
		t.Fatalf("expected quote-rejection error, got %v", err)
	}
}

func TestDockerExecHandler_InterpolatesContainer(t *testing.T) {
	// Container field interpolation — e.g. container = "{{.worktree}}-shell".
	// We only check the input-validation surface (container resolves to a
	// non-empty name); we'd need a running docker daemon to assert against
	// containerRunning. The "not running" error confirms the interpolated
	// name reached the runtime check.
	p := &Plugin{enabled: true}
	err := p.dockerExecHandler(
		&hooks.HookAction{Container: "{{.worktree}}-shell", Command: "true"},
		&hooks.ExecutionContext{},
		&hooks.Variables{Worktree: "grove-test-zzz"},
	)
	if err == nil {
		t.Fatal("expected error (container won't be running)")
	}
	if !strings.Contains(err.Error(), "grove-test-zzz-shell") {
		t.Fatalf("expected interpolated name in error, got %v", err)
	}
}

func TestDockerExecHandler_NotRunningContainer(t *testing.T) {
	// containerRunning() shells out to `docker inspect`. For a guaranteed-not-
	// running container name, the command returns false → handler must surface
	// the actionable error before attempting docker exec.
	p := &Plugin{enabled: true}
	err := p.dockerExecHandler(
		&hooks.HookAction{Container: "grove-test-nonexistent-container-zzz", Command: "true"},
		&hooks.ExecutionContext{},
		&hooks.Variables{},
	)
	if err == nil {
		t.Fatal("expected error for non-running container")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Fatalf("error should explain not-running state, got %v", err)
	}
}
