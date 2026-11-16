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
		"Full Documentation",
	}

	for _, section := range requiredSections {
		if !bytes.Contains(buf.Bytes(), []byte(section)) {
			t.Errorf("agent-help output missing: %s", section)
		}
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
