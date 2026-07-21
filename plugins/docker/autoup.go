package docker

import (
	"fmt"

	"github.com/lost-in-the/grove/internal/config"
)

// ShouldAutoUp reports whether the docker stack should be brought up right
// after a worktree is created. The knob is explicit-only: `auto_up = true` in
// [plugins.docker] opts in, anything else (including agent stacks) stays off.
// Both `grove new` and the dashboard create path consult this, so the two
// surfaces always agree.
func ShouldAutoUp(cfg *config.Config) bool {
	return cfg != nil && cfg.Plugins.Docker.AutoUp != nil && *cfg.Plugins.Docker.AutoUp
}

// AutoUp starts the docker stack for a freshly created worktree. It reports
// whether a stack was actually started: (false, nil) means the plugin is
// disabled or docker is unusable in a way Init treats as a soft-disable.
// Callers own presentation (CLI writer vs. TUI log lines) and the
// ShouldAutoUp / --no-docker gating.
func AutoUp(cfg *config.Config, worktreePath string) (bool, error) {
	plugin := New()
	if cfg != nil && cfg.AgentMode {
		plugin.SetIsolated(true)
	}
	if err := plugin.Init(cfg); err != nil {
		return false, fmt.Errorf("docker init failed: %w", err)
	}
	if !plugin.Enabled() {
		return false, nil
	}
	if err := plugin.Up(worktreePath, true); err != nil {
		return false, fmt.Errorf("docker up failed: %w", err)
	}
	return true, nil
}
