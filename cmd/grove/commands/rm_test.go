package commands

import (
	"testing"
)

func TestRmCmd(t *testing.T) {
	if rmCmd == nil {
		t.Fatal("rmCmd is nil")
	}

	if rmCmd.Use != "rm <name>" {
		t.Errorf("rmCmd.Use = %v, want 'rm <name>'", rmCmd.Use)
	}

	if rmCmd.RunE == nil {
		t.Error("rmCmd.RunE is nil")
	}
}

func TestRmFlags(t *testing.T) {
	flags := rmCmd.Flags()

	tests := []string{"force", "unprotect", "dry-run", "keep-branch", "delete-branch"}
	for _, name := range tests {
		if flags.Lookup(name) == nil {
			t.Errorf("expected --%s flag to exist", name)
		}
	}
}

func TestRmAliases(t *testing.T) {
	aliases := rmCmd.Aliases
	expected := map[string]bool{"remove": true, "delete": true}

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
