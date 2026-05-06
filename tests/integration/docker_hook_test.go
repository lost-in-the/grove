//go:build integration

// Package integration_test scenario 1: a `docker:compose` hook with mode = "run"
// actually fires a command inside a real container managed by docker compose.
//
// This is the only integration scenario that requires a live Docker daemon. It is
// skipped automatically if Docker is unreachable so the suite stays runnable on
// machines without Docker installed.
package integration_test

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/plugins/docker"
)

func TestDockerComposeHook_RunMode_FiresCommandInContainer(t *testing.T) {
	requireDocker(t)

	worktree := setupSimpleComposeFixture(t)

	// Make sure any leftover containers/volumes from a previous run don't
	// linger if this test fails.
	t.Cleanup(func() {
		cmd := exec.Command("docker", "compose", "down", "--remove-orphans", "--volumes")
		cmd.Dir = worktree
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		_ = cmd.Run()
	})

	plugin := docker.New()
	if err := plugin.Init(&config.Config{}); err != nil {
		t.Fatalf("plugin init: %v", err)
	}

	handler, ok := hooks.LookupActionHandler("docker:compose")
	if !ok {
		t.Fatal("docker:compose handler not registered after plugin init")
	}

	// Use a compose-mounted host file as the assertion vector. The container
	// mounts the worktree at /app, so a write to /app/hook.out lands on the
	// host filesystem where the test process can read it.
	action := &hooks.HookAction{
		Type:    "docker:compose",
		Service: "web",
		Command: "printf 'HOOK_OK' > /app/hook.out",
		Mode:    "run",
	}

	ctx := &hooks.ExecutionContext{
		Event:    "post_create",
		MainPath: worktree,
		NewPath:  worktree,
	}

	if err := handler(action, ctx, &hooks.Variables{}); err != nil {
		t.Fatalf("docker:compose hook returned error: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(worktree, "hook.out"))
	if err != nil {
		t.Fatalf("hook.out not produced inside container volume mount: %v", err)
	}
	if got := string(out); got != "HOOK_OK" {
		t.Errorf("hook.out = %q, want %q", got, "HOOK_OK")
	}
}

// requireDocker skips the test unless a Docker daemon is reachable.
func requireDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI not installed")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not reachable")
	}
}

// setupSimpleComposeFixture copies testdata/fixtures/simple-compose into a
// temp dir and returns the absolute path. The fixture is a single-service
// alpine compose stack with a host volume mount so test assertions can read
// files the container wrote.
func setupSimpleComposeFixture(t *testing.T) string {
	t.Helper()

	root, err := repoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	src := filepath.Join(root, "testdata", "fixtures", "simple-compose")

	dst := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0644); err != nil {
			t.Fatalf("write %s: %v", e.Name(), err)
		}
	}
	return dst
}

// repoRoot finds the grove project root by walking up from the test binary's
// working directory until a go.mod is found.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

var _ = runtime.GOOS // keep runtime import available for future GOOS-scoped skips
