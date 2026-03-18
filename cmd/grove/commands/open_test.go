package commands

import (
	"testing"
)

func TestOpenCmd(t *testing.T) {
	if openCmd == nil {
		t.Fatal("openCmd is nil")
	}

	if openCmd.Use != "open [name]" {
		t.Errorf("openCmd.Use = %v, want 'open [name]'", openCmd.Use)
	}

	if openCmd.Short == "" {
		t.Error("openCmd.Short is empty")
	}

	if openCmd.RunE == nil {
		t.Error("openCmd.RunE is nil")
	}
}

func TestOpenCmd_Flags(t *testing.T) {
	flags := openCmd.Flags()

	tests := []string{"no-create", "command", "no-popup", "json", "no-docker"}
	for _, name := range tests {
		if flags.Lookup(name) == nil {
			t.Errorf("expected --%s flag to exist", name)
		}
	}
}

func TestOpenCmd_NoCreateFlag(t *testing.T) {
	flag := openCmd.Flags().Lookup("no-create")
	if flag == nil {
		t.Fatal("openCmd missing --no-create flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--no-create should default to false, got %s", flag.DefValue)
	}
}

func TestOpenCmd_JsonFlag(t *testing.T) {
	flag := openCmd.Flags().Lookup("json")
	if flag == nil {
		t.Fatal("openCmd missing --json flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--json should default to false, got %s", flag.DefValue)
	}
}

func TestOpenCmd_NoDockerFlag(t *testing.T) {
	flag := openCmd.Flags().Lookup("no-docker")
	if flag == nil {
		t.Fatal("openCmd missing --no-docker flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--no-docker should default to false, got %s", flag.DefValue)
	}
}

func TestOpenCmd_RequiresExactlyOneArg(t *testing.T) {
	if openCmd.Args == nil {
		t.Error("openCmd.Args should not be nil — should require exactly one argument")
	}
}
