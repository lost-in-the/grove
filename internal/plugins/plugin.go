package plugins

import (
	"sync"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
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

// StatusLevel represents the severity/type of a plugin status.
type StatusLevel int

const (
	StatusNone    StatusLevel = iota // not applicable
	StatusInfo                       // configured but inactive
	StatusActive                     // running / healthy
	StatusWarning                    // degraded / needs attention
	StatusError                      // down / broken
)

// StatusEntry represents a single status contribution from a plugin.
type StatusEntry struct {
	// ProviderName identifies which plugin produced this (e.g. "docker").
	ProviderName string

	// Level indicates the status severity for display styling.
	Level StatusLevel

	// Short is a compact label for CLI table columns (e.g. "up", "down", "slot 3").
	Short string

	// Detail is a longer description for the TUI detail pane.
	Detail string
}

// StatusProvider is an optional interface that plugins can implement
// to contribute status information to grove ls and the TUI dashboard.
type StatusProvider interface {
	// WorktreeStatuses returns status entries keyed by absolute worktree path.
	// Accepts all worktree paths at once so implementations can batch queries.
	// Only return entries for worktrees the plugin has information about.
	WorktreeStatuses(worktreePaths []string) map[string]StatusEntry
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

// CollectStatuses gathers status entries from all StatusProvider plugins.
// Returns a map of worktree path -> slice of StatusEntry.
func (m *Manager) CollectStatuses(worktreePaths []string) map[string][]StatusEntry {
	var providers []StatusProvider
	for _, p := range m.plugins {
		if !p.Enabled() {
			continue
		}
		if sp, ok := p.(StatusProvider); ok {
			providers = append(providers, sp)
		}
	}

	if len(providers) == 0 {
		return nil
	}

	result := make(map[string][]StatusEntry)
	var mu sync.Mutex

	var wg sync.WaitGroup
	for _, sp := range providers {
		wg.Add(1)
		go func(sp StatusProvider) {
			defer wg.Done()
			statuses := sp.WorktreeStatuses(worktreePaths)
			mu.Lock()
			for path, entry := range statuses {
				result[path] = append(result[path], entry)
			}
			mu.Unlock()
		}(sp)
	}
	wg.Wait()

	return result
}
