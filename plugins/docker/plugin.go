package docker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	cfg           *config.Config
	enabled       bool
	strategy      modeStrategy
	forceIsolated bool
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
		if p.forceIsolated || isAgentMode(cfg) {
			p.strategy = newAgentExternalStrategy(cfg)
		} else {
			p.strategy = newExternalStrategy(cfg)
		}
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

	registry.Register(hooks.EventPreRemove, func(ctx *hooks.Context) error {
		return p.onPreRemove(ctx)
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

// onPreRemove handles the pre-remove hook — tears down agent stacks if running.
func (p *Plugin) onPreRemove(ctx *hooks.Context) error {
	if p.strategy == nil {
		return nil
	}
	agent, ok := p.strategy.(*agentExternalStrategy)
	if !ok {
		return nil
	}
	if ctx.WorktreePath == "" {
		return nil
	}
	wtName := filepath.Base(ctx.WorktreePath)
	slot, _ := agent.slots.FindSlot(wtName)
	if slot == 0 {
		return nil
	}
	fmt.Fprintf(os.Stderr, "Stopping agent stack for '%s'...\n", ctx.Worktree)
	return p.strategy.Down(ctx.WorktreePath)
}

// SetIsolated forces the plugin to use the agent external strategy,
// regardless of the GROVE_AGENT_MODE env var. Used by --isolated flags.
func (p *Plugin) SetIsolated(isolated bool) {
	p.forceIsolated = isolated
}

// IsIsolated returns true if the plugin is using the agent external strategy.
func (p *Plugin) IsIsolated() bool {
	_, ok := p.strategy.(*agentExternalStrategy)
	return ok
}

// HasActiveAgentSlot checks whether the worktree at the given path has an active
// agent stack slot allocated. Used for auto-detecting isolated mode in commands
// like down, logs, and restart.
func HasActiveAgentSlot(cfg *config.Config, worktreePath string) bool {
	return FindWorktreeSlot(cfg, worktreePath) > 0
}

// FindWorktreeSlot returns the agent slot number for a worktree, or 0 if none.
func FindWorktreeSlot(cfg *config.Config, worktreePath string) int {
	sm := buildSlotManager(cfg)
	if sm == nil {
		return 0
	}
	wtName := filepath.Base(worktreePath)
	slot, _ := sm.FindSlot(wtName)
	return slot
}

// ListActiveSlots returns all currently allocated agent slots.
func ListActiveSlots(cfg *config.Config) ([]SlotInfo, error) {
	sm := buildSlotManager(cfg)
	if sm == nil {
		return nil, fmt.Errorf("agent stack not configured")
	}
	return sm.ListActive()
}

// AgentURL derives the agent URL for a given slot from the config's URLPattern.
// Returns empty string if no pattern is configured.
func AgentURL(cfg *config.Config, slot int) string {
	if cfg == nil || cfg.Plugins.Docker.External == nil || cfg.Plugins.Docker.External.Agent == nil {
		return ""
	}
	pattern := cfg.Plugins.Docker.External.Agent.URLPattern
	if pattern == "" {
		return ""
	}
	return strings.ReplaceAll(pattern, "{slot}", fmt.Sprintf("%d", slot))
}

// AgentComposeProjectName returns the compose project name for a given slot.
func AgentComposeProjectName(cfg *config.Config, slot int) string {
	project := ""
	if cfg != nil {
		project = cfg.ProjectName
	}
	if project == "" {
		project = "grove"
	}
	if slot > 0 {
		return fmt.Sprintf("%s-agent-%d", project, slot)
	}
	return fmt.Sprintf("%s-agent-ephemeral", project)
}

// buildSlotManager creates a SlotManager from config, or nil if agent mode
// is not configured.
func buildSlotManager(cfg *config.Config) *SlotManager {
	if cfg == nil || !cfg.IsExternalDockerMode() {
		return nil
	}
	ext := cfg.Plugins.Docker.External
	if ext == nil || ext.Agent == nil || ext.Agent.Enabled == nil || !*ext.Agent.Enabled {
		return nil
	}
	if ext.Agent.TemplatePath == "" {
		return nil
	}
	maxSlots := ext.Agent.MaxSlots
	if maxSlots <= 0 {
		maxSlots = 5
	}
	slotsFile := filepath.Join(resolveComposePath(ext.Path), filepath.Dir(ext.Agent.TemplatePath), ".slots.json")
	return NewSlotManager(slotsFile, maxSlots)
}

// isAgentMode returns true when the agent mode env var is set and the config
// has an enabled agent section.
func isAgentMode(cfg *config.Config) bool {
	if cfg.AgentMode || os.Getenv("GROVE_AGENT_MODE") == "1" {
		ext := cfg.Plugins.Docker.External
		if ext == nil || ext.Agent == nil {
			return false
		}
		return ext.Agent.Enabled != nil && *ext.Agent.Enabled
	}
	return false
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
