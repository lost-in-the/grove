package commands

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestCreateCommandsShareTheirFlags(t *testing.T) {
	type flag struct{ name, shorthand string }
	stack := []flag{{"up", "u"}, {"no-docker", ""}}
	all := append([]flag{{"json", "j"}, {"no-switch", "n"}}, stack...)

	tests := []struct {
		cmd   *cobra.Command
		flags []flag
	}{
		{newCmd, all},
		{forkCmd, all},
		{fetchCmd, all},
		{openCmd, append([]flag{{"json", "j"}}, stack...)},
	}
	for _, tt := range tests {
		for _, want := range tt.flags {
			f := tt.cmd.Flags().Lookup(want.name)
			if f == nil {
				t.Errorf("%s has no --%s flag", tt.cmd.Name(), want.name)
				continue
			}
			if f.Shorthand != want.shorthand {
				t.Errorf("%s --%s shorthand = %q, want %q", tt.cmd.Name(), want.name, f.Shorthand, want.shorthand)
			}
		}
	}
}

func TestSharedFlagHelpIsTheSameOnEveryCommand(t *testing.T) {
	groups := map[string][]*cobra.Command{
		"up":        {newCmd, forkCmd, fetchCmd, openCmd},
		"no-docker": {newCmd, forkCmd, fetchCmd, openCmd},
		"no-switch": {newCmd, forkCmd, fetchCmd},
		// Every one of these prints an object with a switch_to field.
		"json": {newCmd, forkCmd, fetchCmd, openCmd, toCmd, lastCmd},
	}
	for name, cmds := range groups {
		var first *pflag.Flag
		for _, cmd := range cmds {
			f := cmd.Flags().Lookup(name)
			if f == nil {
				t.Errorf("%s has no --%s flag", cmd.Name(), name)
				continue
			}
			if first == nil {
				first = f
				continue
			}
			if f.Usage != first.Usage {
				t.Errorf("--%s help differs: %s says %q, %s says %q", name, cmds[0].Name(), first.Usage, cmd.Name(), f.Usage)
			}
		}
	}
}

func TestUpCannotBeCombinedWithNoDocker(t *testing.T) {
	for _, cmd := range []*cobra.Command{newCmd, forkCmd, fetchCmd, openCmd} {
		f := cmd.Flags().Lookup("up")
		if f == nil {
			t.Errorf("%s has no --up flag", cmd.Name())
			continue
		}
		groups := f.Annotations["cobra_annotation_mutually_exclusive"]
		if !slices.Contains(groups, "up no-docker") {
			t.Errorf("%s: --up exclusive groups = %v, want %q among them", cmd.Name(), groups, "up no-docker")
		}
	}
}

func TestNoDockerHelpSaysStack(t *testing.T) {
	for _, cmd := range []*cobra.Command{newCmd, forkCmd, fetchCmd, openCmd} {
		f := cmd.Flags().Lookup("no-docker")
		if f == nil {
			continue
		}
		if !strings.Contains(f.Usage, "stack") || strings.Contains(f.Usage, "auto-start") {
			t.Errorf("%s --no-docker help = %q, want the stack vocabulary", cmd.Name(), f.Usage)
		}
	}
}

func TestFlagHelpHasNoAccidentalValueName(t *testing.T) {
	// Cobra reads a back-quoted word in a bool flag's help as its value name
	// and prints "--dirty git diff HEAD".
	rootCmd.InitDefaultHelpCmd()
	for _, cmd := range rootCmd.Commands() {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Value.Type() != "bool" {
				return
			}
			if name, _ := pflag.UnquoteUsage(f); name != "" {
				t.Errorf("%s --%s: help back-quotes %q, which cobra prints as a value name", cmd.Name(), f.Name, name)
			}
		})
	}
}

func TestEveryJSONFlagHasTheShorthand(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if f := cmd.Flags().Lookup("json"); f != nil && f.Shorthand != "j" {
			t.Errorf("%s --json shorthand = %q, want %q", cmd.Name(), f.Shorthand, "j")
		}
	}
}

func TestQuietCannotBeCombinedWithJSON(t *testing.T) {
	for _, cmd := range []*cobra.Command{hereCmd, lsCmd} {
		groups := cmd.Flags().Lookup("quiet").Annotations["cobra_annotation_mutually_exclusive"]
		if !slices.Contains(groups, "json quiet") {
			t.Errorf("%s: --quiet exclusive groups = %v, want %q among them", cmd.Name(), groups, "json quiet")
		}
	}
}

func TestCommandsThatTakeAWorktreeNameCompleteIt(t *testing.T) {
	for _, cmd := range []*cobra.Command{attachCmd, applyCmd, syncCmd, testCmd} {
		if cmd.ValidArgsFunction == nil {
			t.Errorf("%s does not complete worktree names", cmd.Name())
		}
	}
}

func TestStateFlagCompletesItsValues(t *testing.T) {
	for _, cmd := range []*cobra.Command{prsCmd, issuesCmd} {
		fn, ok := cmd.GetFlagCompletionFunc("state")
		if !ok {
			t.Errorf("%s --state has no value completion", cmd.Name())
			continue
		}
		got, directive := fn(cmd, nil, "")
		if !slices.Equal(got, []string{"open", "closed", "all"}) {
			t.Errorf("%s --state completes %v, want [open closed all]", cmd.Name(), got)
		}
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("%s --state directive = %v, want no file completion", cmd.Name(), directive)
		}
	}
}

func TestJSONRequested(t *testing.T) {
	newTestCmd := func() *cobra.Command {
		cmd := &cobra.Command{Use: "x"}
		cmd.Flags().BoolP("json", "j", false, "")
		return cmd
	}

	on := newTestCmd()
	if err := on.Flags().Set("json", "true"); err != nil {
		t.Fatal(err)
	}
	if !jsonRequested(on) {
		t.Error("jsonRequested = false with --json set")
	}
	if jsonRequested(newTestCmd()) {
		t.Error("jsonRequested = true with --json unset")
	}
	if jsonRequested(&cobra.Command{Use: "plain"}) {
		t.Error("jsonRequested = true for a command with no --json flag")
	}
	if jsonRequested(nil) {
		t.Error("jsonRequested = true for no command")
	}
}
