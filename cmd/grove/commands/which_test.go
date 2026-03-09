package commands

import (
	"testing"
)

func TestWhichCmd(t *testing.T) {
	if whichCmd == nil {
		t.Fatal("whichCmd is nil")
	}

	if whichCmd.Use != "which" {
		t.Errorf("whichCmd.Use = %v, want 'which'", whichCmd.Use)
	}

	if whichCmd.RunE == nil {
		t.Error("whichCmd.RunE is nil")
	}
}

func TestWhichAliases(t *testing.T) {
	found := false
	for _, alias := range whichCmd.Aliases {
		if alias == "status" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'status' alias on whichCmd")
	}
}

func TestWhichFlags(t *testing.T) {
	flags := whichCmd.Flags()

	jsonFlag := flags.Lookup("json")
	if jsonFlag == nil {
		t.Error("expected --json flag to exist")
	}
}
