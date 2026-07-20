package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/lost-in-the/grove/internal/hooks"
)

// dockerExecHandler runs a command inside an externally-managed container
// (one grove doesn't lifecycle, e.g. a long-running dev shell). Bypasses
// compose entirely and uses raw `docker exec`.
func (p *Plugin) dockerExecHandler(action *hooks.HookAction, ctx *hooks.ExecutionContext, vars *hooks.Variables) error {
	container := vars.Interpolate(action.Container)
	if container == "" {
		return fmt.Errorf("docker:exec hook: 'container' is required")
	}
	// Reference GROVE_HOOK_* env vars instead of splicing values into the shell
	// command; the values ride along as `docker exec -e` environment so an
	// untrusted {{.branch}} cannot inject a command (B13).
	command := vars.InterpolateShell(action.Command)
	if command == "" {
		return fmt.Errorf("docker:exec hook: 'command' is required")
	}

	shell := strings.TrimSpace(vars.Interpolate(action.Shell))
	if shell == "" {
		shell = "bash -lc"
	}
	// Reject quoted args — strings.Fields can't preserve them and partial
	// support is worse than none. Users with complex shell needs should put
	// the logic in a wrapper script and invoke it.
	if strings.ContainsAny(shell, `'"`) {
		return fmt.Errorf("docker:exec hook: 'shell' must be a plain command + flags (no quotes); got %q. For complex shell pipelines, put them in a script and call it with shell = \"bash -lc\"", shell)
	}
	parts := strings.Fields(shell)

	// Confirm the container is running — surface a clear error rather than
	// the noisy `docker exec` "no such container" message.
	if !containerRunning(container) {
		return fmt.Errorf("docker:exec hook: container %q is not running — start it before grove new, or use type = \"docker:compose\"", container)
	}

	timeout := time.Duration(action.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	cctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := append([]string{"exec"}, dockerEnvArgs(vars.ShellEnv())...)
	args = append(args, container)
	args = append(args, parts...)
	args = append(args, command)
	cmd := exec.CommandContext(cctx, "docker", args...)
	w := ctx.Out()
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker exec %s: %w", container, err)
	}

	_, _ = fmt.Fprintf(w, "✓ Ran in container %s: %s\n", container, command)
	return nil
}

// containerRunning reports whether a docker container with the given name is
// in the running state. Returns false if docker is unavailable or the
// container doesn't exist. Bounded to 3s so a hung daemon doesn't block grove.
func containerRunning(name string) bool {
	cctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "docker", "inspect", "-f", "{{.State.Running}}", name)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}
