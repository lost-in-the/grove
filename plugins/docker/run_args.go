package docker

import (
	"fmt"

	"github.com/lost-in-the/grove/internal/config"
)

// buildRunArgs constructs the `docker compose run ...` argument list for grove test,
// applying --no-deps (unless include_deps is set) and bind_mount based on TestConfig.
func buildRunArgs(cfg *config.Config, worktreePath, service, command string) []string {
	args := []string{"run", "--rm"}
	if !cfg.Test.IncludeDepsValue() {
		args = append(args, "--no-deps")
	}
	if cfg.Test.BindMount != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s", worktreePath, cfg.Test.BindMount))
	}
	args = append(args, service, "bash", "-cil", command)
	return args
}
