package claude

import (
	"fmt"
	"os/exec"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
)

// Plugin implements the Claude Code devcontainer plugin for grove.
type Plugin struct {
	cfg     *config.Config
	enabled bool
}

// New creates a new Claude Code plugin.
func New() *Plugin {
	return &Plugin{
		enabled: true,
	}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "claude"
}

// Init initializes the plugin with configuration.
func (p *Plugin) Init(cfg *config.Config) error {
	p.cfg = cfg

	if cfg != nil && cfg.Plugins.Claude.Enabled != nil && !*cfg.Plugins.Claude.Enabled {
		p.enabled = false
		return nil
	}

	// Check if devcontainer CLI is available when devcontainer mode is enabled
	if p.devcontainerEnabled() {
		if _, err := exec.LookPath("devcontainer"); err != nil {
			p.enabled = false
			return fmt.Errorf("devcontainer CLI not found in PATH")
		}
	}

	return nil
}

// RegisterHooks registers claude plugin hooks.
func (p *Plugin) RegisterHooks(registry *hooks.Registry) error {
	registry.Register(hooks.EventPostCreate, func(ctx *hooks.Context) error {
		return p.onPostCreate(ctx)
	})

	registry.Register(hooks.EventPreRemove, func(ctx *hooks.Context) error {
		return p.onPreRemove(ctx)
	})

	return nil
}

// Enabled returns whether the plugin is enabled.
func (p *Plugin) Enabled() bool {
	return p.enabled
}

// onPostCreate scaffolds the devcontainer and injects grove context.
func (p *Plugin) onPostCreate(ctx *hooks.Context) error {
	if ctx.WorktreePath == "" {
		return nil
	}

	claudeCfg := p.cfg.Plugins.Claude

	// Scaffold devcontainer if enabled
	if p.devcontainerEnabled() {
		devCfg := claudeCfg.Devcontainer
		if err := scaffoldDevcontainer(ctx.WorktreePath, devCfg); err != nil {
			return fmt.Errorf("failed to scaffold devcontainer: %w", err)
		}

		// Forward Claude config files into devcontainer
		if err := forwardConfigFiles(ctx.WorktreePath, &claudeCfg); err != nil {
			return fmt.Errorf("failed to forward config files: %w", err)
		}
	}

	// Inject grove context into CLAUDE.md
	if claudeCfg.InjectGroveContext == nil || *claudeCfg.InjectGroveContext {
		if err := injectGroveContext(ctx.WorktreePath); err != nil {
			return fmt.Errorf("failed to inject grove context: %w", err)
		}
	}

	return nil
}

// onPreRemove stops and removes the devcontainer for a worktree.
func (p *Plugin) onPreRemove(ctx *hooks.Context) error {
	if ctx.WorktreePath == "" {
		return nil
	}

	if !p.devcontainerEnabled() {
		return nil
	}

	return stopSandbox(ctx.WorktreePath)
}

// devcontainerEnabled returns true if devcontainer mode is enabled.
func (p *Plugin) devcontainerEnabled() bool {
	if p.cfg == nil {
		return false
	}
	dc := p.cfg.Plugins.Claude.Devcontainer
	if dc == nil {
		return true // default: enabled when plugin is enabled
	}
	if dc.Enabled == nil {
		return true
	}
	return *dc.Enabled
}

// Config returns the Claude plugin configuration.
func (p *Plugin) Config() config.ClaudePluginConfig {
	if p.cfg == nil {
		return config.ClaudePluginConfig{}
	}
	return p.cfg.Plugins.Claude
}
