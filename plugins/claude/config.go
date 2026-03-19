package claude

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/lost-in-the/grove/internal/config"
)

// forwardConfigFiles sets up Claude Code configuration inside the devcontainer.
// It generates a settings.json with permission overrides when configured.
func forwardConfigFiles(worktreePath string, claudeCfg *config.ClaudePluginConfig) error {
	devcontainerDir := filepath.Join(worktreePath, ".devcontainer")

	// Generate permission-scoped settings if configured
	if claudeCfg.Permissions != nil {
		if err := generatePermissionSettings(devcontainerDir, claudeCfg.Permissions); err != nil {
			return err
		}
	}

	return nil
}

// claudeSettings represents the subset of Claude Code settings.json we generate.
type claudeSettings struct {
	AllowedTools []string `json:"allowedTools,omitempty"`
	AllowedMCPs  []string `json:"allowedMCPServers,omitempty"`
	MaxTurns     int      `json:"maxTurns,omitempty"`
}

// generatePermissionSettings writes a Claude Code settings.json into the
// devcontainer directory with the configured permission restrictions.
func generatePermissionSettings(devcontainerDir string, perms *config.ClaudePermissionsConfig) error {
	settings := claudeSettings{
		AllowedTools: perms.AllowedTools,
		AllowedMCPs:  perms.AllowedMCPs,
		MaxTurns:     perms.MaxTurns,
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	settingsPath := filepath.Join(devcontainerDir, "claude-settings.json")
	return os.WriteFile(settingsPath, append(data, '\n'), 0o644)
}
