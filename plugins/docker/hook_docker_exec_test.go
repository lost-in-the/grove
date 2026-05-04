package docker

import (
	"bytes"
	"os"
	"os/exec"
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

// TestDockerExecHandler_Success exercises the happy path by running a real
// `docker exec` against a container that is actually running, OR — since
// we cannot rely on a live daemon in CI — we use the fake-binary trick:
// put a script named "docker" on PATH that exits 0 and prints to stdout.
func TestDockerExecHandler_Success(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	// Build a fake `docker` binary that:
	//   - prints "running" when called as `docker inspect -f ... <container>`
	//   - exits 0 and prints a marker line for any other invocation
	fakeDockerDir := t.TempDir()
	fakeDockerScript := `#!/bin/sh
if [ "$1" = "inspect" ]; then
  echo "true"
  exit 0
fi
echo "fake docker exec ok"
exit 0
`
	fakeDockerPath := fakeDockerDir + "/docker"
	if err := os.WriteFile(fakeDockerPath, []byte(fakeDockerScript), 0755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeDockerDir+":"+origPath)

	var buf bytes.Buffer
	p := &Plugin{enabled: true}
	err := p.dockerExecHandler(
		&hooks.HookAction{Container: "my-container", Command: "true"},
		&hooks.ExecutionContext{Output: &buf},
		&hooks.Variables{},
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(buf.String(), "my-container") {
		t.Errorf("expected container name in success output, got %q", buf.String())
	}
}

// TestDockerExecHandler_NonZeroExit verifies that a failing docker exec is
// wrapped and returned as an error.
func TestDockerExecHandler_NonZeroExit(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	fakeDockerDir := t.TempDir()
	fakeDockerScript := `#!/bin/sh
if [ "$1" = "inspect" ]; then
  echo "true"
  exit 0
fi
exit 42
`
	fakeDockerPath := fakeDockerDir + "/docker"
	if err := os.WriteFile(fakeDockerPath, []byte(fakeDockerScript), 0755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeDockerDir+":"+origPath)

	p := &Plugin{enabled: true}
	err := p.dockerExecHandler(
		&hooks.HookAction{Container: "my-container", Command: "fail-cmd"},
		&hooks.ExecutionContext{},
		&hooks.Variables{},
	)
	if err == nil {
		t.Fatal("expected error for non-zero docker exec exit, got nil")
	}
	if !strings.Contains(err.Error(), "my-container") {
		t.Errorf("error should reference container name, got %v", err)
	}
}
