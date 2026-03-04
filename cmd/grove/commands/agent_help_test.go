package commands

import (
	"bytes"
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
		"Full Documentation",
	}

	for _, section := range requiredSections {
		if !bytes.Contains(buf.Bytes(), []byte(section)) {
			t.Errorf("agent-help output missing: %s", section)
		}
	}
}
