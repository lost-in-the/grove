package commands

import (
	"strings"
	"testing"
)

func TestTimeCommand(t *testing.T) {
	// Verify command exists and is properly configured
	if timeCmd == nil {
		t.Fatal("timeCmd is nil")
	}

	if !strings.HasPrefix(timeCmd.Use, "time") {
		t.Errorf("expected Use to start with 'time', got %q", timeCmd.Use)
	}

	if timeCmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

func TestTimeCommand_Flags(t *testing.T) {
	// Verify flags are registered
	flags := timeCmd.Flags()

	if flags.Lookup("all") == nil {
		t.Error("expected --all flag to be registered")
	}

	if flags.Lookup("json") == nil {
		t.Error("expected --json flag to be registered")
	}
}
