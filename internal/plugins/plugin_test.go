package plugins

import (
	"errors"
	"testing"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
)

// mockPlugin is a test implementation of the Plugin interface
type mockPlugin struct {
	name    string
	enabled bool
	initErr error
	hookErr error
}

func (m *mockPlugin) Name() string {
	return m.name
}

func (m *mockPlugin) Init(cfg *config.Config) error {
	return m.initErr
}

func (m *mockPlugin) RegisterHooks(registry *hooks.Registry) error {
	if m.hookErr != nil {
		return m.hookErr
	}
	// Register a test hook
	registry.Register(hooks.EventPostSwitch, func(ctx *hooks.Context) error {
		return nil
	})
	return nil
}

func (m *mockPlugin) Enabled() bool {
	return m.enabled
}

func TestNewManager(t *testing.T) {
	cfg := &config.Config{}
	manager := NewManager(cfg)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.cfg != cfg {
		t.Error("Manager config not set correctly")
	}

	if len(manager.plugins) != 0 {
		t.Errorf("Expected 0 plugins, got %d", len(manager.plugins))
	}
}

func TestManager_Register(t *testing.T) {
	tests := []struct {
		name    string
		plugin  Plugin
		wantErr bool
	}{
		{
			name: "successful registration",
			plugin: &mockPlugin{
				name:    "test-plugin",
				enabled: true,
			},
			wantErr: false,
		},
		{
			name: "init error",
			plugin: &mockPlugin{
				name:    "error-plugin",
				enabled: true,
				initErr: errors.New("init failed"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			manager := NewManager(cfg)

			err := manager.Register(tt.plugin)
			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if len(manager.plugins) != 1 {
					t.Errorf("Expected 1 plugin, got %d", len(manager.plugins))
				}
			}
		})
	}
}

func TestManager_RegisterHooks(t *testing.T) {
	tests := []struct {
		name    string
		plugins []Plugin
		wantErr bool
	}{
		{
			name: "register enabled plugin hooks",
			plugins: []Plugin{
				&mockPlugin{
					name:    "enabled-plugin",
					enabled: true,
				},
			},
			wantErr: false,
		},
		{
			name: "skip disabled plugin",
			plugins: []Plugin{
				&mockPlugin{
					name:    "disabled-plugin",
					enabled: false,
				},
			},
			wantErr: false,
		},
		{
			name: "hook registration error",
			plugins: []Plugin{
				&mockPlugin{
					name:    "error-plugin",
					enabled: true,
					hookErr: errors.New("hook registration failed"),
				},
			},
			wantErr: true,
		},
		{
			name: "multiple plugins",
			plugins: []Plugin{
				&mockPlugin{name: "plugin1", enabled: true},
				&mockPlugin{name: "plugin2", enabled: true},
				&mockPlugin{name: "plugin3", enabled: false},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			manager := NewManager(cfg)
			registry := hooks.NewRegistry()

			for _, plugin := range tt.plugins {
				_ = manager.Register(plugin)
			}

			err := manager.RegisterHooks(registry)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterHooks() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManager_GetPlugin(t *testing.T) {
	cfg := &config.Config{}
	manager := NewManager(cfg)

	plugin1 := &mockPlugin{name: "plugin1", enabled: true}
	plugin2 := &mockPlugin{name: "plugin2", enabled: true}

	_ = manager.Register(plugin1)
	_ = manager.Register(plugin2)

	tests := []struct {
		name       string
		pluginName string
		wantNil    bool
	}{
		{
			name:       "existing plugin",
			pluginName: "plugin1",
			wantNil:    false,
		},
		{
			name:       "non-existent plugin",
			pluginName: "nonexistent",
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.GetPlugin(tt.pluginName)
			if (result == nil) != tt.wantNil {
				t.Errorf("GetPlugin(%s) nil = %v, wantNil %v", tt.pluginName, result == nil, tt.wantNil)
			}
			if !tt.wantNil && result.Name() != tt.pluginName {
				t.Errorf("GetPlugin(%s) returned wrong plugin: %s", tt.pluginName, result.Name())
			}
		})
	}
}

func TestManager_ListPlugins(t *testing.T) {
	cfg := &config.Config{}
	manager := NewManager(cfg)

	// Start with no plugins
	if len(manager.ListPlugins()) != 0 {
		t.Errorf("Expected 0 plugins initially, got %d", len(manager.ListPlugins()))
	}

	// Add plugins
	plugin1 := &mockPlugin{name: "plugin1", enabled: true}
	plugin2 := &mockPlugin{name: "plugin2", enabled: true}

	_ = manager.Register(plugin1)
	_ = manager.Register(plugin2)

	plugins := manager.ListPlugins()
	if len(plugins) != 2 {
		t.Errorf("Expected 2 plugins, got %d", len(plugins))
	}
}
