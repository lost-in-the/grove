package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAgentHelpOutput(t *testing.T) {
	// Capture output
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	// Call the output function directly
	writeAgentHelp(cmd)

	// Must contain key sections
	requiredSections := []string{
		"Grove Agent Quick Reference",
		"Environment",
		"GROVE_AGENT_MODE=1",
		"GROVE_NONINTERACTIVE=1",
		"GROVE_SHELL=1",
		"Common Commands",
		"grove new <name>",
		"grove ls --json",
		"grove to <name>",
		"grove fetch pr/",
		"grove rm <name>",
		"Agent Tips",
		"grove to <name> --up",
		"does not touch the stack by default",
		"grove up --isolated",
		"grove new <name> --json prints switch_to",
		"Full Documentation",
	}

	for _, section := range requiredSections {
		if !bytes.Contains(buf.Bytes(), []byte(section)) {
			t.Errorf("agent-help output missing: %s", section)
		}
	}
	if bytes.Contains(buf.Bytes(), []byte("grove new emits cd:")) {
		t.Error("agent-help says grove new emits cd: directives; that needs GROVE_SHELL=1")
	}
}

// Each stack command's line in the agent reference says what its own help says.
func TestAgentHelp_StackCommandsMatchTheirShortHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	writeAgentHelp(cmd)

	for name, short := range map[string]string{"grove up": upCmd.Short, "grove down": downCmd.Short} {
		var line string
		for _, l := range strings.Split(buf.String(), "\n") {
			if f := strings.Fields(l); len(f) >= 3 && f[0]+" "+f[1] == name && !strings.HasPrefix(f[2], "-") {
				line = l
				break
			}
		}
		if got := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), name)); got != short {
			t.Errorf("agent help %q describes it as %q, want its Short help %q", name, got, short)
		}
	}
}

func TestAgentHelp_ListsEveryCommandWithAJSONFlag(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	writeAgentHelp(cmd)

	var line string
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "--json works on:") {
			line = l
		}
	}
	if line == "" {
		t.Fatal("agent help has no '--json works on:' line")
	}
	_, list, _ := strings.Cut(line, "--json works on:")
	listed := strings.Fields(strings.ReplaceAll(list, ",", " "))
	for _, c := range rootCmd.Commands() {
		if c.Flags().Lookup("json") == nil {
			continue
		}
		found := false
		for _, name := range listed {
			found = found || name == c.Name()
		}
		if !found {
			t.Errorf("agent help does not list %q among the --json commands: %v", c.Name(), listed)
		}
	}
}
