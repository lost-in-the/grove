package commands

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/plugins/docker"
)

func init() {
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system health for grove",
	Long: `Run diagnostic checks to verify that grove's dependencies and configuration
are set up correctly.

Checks include Docker availability, external compose configuration,
agent stack readiness, and network connectivity.

Examples:
  grove doctor`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		fmt.Println("grove doctor")
		fmt.Println(strings.Repeat("━", 40))

		allPassed := true

		// Check: Docker available
		allPassed = runCheck("Docker available", func() (string, error) {
			if _, err := exec.LookPath("docker"); err != nil {
				return "", fmt.Errorf("docker not found in PATH")
			}
			return "found in PATH", nil
		}) && allPassed

		// Check: Docker daemon running
		allPassed = runCheck("Docker running", func() (string, error) {
			cmd := exec.Command("docker", "info", "--format", "{{.ServerVersion}}")
			out, err := cmd.Output()
			if err != nil {
				return "", fmt.Errorf("docker daemon not responding (is Docker running?)")
			}
			return "v" + strings.TrimSpace(string(out)), nil
		}) && allPassed

		// Check: External compose mode
		cfg := ctx.Config
		if cfg == nil || !cfg.IsExternalDockerMode() {
			runInfo("External mode", "not configured (using local compose)")
		} else {
			ext := cfg.Plugins.Docker.External
			allPassed = runCheck("External compose path", func() (string, error) {
				if ext.Path == "" {
					return "", fmt.Errorf("plugins.docker.external.path not set")
				}
				return ext.Path, nil
			}) && allPassed

			// Check: Agent stack config
			if ext.Agent == nil || ext.Agent.Enabled == nil || !*ext.Agent.Enabled {
				runInfo("Agent stacks", "not enabled")
			} else {
				allPassed = runCheck("Agent config", func() (string, error) {
					if len(ext.Agent.Services) == 0 {
						return "", fmt.Errorf("agent.services is empty")
					}
					if ext.Agent.TemplatePath == "" {
						return "", fmt.Errorf("agent.template_path not set")
					}
					return fmt.Sprintf("%d services, max %d slots", len(ext.Agent.Services), ext.Agent.MaxSlots), nil
				}) && allPassed

				// Check: Network exists (if configured)
				if ext.Agent.Network != "" {
					allPassed = runCheck("Docker network '"+ext.Agent.Network+"'", func() (string, error) {
						cmd := exec.Command("docker", "network", "ls", "--format", "{{.Name}}")
						out, err := cmd.Output()
						if err != nil {
							return "", fmt.Errorf("failed to list networks: %w", err)
						}
						for _, line := range strings.Split(string(out), "\n") {
							if strings.TrimSpace(line) == ext.Agent.Network {
								return "exists", nil
							}
						}
						return "", fmt.Errorf("network not found (is the main stack running?)")
					}) && allPassed
				}

				// Check: Active slots
				slots, err := docker.ListActiveSlots(cfg)
				if err == nil {
					maxSlots := ext.Agent.MaxSlots
					if maxSlots <= 0 {
						maxSlots = 5
					}
					runInfo("Active stacks", fmt.Sprintf("%d/%d slots in use", len(slots), maxSlots))
				}
			}
		}

		fmt.Println()
		if allPassed {
			fmt.Println("All checks passed")
		} else {
			fmt.Println("Some checks failed — see above for details")
		}

		return nil
	}),
}

func runCheck(name string, check func() (string, error)) bool {
	detail, err := check()
	if err != nil {
		fmt.Printf("  FAIL  %s: %v\n", name, err)
		return false
	}
	fmt.Printf("  OK    %s", name)
	if detail != "" {
		fmt.Printf(" (%s)", detail)
	}
	fmt.Println()
	return true
}

func runInfo(name string, detail string) {
	fmt.Printf("  ---   %s: %s\n", name, detail)
}
