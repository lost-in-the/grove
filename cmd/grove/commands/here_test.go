package commands

import (
	"testing"
)

func TestHereCmd(t *testing.T) {
	if hereCmd == nil {
		t.Fatal("hereCmd is nil")
	}

	if hereCmd.Use != "here" {
		t.Errorf("hereCmd.Use = %v, want 'here'", hereCmd.Use)
	}

	if hereCmd.RunE == nil {
		t.Error("hereCmd.RunE is nil")
	}
}

func TestHereFlags(t *testing.T) {
	// Verify flags exist
	flags := hereCmd.Flags()

	quietFlag := flags.Lookup("quiet")
	if quietFlag == nil {
		t.Error("expected --quiet flag to exist")
	}

	jsonFlag := flags.Lookup("json")
	if jsonFlag == nil {
		t.Error("expected --json flag to exist")
	}
}
