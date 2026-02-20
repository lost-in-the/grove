package docker

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
)

// ErrNoComposeFile is returned when no docker-compose file is found
var ErrNoComposeFile = errors.New("no docker-compose file found")

// modeStrategy defines the interface for local vs external docker compose behavior.
type modeStrategy interface {
	OnPreSwitch(ctx *hooks.Context) error
	OnPostSwitch(ctx *hooks.Context) error
	OnPostCreate(ctx *hooks.Context) error
	Up(worktreePath string, detach bool) error
	Down(worktreePath string) error
	Logs(worktreePath string, service string, follow bool) error
	Restart(worktreePath string, service string) error
	Run(worktreePath string, service string, command string) error
}

// Plugin implements the docker plugin for grove
type Plugin struct {
	cfg      *config.Config
	enabled  bool
	strategy modeStrategy
}

// New creates a new Docker plugin
func New() *Plugin {
	return &Plugin{
		enabled: true,
	}
}

// Name returns the plugin name
func (p *Plugin) Name() string {
	return "docker"
}

// Init initializes the plugin with configuration
func (p *Plugin) Init(cfg *config.Config) error {
	p.cfg = cfg

	// Check if plugin is disabled in config
	if cfg != nil && cfg.Plugins.Docker.Enabled != nil && !*cfg.Plugins.Docker.Enabled {
		p.enabled = false
		return nil
	}

	// Check if docker is available
	if _, err := exec.LookPath("docker-compose"); err != nil {
		if _, err := exec.LookPath("docker"); err != nil {
			p.enabled = false
			return fmt.Errorf("docker or docker-compose not found in PATH")
		}
	}

	// Select strategy based on mode
	if cfg != nil && cfg.IsExternalDockerMode() {
		p.strategy = newExternalStrategy(cfg)
	} else {
		p.strategy = newLocalStrategy(cfg)
	}

	return nil
}

// RegisterHooks registers docker plugin hooks
func (p *Plugin) RegisterHooks(registry *hooks.Registry) error {
	registry.Register(hooks.EventPostSwitch, func(ctx *hooks.Context) error {
		return p.onPostSwitch(ctx)
	})

	registry.Register(hooks.EventPreSwitch, func(ctx *hooks.Context) error {
		return p.onPreSwitch(ctx)
	})

	registry.Register(hooks.EventPostCreate, func(ctx *hooks.Context) error {
		return p.onPostCreate(ctx)
	})

	return nil
}

// Enabled returns whether the plugin is enabled
func (p *Plugin) Enabled() bool {
	return p.enabled
}

// onPostSwitch handles the post-switch hook
func (p *Plugin) onPostSwitch(ctx *hooks.Context) error {
	if p.strategy == nil {
		return nil
	}
	return p.strategy.OnPostSwitch(ctx)
}

// onPreSwitch handles the pre-switch hook
func (p *Plugin) onPreSwitch(ctx *hooks.Context) error {
	if p.strategy == nil {
		return nil
	}
	return p.strategy.OnPreSwitch(ctx)
}

// onPostCreate handles the post-create hook
func (p *Plugin) onPostCreate(ctx *hooks.Context) error {
	if p.strategy == nil {
		return nil
	}
	return p.strategy.OnPostCreate(ctx)
}

// Up starts containers for a worktree
func (p *Plugin) Up(worktreePath string, detach bool) error {
	if p.strategy == nil {
		return fmt.Errorf("docker plugin not initialized")
	}
	return p.strategy.Up(worktreePath, detach)
}

// Down stops containers for a worktree
func (p *Plugin) Down(worktreePath string) error {
	if p.strategy == nil {
		return fmt.Errorf("docker plugin not initialized")
	}
	return p.strategy.Down(worktreePath)
}

// Logs tails logs for a service
func (p *Plugin) Logs(worktreePath string, service string, follow bool) error {
	if p.strategy == nil {
		return fmt.Errorf("docker plugin not initialized")
	}
	return p.strategy.Logs(worktreePath, service, follow)
}

// Restart restarts a service
func (p *Plugin) Restart(worktreePath string, service string) error {
	if p.strategy == nil {
		return fmt.Errorf("docker plugin not initialized")
	}
	return p.strategy.Restart(worktreePath, service)
}

// Run executes a command in a fresh ephemeral container for a worktree
func (p *Plugin) Run(worktreePath string, service string, command string) error {
	if p.strategy == nil {
		return fmt.Errorf("docker plugin not initialized")
	}
	return p.strategy.Run(worktreePath, service, command)
}
