package docker

import (
	"fmt"

	"github.com/lost-in-the/grove/internal/config"
)

// buildRunArgs constructs the `docker compose run ...` argument list for grove test,
// applying --no-deps (unless include_deps is set) and bind_mount based on TestConfig.
//
// hookEnv carries GROVE_HOOK_* bindings for docker:compose hooks; each becomes a
// `-e KEY=VALUE` flag so the value reaches the container as environment instead
// of being spliced into the `bash -cil` command string (which would let a
// hostile branch name injected via {{.branch}} execute — see dockerEnvArgs).
// Pass nil from the trusted `grove test` path.
func buildRunArgs(cfg *config.Config, worktreePath, service, command string, hookEnv []string) []string {
	args := []string{"run", "--rm"}
	args = append(args, dockerEnvArgs(hookEnv)...)
	if !cfg.Test.IncludeDepsValue() {
		args = append(args, "--no-deps")
	}
	if cfg.Test.BindMount != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s", worktreePath, cfg.Test.BindMount))
	}
	args = append(args, service, "bash", "-cil", command)
	return args
}

// dockerEnvArgs turns ["KEY=VALUE", ...] into ["-e", "KEY=VALUE", ...] for
// `docker compose run/exec` and `docker exec`. Each value is a single argv
// element handed to the docker binary (no shell parses it), and inside the
// container it is referenced as ${KEY}, expanded after the command is parsed —
// so a value containing $(...), backticks, or ; can never inject a command.
func dockerEnvArgs(env []string) []string {
	if len(env) == 0 {
		return nil
	}
	args := make([]string, 0, len(env)*2)
	for _, e := range env {
		args = append(args, "-e", e)
	}
	return args
}
