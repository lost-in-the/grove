package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
)

// Plugin implements the docker plugin for grove
type Plugin struct {
	cfg     *config.Config
	enabled bool
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
	if cfg != nil && !cfg.Plugins.Docker.Enabled {
		p.enabled = false
		return nil
	}

	// Check if docker-compose is available
	if _, err := exec.LookPath("docker-compose"); err != nil {
		// Try docker compose (newer syntax)
		if _, err := exec.LookPath("docker"); err != nil {
			p.enabled = false
			return fmt.Errorf("docker or docker-compose not found in PATH")
		}
	}

	return nil
}

// RegisterHooks registers docker plugin hooks
func (p *Plugin) RegisterHooks(registry *hooks.Registry) error {
	// Auto-start containers after switching to a worktree
	registry.Register(hooks.EventPostSwitch, func(ctx *hooks.Context) error {
		return p.onPostSwitch(ctx)
	})

	// Stop containers before switching away (optional)
	registry.Register(hooks.EventPreSwitch, func(ctx *hooks.Context) error {
		return p.onPreSwitch(ctx)
	})

	return nil
}

// Enabled returns whether the plugin is enabled
func (p *Plugin) Enabled() bool {
	return p.enabled
}

// onPostSwitch handles the post-switch hook
func (p *Plugin) onPostSwitch(ctx *hooks.Context) error {
	// Only auto-start if configured
	autoStart := p.getAutoStart()
	if !autoStart {
		return nil
	}

	worktreePath := p.getWorktreePath(ctx.Worktree)
	if !p.hasDockerCompose(worktreePath) {
		return nil // No docker-compose file, nothing to do
	}

	// Start containers
	return p.up(worktreePath, false)
}

// onPreSwitch handles the pre-switch hook
func (p *Plugin) onPreSwitch(ctx *hooks.Context) error {
	// Only auto-stop if configured
	autoStop := p.getAutoStop()
	if !autoStop {
		return nil
	}

	worktreePath := p.getWorktreePath(ctx.PrevWorktree)
	if !p.hasDockerCompose(worktreePath) {
		return nil // No docker-compose file, nothing to do
	}

	// Stop containers
	return p.down(worktreePath)
}

// Up starts containers for a worktree
func (p *Plugin) Up(worktreePath string, detach bool) error {
	return p.up(worktreePath, detach)
}

// Down stops containers for a worktree
func (p *Plugin) Down(worktreePath string) error {
	return p.down(worktreePath)
}

// Logs tails logs for a service
func (p *Plugin) Logs(worktreePath string, service string, follow bool) error {
	if !p.hasDockerCompose(worktreePath) {
		return fmt.Errorf("no docker-compose file found in %s", worktreePath)
	}

	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if service != "" {
		args = append(args, service)
	}

	cmd := p.composeCommand(worktreePath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// Restart restarts a service
func (p *Plugin) Restart(worktreePath string, service string) error {
	if !p.hasDockerCompose(worktreePath) {
		return fmt.Errorf("no docker-compose file found in %s", worktreePath)
	}

	args := []string{"restart"}
	if service != "" {
		args = append(args, service)
	}

	cmd := p.composeCommand(worktreePath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// up starts containers
func (p *Plugin) up(worktreePath string, detach bool) error {
	args := []string{"up"}
	if detach {
		args = append(args, "-d")
	}

	cmd := p.composeCommand(worktreePath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// down stops containers
func (p *Plugin) down(worktreePath string) error {
	cmd := p.composeCommand(worktreePath, "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// composeCommand creates a docker-compose command
func (p *Plugin) composeCommand(worktreePath string, args ...string) *exec.Cmd {
	// Try docker compose first (newer syntax)
	if _, err := exec.LookPath("docker"); err == nil {
		cmdArgs := append([]string{"compose"}, args...)
		cmd := exec.Command("docker", cmdArgs...)
		cmd.Dir = worktreePath
		return cmd
	}

	// Fall back to docker-compose
	cmd := exec.Command("docker-compose", args...)
	cmd.Dir = worktreePath
	return cmd
}

// hasDockerCompose checks if a worktree has a docker-compose file
func (p *Plugin) hasDockerCompose(worktreePath string) bool {
	composeFiles := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, file := range composeFiles {
		path := filepath.Join(worktreePath, file)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// getWorktreePath returns the full path to a worktree
func (p *Plugin) getWorktreePath(name string) string {
	if name == "" {
		return ""
	}

	// If it's already an absolute path, return it
	if filepath.IsAbs(name) {
		return name
	}

	// Get from config or use current directory
	if p.cfg != nil && p.cfg.ProjectsDir != "" {
		projectsDir := p.cfg.ProjectsDir
		// Expand ~ to home directory
		if strings.HasPrefix(projectsDir, "~/") {
			home, err := os.UserHomeDir()
			if err == nil {
				projectsDir = filepath.Join(home, projectsDir[2:])
			}
		}
		return filepath.Join(projectsDir, name)
	}

	// Fall back to current directory
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, name)
}

// getAutoStart returns whether to auto-start containers on switch
func (p *Plugin) getAutoStart() bool {
	if p.cfg != nil {
		return p.cfg.Plugins.Docker.AutoStart
	}
	// Default to true
	return true
}

// getAutoStop returns whether to auto-stop containers on switch
func (p *Plugin) getAutoStop() bool {
	if p.cfg != nil {
		return p.cfg.Plugins.Docker.AutoStop
	}
	// Default to false
	return false
}
