package commands

import (
	"testing"
)

func TestFetchCmd(t *testing.T) {
	if fetchCmd == nil {
		t.Fatal("fetchCmd is nil")
	}

	if fetchCmd.Use != "fetch <pr|issue>/<number>" {
		t.Errorf("fetchCmd.Use = %v, want 'fetch <pr|issue>/<number>'", fetchCmd.Use)
	}

	if fetchCmd.Short == "" {
		t.Error("fetchCmd.Short is empty")
	}

	if fetchCmd.Args == nil {
		t.Error("fetchCmd.Args is nil")
	}

	if fetchCmd.RunE == nil {
		t.Error("fetchCmd.RunE is nil")
	}
}

// Integration tests would require gh CLI and a real repository
// Skipping for unit tests
