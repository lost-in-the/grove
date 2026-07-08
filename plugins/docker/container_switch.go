package docker

import (
	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
)

// ContainerSwitchAction represents the resolved action for container lifecycle during switch.
type ContainerSwitchAction int

const (
	ContainerSwitchAuto   ContainerSwitchAction = iota // Current behavior — auto start/stop
	ContainerSwitchPrompt                              // Ask user before start/stop
	ContainerSwitchOff                                 // Skip all container lifecycle
)

// ResolveContainerSwitch maps a config string to a ContainerSwitchAction.
// Non-interactive sessions fall back to Auto (never prompt).
func ResolveContainerSwitch(configValue string, isInteractive bool) ContainerSwitchAction {
	switch configValue {
	case "off":
		return ContainerSwitchOff
	case "prompt":
		if !isInteractive {
			return ContainerSwitchAuto
		}
		return ContainerSwitchPrompt
	default: // "auto" or ""
		return ContainerSwitchAuto
	}
}

// resolveFromConfig is a convenience that reads the config and checks interactivity.
func resolveFromConfig(containerSwitch string) ContainerSwitchAction {
	return ResolveContainerSwitch(containerSwitch, cli.IsInteractive())
}

// Prompt strings shared by the local and external switch hooks.
const (
	promptStopContainers  = "Stop containers in previous worktree?"
	promptStartContainers = "Start containers for this worktree?"
)

// containerSwitchSetting reads the container_switch config value from the hook context.
func containerSwitchSetting(ctx *hooks.Context) string {
	if ctx.Config != nil {
		return ctx.Config.Switch.ContainerSwitch
	}
	return ""
}

// confirmSwitchAction gates a container lifecycle action during switch hooks,
// combining the auto_start/auto_stop config gate, the container_switch
// resolution, and the interactive prompt. Shared by the local and external
// strategies; per-mode defaults live in the strategies' getters. Returns true
// when the caller should proceed with the container action. A declined or
// failed prompt counts as "don't proceed" — the switch itself must not fail
// because of a container prompt.
func confirmSwitchAction(autoEnabled bool, ctx *hooks.Context, prompt string, promptDefault bool) bool {
	if !autoEnabled {
		return false
	}

	action := resolveFromConfig(containerSwitchSetting(ctx))
	if action == ContainerSwitchOff {
		return false
	}

	if action == ContainerSwitchPrompt {
		yes, err := cli.Confirm(prompt, promptDefault)
		if err != nil || !yes {
			return false
		}
	}

	return true
}

// dockerAutoStart returns the configured auto_start value or the strategy's default.
func dockerAutoStart(cfg *config.Config, defaultValue bool) bool {
	if cfg != nil && cfg.Plugins.Docker.AutoStart != nil {
		return *cfg.Plugins.Docker.AutoStart
	}
	return defaultValue
}

// dockerAutoStop returns the configured auto_stop value or the strategy's default.
func dockerAutoStop(cfg *config.Config, defaultValue bool) bool {
	if cfg != nil && cfg.Plugins.Docker.AutoStop != nil {
		return *cfg.Plugins.Docker.AutoStop
	}
	return defaultValue
}
