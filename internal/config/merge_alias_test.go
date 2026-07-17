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
