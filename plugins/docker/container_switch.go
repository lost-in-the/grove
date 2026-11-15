package docker

import (
	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
)

// ContainerSwitchAction represents the resolved action for container lifecycle during switch.
type ContainerSwitchAction int

const (
	ContainerSwitchAuto   ContainerSwitchAction = iota // The stack follows every switch
	ContainerSwitchPrompt                              // Ask before the stack follows
	ContainerSwitchOff                                 // A switch never touches the stack (default)
)

// ResolveContainerSwitch maps a config string to a ContainerSwitchAction.
// An unset value is Off, matching the default. Non-interactive sessions fall
// back from "prompt" to Auto (never prompt).
func ResolveContainerSwitch(configValue string, isInteractive bool) ContainerSwitchAction {
	switch configValue {
	case "auto":
		return ContainerSwitchAuto
	case "prompt":
		if !isInteractive {
			return ContainerSwitchAuto
		}
		return ContainerSwitchPrompt
	default: // "off" or unset
		return ContainerSwitchOff
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

// switchHooksOff reports whether container_switch resolves to "off" for this
// hook context. The plugin checks it before any strategy runs, so no strategy
// has to.
func switchHooksOff(ctx *hooks.Context) bool {
	return resolveFromConfig(containerSwitchSetting(ctx)) == ContainerSwitchOff
}

// confirmSwitchAction gates a container lifecycle action during switch hooks,
// combining the auto_start/auto_stop config gate and the interactive prompt
// ("off" never reaches a strategy, see switchHooksOff). Shared by the local and
// external strategies; per-mode defaults live in the strategies' getters.
// Returns true when the caller should proceed with the container action. A
// declined or failed prompt counts as "don't proceed" — the switch itself must
// not fail because of a container prompt.
func confirmSwitchAction(autoEnabled bool, ctx *hooks.Context, prompt string, promptDefault bool) bool {
	if !autoEnabled {
		return false
	}

	if resolveFromConfig(containerSwitchSetting(ctx)) == ContainerSwitchPrompt {
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
