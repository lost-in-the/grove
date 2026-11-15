package docker

import (
	"path/filepath"

	"github.com/lost-in-the/grove/internal/config"
)

// ConfiguredWorktreePath returns the absolute path of the worktree the shared
// external stack is configured for, as recorded in the env file's env_var.
// It reads that file only (no Docker calls). The value says what grove
// configured, not what the running containers have mounted.
// ok is false outside external mode or when the variable is unset.
func ConfiguredWorktreePath(cfg *config.Config) (path string, ok bool) {
	if cfg == nil || !cfg.IsExternalDockerMode() {
		return "", false
	}
	ext := cfg.Plugins.Docker.External
	if ext == nil || ext.EnvVar == "" {
		return "", false
	}

	composePath := resolveComposePath(ext.Path)
	value := readEnvVar(composePath, ext.EnvFileName(), ext.EnvVar)
	if value == "" {
		return "", false
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(composePath, value)
	}
	return filepath.Clean(value), true
}
