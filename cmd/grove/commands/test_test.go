package commands

import (
	"testing"
)

// resolveTestOptions composes effective test options from CLI flags layered over config.
// Unit-test the resolution logic without invoking docker.
func TestResolveTestOptions_FlagOverridesConfig(t *testing.T) {
	tests := []struct {
		name            string
		cfgIncludeDeps  bool
		cfgBindMount    string
		flagWithDeps    bool
		flagBind        string
		wantIncludeDeps bool
		wantBindMount   string
	}{
		{"defaults", false, "", false, "", false, ""},
		{"config opts in to deps", true, "", false, "", true, ""},
		{"flag overrides config off", false, "", true, "", true, ""},
		{"flag bind overrides empty config", false, "", false, "/app", false, "/app"},
		{"flag bind overrides config bind", false, "/old", false, "/new", false, "/new"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := resolveTestOptions(tc.cfgIncludeDeps, tc.cfgBindMount, tc.flagWithDeps, tc.flagBind)
			if opts.IncludeDeps != tc.wantIncludeDeps {
				t.Errorf("IncludeDeps: got %v want %v", opts.IncludeDeps, tc.wantIncludeDeps)
			}
			if opts.BindMount != tc.wantBindMount {
				t.Errorf("BindMount: got %q want %q", opts.BindMount, tc.wantBindMount)
			}
		})
	}
}
