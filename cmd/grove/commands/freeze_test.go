package commands

import (
	"testing"
)

// Note: These tests require actual git repository setup which is complex in unit tests.
// The freeze and resume commands are primarily tested through integration tests.
// The core logic (state management, hooks, docker integration) is tested in their respective packages.

func TestFreezeCmdExists(t *testing.T) {
	if freezeCmd == nil {
		t.Fatal("freezeCmd should not be nil")
	}
	if freezeCmd.Use != "freeze [name]" {
		t.Errorf("freezeCmd.Use = %q, want %q", freezeCmd.Use, "freeze [name]")
	}
}

func TestFreezeCmdFlags(t *testing.T) {
	flag := freezeCmd.Flags().Lookup("all")
	if flag == nil {
		t.Fatal("--all flag should exist")
	}
	if flag.DefValue != "false" {
		t.Errorf("--all flag default = %q, want %q", flag.DefValue, "false")
	}
}

func TestFreezeCmdArgs(t *testing.T) {
	// MaximumNArgs(1) means 0 or 1 args allowed
	if freezeCmd.Args == nil {
		t.Fatal("freezeCmd.Args should be set")
	}
}

