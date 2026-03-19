package claude

import (
	"github.com/lost-in-the/grove/internal/plugins"
)

// WorktreeStatuses implements plugins.StatusProvider for the Claude plugin.
func (p *Plugin) WorktreeStatuses(worktreePaths []string) map[string]plugins.StatusEntry {
	if !p.enabled {
		return nil
	}

	result := make(map[string]plugins.StatusEntry)
	for _, wt := range worktreePaths {
		state := getSandboxState(wt)
		switch state {
		case "running":
			result[wt] = plugins.StatusEntry{
				ProviderName: "claude",
				Level:        plugins.StatusActive,
				Short:        "sandbox",
				Detail:       "Claude Code sandbox running",
			}
		case "stopped":
			result[wt] = plugins.StatusEntry{
				ProviderName: "claude",
				Level:        plugins.StatusInfo,
				Short:        "stopped",
				Detail:       "Claude Code sandbox stopped",
			}
		}
		// "not-created" — no entry
	}
	return result
}
