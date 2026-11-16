package commands

import "github.com/spf13/cobra"

// jsonSwitchHelp is the --json help of every command whose result object has
// a switch_to field.
const jsonSwitchHelp = "Output as JSON with switch_to field"

// createFlags are the flags the commands that create a worktree share.
// Registering them in one place keeps the commands from drifting apart.
type createFlags struct {
	json     bool
	noSwitch bool
	up       bool
	noDocker bool
}

func (f *createFlags) registerJSON(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&f.json, "json", "j", false, jsonSwitchHelp)
}

func (f *createFlags) registerNoSwitch(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&f.noSwitch, "no-switch", "n", false, "Stay where you are after the worktree is created")
}

// registerStack adds --up and --no-docker, the per-run forms of
// [plugins.docker] auto_up = true and false.
func (f *createFlags) registerStack(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&f.up, "up", "u", false, "Also bring the stack to the worktree")
	cmd.Flags().BoolVar(&f.noDocker, "no-docker", false, "Do not bring the stack, even with auto_up")
	cmd.MarkFlagsMutuallyExclusive("up", "no-docker")
}

func (f *createFlags) registerAll(cmd *cobra.Command) {
	f.registerJSON(cmd)
	f.registerNoSwitch(cmd)
	f.registerStack(cmd)
}
