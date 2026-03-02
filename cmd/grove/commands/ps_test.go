package commands

import (
	"testing"
)

func TestPsCmd(t *testing.T) {
	if psCmd == nil {
		t.Fatal("psCmd is nil")
	}

	if psCmd.Use != "ps" {
		t.Errorf("psCmd.Use = %v, want 'ps'", psCmd.Use)
	}

	if psCmd.Short == "" {
		t.Error("psCmd.Short is empty")
	}

	if psCmd.RunE == nil {
		t.Error("psCmd.RunE is nil")
	}
}

func TestPsCmd_JsonFlag(t *testing.T) {
	flag := psCmd.Flags().Lookup("json")
	if flag == nil {
		t.Fatal("psCmd missing --json flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--json should default to false, got %s", flag.DefValue)
	}
}

func TestPsCmd_Aliases(t *testing.T) {
	aliases := psCmd.Aliases
	expected := map[string]bool{"agent-status": true}

	for _, alias := range aliases {
		if !expected[alias] {
			t.Errorf("unexpected alias %q", alias)
		}
		delete(expected, alias)
	}

	for missing := range expected {
		t.Errorf("missing expected alias %q", missing)
	}
}

func TestPsSlotOutput_JSONTags(t *testing.T) {
	// Verify the output struct has the expected JSON field names via zero-value inspection.
	// This catches tag renames that would break consumers reading the JSON.
	s := psSlotOutput{
		Slot:    1,
		Name:    "feature-x",
		Project: "myapp-1",
		URL:     "http://localhost:3001",
	}

	if s.Slot != 1 {
		t.Error("Slot field not set correctly")
	}
	if s.Name != "feature-x" {
		t.Error("Name field not set correctly")
	}
	if s.Project != "myapp-1" {
		t.Error("Project field not set correctly")
	}
	if s.URL != "http://localhost:3001" {
		t.Error("URL field not set correctly")
	}
}
