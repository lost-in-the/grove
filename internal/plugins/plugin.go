package plugins

import (
	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
)

// Plugin represents a grove plugin that can extend functionality
type Plugin interface {
	// Name returns the plugin name
	Name() string

	// Init initializes the plugin with configuration
	Init(cfg *config.Config) error

	// RegisterHooks registers plugin hooks with the hook registry
	RegisterHooks(registry *hooks.Registry) error

	// Enabled returns whether the plugin is enabled
	Enabled() bool
}

// Manager manages plugin lifecycle
type Manager struct {
	plugins []Plugin
	cfg     *config.Config
}

// NewManager creates a new plugin manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		plugins: make([]Plugin, 0),
		cfg:     cfg,
	}
}

// Register registers a plugin with the manager
func (m *Manager) Register(plugin Plugin) error {
	if err := plugin.Init(m.cfg); err != nil {
		return err
	}
	m.plugins = append(m.plugins, plugin)
	return nil
}

// RegisterHooks registers all plugin hooks with the provided registry
func (m *Manager) RegisterHooks(registry *hooks.Registry) error {
	for _, plugin := range m.plugins {
		if !plugin.Enabled() {
			continue
		}
		if err := plugin.RegisterHooks(registry); err != nil {
			return err
		}
	}
	return nil
}

// GetPlugin returns a plugin by name
func (m *Manager) GetPlugin(name string) Plugin {
	for _, plugin := range m.plugins {
		if plugin.Name() == name {
			return plugin
		}
	}
	return nil
}

// ListPlugins returns all registered plugins
func (m *Manager) ListPlugins() []Plugin {
	return m.plugins
}
