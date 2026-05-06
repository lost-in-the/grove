package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveProjectPaths normalizes any path-valued fields in cfg to absolute
// paths. Relative paths resolve against projectRoot (the directory containing
// .grove/, i.e., where a developer would `cd` into the project). ~/-prefixed
// paths expand against $HOME. Absolute and empty paths are unchanged.
//
// This is called once per config file at load time so downstream consumers
// always see absolute paths regardless of where grove is invoked from.
func resolveProjectPaths(cfg *Config, projectRoot string) error {
	if cfg == nil || cfg.Plugins.Docker.External == nil {
		return nil
	}
	ext := cfg.Plugins.Docker.External

	resolved, err := expandConfigPath(ext.Path, projectRoot)
	if err != nil {
		return fmt.Errorf("plugins.docker.external.path: %w", err)
	}
	ext.Path = resolved

	// Expand ~/ in template_path and each template_overlays entry; non-absolute,
	// non-tilde values stay as written (the docker plugin joins them against the
	// compose directory at exec time, which is the documented contract).
	if ext.Agent != nil {
		if ext.Agent.TemplatePath != "" {
			expanded, err := expandTildePath(ext.Agent.TemplatePath)
			if err != nil {
				return fmt.Errorf("plugins.docker.external.agent.template_path: %w", err)
			}
			ext.Agent.TemplatePath = expanded
		}
		for i, overlay := range ext.Agent.TemplateOverlays {
			if overlay == "" {
				continue
			}
			expanded, err := expandTildePath(overlay)
			if err != nil {
				return fmt.Errorf("plugins.docker.external.agent.template_overlays[%d]: %w", i, err)
			}
			ext.Agent.TemplateOverlays[i] = expanded
		}
	}
	return nil
}

// expandTildePath expands ~ and ~/ prefixes against $HOME. Other paths
// (absolute or relative) pass through unchanged.
func expandTildePath(p string) (string, error) {
	if p == "" {
		return p, nil
	}
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand ~: %w", err)
		}
		return home, nil
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand ~: %w", err)
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

// expandConfigPath converts p into a cleaned absolute path. Empty strings pass
// through unchanged (validation will reject them where required). ~/-prefixed
// paths expand against $HOME. Absolute paths are cleaned. Relative paths are
// joined to baseDir; if baseDir is empty, they are cleaned as-is.
func expandConfigPath(p, baseDir string) (string, error) {
	if p == "" {
		return p, nil
	}
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand ~: %w", err)
		}
		return home, nil
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand ~: %w", err)
		}
		return filepath.Join(home, p[2:]), nil
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	if baseDir == "" {
		return filepath.Clean(p), nil
	}
	return filepath.Clean(filepath.Join(baseDir, p)), nil
}

// projectRootFor returns the directory that should anchor relative paths in a
// config file at configPath. By convention .grove/config.toml lives one level
// below the project root, so the root is the parent of the config file's
// directory.
func projectRootFor(configPath string) string {
	if configPath == "" {
		return ""
	}
	return filepath.Dir(filepath.Dir(configPath))
}
