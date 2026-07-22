package config

import "testing"

// TestMergeConfigs_DoesNotMutateInputs guards mergeExternalComposeConfig
// against pointer aliasing: mergeConfigs shallow-copies Config, so
// result.Plugins.Docker.External is the SAME pointer as the base's — the old
// field-merge wrote through it, corrupting the caller's base config, and its
// nil-base branch adopted the override's slices/Agent by reference so later
// edits leaked between layers. Merging must never mutate its inputs.
func TestMergeConfigs_DoesNotMutateInputs(t *testing.T) {
	t.Run("base external is not written through", func(t *testing.T) {
		base := LoadDefaults()
		base.Plugins.Docker.External = &ExternalComposeConfig{
			Path:     "/orig",
			Services: []string{"web", "db"},
		}
		override := LoadDefaults()
		override.Plugins.Docker.External = &ExternalComposeConfig{Path: "/override"}

		merged := mergeConfigs(base, override)

		if base.Plugins.Docker.External.Path != "/orig" {
			t.Errorf("merge mutated the base config: Path = %q, want %q",
				base.Plugins.Docker.External.Path, "/orig")
		}
		if merged.Plugins.Docker.External == base.Plugins.Docker.External {
			t.Error("merged External aliases the base's — must be a fresh struct")
		}
		if merged.Plugins.Docker.External.Path != "/override" {
			t.Errorf("merged Path = %q, want %q", merged.Plugins.Docker.External.Path, "/override")
		}
		if got := merged.Plugins.Docker.External.Services; len(got) != 2 || got[0] != "web" {
			t.Errorf("merged Services = %v, want base's [web db]", got)
		}
	})

	// The agent section must field-merge like its parent: wholesale pointer
	// replacement meant a config.local.toml setting only max_slots wiped the
	// required template_path, failing validation — the exact B29 partial-
	// override bug one level down.
	t.Run("agent overrides field-merge, not replace", func(t *testing.T) {
		enabled := true
		base := LoadDefaults()
		base.Plugins.Docker.External = &ExternalComposeConfig{
			Path: "/x",
			Agent: &AgentStackConfig{
				Enabled:      &enabled,
				TemplatePath: "agent-stacks/template.yml",
				Services:     []string{"app"},
				Network:      "grove-net",
			},
		}
		override := LoadDefaults()
		override.Plugins.Docker.External = &ExternalComposeConfig{
			Agent: &AgentStackConfig{MaxSlots: 10},
		}

		merged := mergeConfigs(base, override)
		agent := merged.Plugins.Docker.External.Agent

		if agent.MaxSlots != 10 {
			t.Errorf("MaxSlots = %d, want 10 (override)", agent.MaxSlots)
		}
		if agent.TemplatePath != "agent-stacks/template.yml" {
			t.Errorf("TemplatePath = %q, want base value (wiped by wholesale replace)", agent.TemplatePath)
		}
		if len(agent.Services) != 1 || agent.Services[0] != "app" {
			t.Errorf("Services = %v, want base [app]", agent.Services)
		}
		if agent.Network != "grove-net" {
			t.Errorf("Network = %q, want base value", agent.Network)
		}
		if agent.Enabled == nil || !*agent.Enabled {
			t.Errorf("Enabled = %v, want base true", agent.Enabled)
		}
	})

	t.Run("override slices and agent are not shared", func(t *testing.T) {
		enabled := true
		base := LoadDefaults() // External nil — exercises the adopt-copy branch
		override := LoadDefaults()
		override.Plugins.Docker.External = &ExternalComposeConfig{
			Path:     "/x",
			Services: []string{"web"},
			Agent:    &AgentStackConfig{Enabled: &enabled, Services: []string{"agent"}},
		}

		merged := mergeConfigs(base, override)
		ext := merged.Plugins.Docker.External

		ext.Services[0] = "tampered"
		if override.Plugins.Docker.External.Services[0] == "tampered" {
			t.Error("merged Services share backing with the override's")
		}
		if ext.Agent == override.Plugins.Docker.External.Agent {
			t.Error("merged Agent aliases the override's — must be a fresh struct")
		}
		ext.Agent.Services[0] = "tampered"
		if override.Plugins.Docker.External.Agent.Services[0] == "tampered" {
			t.Error("merged Agent.Services share backing with the override's")
		}
	})
}
