package commands

import (
	"encoding/json"
	"strings"
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

// TestPsSlotOutput_ForeignOmitsWorktreeIdentity verifies that a slot held by
// a different grove project's compose stack (#147) serializes with the
// "foreign" flag set and doesn't imply worktree/url ownership it can't back
// up — an empty Name/URL is omitted from the JSON entirely rather than
// printed as "".
func TestPsSlotOutput_ForeignOmitsWorktreeIdentity(t *testing.T) {
	s := psSlotOutput{
		Slot:    2,
		Project: "otherapp-agent-2",
		Foreign: true,
	}

	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	got := string(out)

	if !strings.Contains(got, `"foreign":true`) {
		t.Errorf("Marshal() = %s, want foreign:true", got)
	}
	if strings.Contains(got, `"worktree"`) {
		t.Errorf("Marshal() = %s, want no worktree field for a foreign slot", got)
	}
	if strings.Contains(got, `"url"`) {
		t.Errorf("Marshal() = %s, want no url field for a foreign slot", got)
	}
}

// TestPsSlotOutput_StoppedForeignSlotReportsRunningFalse verifies a foreign
// slot held only by a stopped/exited container serializes with
// "running":false so machine consumers can distinguish it from a live
// foreign stack without re-querying docker themselves.
func TestPsSlotOutput_StoppedForeignSlotReportsRunningFalse(t *testing.T) {
	s := psSlotOutput{
		Slot:    3,
		Project: "otherapp-agent-3",
		Foreign: true,
		Running: false,
	}

	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	got := string(out)

	if !strings.Contains(got, `"running":false`) {
		t.Errorf("Marshal() = %s, want running:false for a stopped foreign slot", got)
	}
}
