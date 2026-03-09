package docker

import "github.com/LeahArmstrong/grove-cli/internal/cli"

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
