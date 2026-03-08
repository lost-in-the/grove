package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/LeahArmstrong/grove-cli/internal/cmdexec"
	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

// agentExternalStrategy implements the docker mode for agent worktrees that run
// independent stacks (not sharing the main compose services).
type agentExternalStrategy struct {
	cfg   *config.Config
	ext   *config.ExternalComposeConfig
	agent *config.AgentStackConfig
	slots *SlotManager
}

func newAgentExternalStrategy(cfg *config.Config) *agentExternalStrategy {
	ext := cfg.Plugins.Docker.External
	agent := ext.Agent

	maxSlots := agent.MaxSlots
	if maxSlots <= 0 {
		maxSlots = 5
	}

	templateDir := filepath.Dir(agent.TemplatePath)
	var slotsFile string
	if filepath.IsAbs(agent.TemplatePath) {
		slotsFile = filepath.Join(templateDir, ".slots.json")
	} else {
		slotsFile = filepath.Join(resolveComposePath(ext.Path), templateDir, ".slots.json")
	}

	return &agentExternalStrategy{
		cfg:   cfg,
		ext:   ext,
		agent: agent,
		slots: NewSlotManager(slotsFile, maxSlots),
	}
}

// OnPreSwitch is a no-op for agent mode — agent stacks are independent.
func (s *agentExternalStrategy) OnPreSwitch(_ *hooks.Context) error {
	return nil
}

// OnPostSwitch is a no-op for agent mode — agent stacks are independent.
func (s *agentExternalStrategy) OnPostSwitch(_ *hooks.Context) error {
	return nil
}

// OnPostCreate copies credentials and creates symlinks, same as the human workflow.
func (s *agentExternalStrategy) OnPostCreate(ctx *hooks.Context) error {
	if ctx.WorktreePath == "" || ctx.MainPath == "" {
		return nil
	}

	return setupWorktreeFiles(s.ext, ctx.WorktreePath, ctx.MainPath)
}

// Up starts a persistent agent stack for the worktree (full stack mode).
func (s *agentExternalStrategy) Up(worktreePath string, detach bool) error {
	wtName := filepath.Base(worktreePath)

	slot, err := s.slots.Allocate(wtName)
	if err != nil {
		return fmt.Errorf("failed to allocate agent slot: %w", err)
	}

	projectName := s.composeProjectName(slot)
	templatePath := s.resolveTemplatePath()
	composePath := s.composePath()

	// Check that the required external Docker network exists (if configured)
	if s.agent.Network != "" {
		if err := checkDockerNetwork(s.agent.Network); err != nil {
			_ = s.slots.Release(wtName)
			return err
		}
	}

	// Show slot usage
	active, _ := s.slots.ListActive()
	fmt.Fprintf(os.Stderr, "Using slot %d/%d", slot, s.slots.maxSlots)
	if len(active) > 1 {
		fmt.Fprintf(os.Stderr, " (%d active)", len(active))
	}
	fmt.Fprintln(os.Stderr)

	// Warn about memory if possible
	warnMemoryUsage(len(active))

	env := s.agentEnv(worktreePath, slot)

	args := []string{"up"}
	if detach {
		args = append(args, "-d")
	}
	args = append(args, s.agent.Services...)

	cmd := agentComposeCommand(composePath, templatePath, projectName, env, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start agent stack: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Agent stack started (slot %d)\n", slot)
	if s.agent.URLPattern != "" {
		fmt.Fprintf(os.Stderr, "Available at: %s\n", formatAgentURL(s.agent.URLPattern, slot))
	}

	return nil
}

// Down stops and removes a persistent agent stack.
func (s *agentExternalStrategy) Down(worktreePath string) error {
	wtName := filepath.Base(worktreePath)

	slot, err := s.slots.FindSlot(wtName)
	if err != nil {
		return fmt.Errorf("failed to find agent slot: %w", err)
	}
	if slot == 0 {
		return fmt.Errorf("no agent stack running for worktree %q", wtName)
	}

	projectName := s.composeProjectName(slot)
	templatePath := s.resolveTemplatePath()
	composePath := s.composePath()

	env := s.agentEnv(worktreePath, slot)

	cmd := agentComposeCommand(composePath, templatePath, projectName, env, "down")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop agent stack: %w", err)
	}

	if err := s.slots.Release(wtName); err != nil {
		return fmt.Errorf("failed to release agent slot: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Agent stack stopped (slot %d)\n", slot)
	return nil
}

// agentCtx holds the resolved context needed to run agent compose commands.
type agentCtx struct {
	composePath  string
	templatePath string
	projectName  string
	env          []string
	wtName       string
}

// agentContext resolves compose paths, project name, and base env for a worktree.
// Uses an existing slot if allocated; falls back to the ephemeral project name.
func (s *agentExternalStrategy) agentContext(worktreePath string) agentCtx {
	wtName := filepath.Base(worktreePath)
	slot, _ := s.slots.FindSlot(wtName)
	return agentCtx{
		composePath:  s.composePath(),
		templatePath: s.resolveTemplatePath(),
		projectName:  s.composeProjectName(slot),
		env:          s.agentEnv(worktreePath, slot),
		wtName:       wtName,
	}
}

// Run executes a command in an ephemeral container using the agent compose project.
func (s *agentExternalStrategy) Run(worktreePath string, service string, command string) error {
	ac := s.agentContext(worktreePath)

	env := ac.env
	// Add TEST_ENV_NUMBER for test commands
	if isTestCommand(command) {
		envNum := worktree.TestEnvNumber(ac.wtName)
		env = append(env, fmt.Sprintf("TEST_ENV_NUMBER=%d", envNum))
	}

	cmd := agentComposeCommand(ac.composePath, ac.templatePath, ac.projectName, env, "run", "--rm", service, "bash", "-cil", command)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// Logs tails logs for agent-specific containers.
func (s *agentExternalStrategy) Logs(worktreePath string, service string, follow bool) error {
	ac := s.agentContext(worktreePath)

	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if service != "" {
		args = append(args, service)
	} else {
		args = append(args, s.agent.Services...)
	}

	cmd := agentComposeCommand(ac.composePath, ac.templatePath, ac.projectName, ac.env, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// Restart restarts agent containers.
func (s *agentExternalStrategy) Restart(worktreePath string, service string) error {
	ac := s.agentContext(worktreePath)

	args := []string{"restart"}
	if service != "" {
		args = append(args, service)
	} else {
		args = append(args, s.agent.Services...)
	}

	cmd := agentComposeCommand(ac.composePath, ac.templatePath, ac.projectName, ac.env, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// composeProjectName returns the compose project name for a given slot.
func (s *agentExternalStrategy) composeProjectName(slot int) string {
	return agentComposeProjectName(s.cfg.ProjectName, slot)
}

// agentComposeProjectName is the shared naming logic used by both the strategy
// and the public AgentComposeProjectName function.
func agentComposeProjectName(projectName string, slot int) string {
	if projectName == "" {
		projectName = "grove"
	}
	if slot > 0 {
		return fmt.Sprintf("%s-agent-%d", projectName, slot)
	}
	return fmt.Sprintf("%s-agent-ephemeral", projectName)
}

// composePath returns the resolved absolute path to the external compose directory.
func (s *agentExternalStrategy) composePath() string {
	return resolveComposePath(s.ext.Path)
}

// resolveTemplatePath returns the absolute path to the agent compose template.
func (s *agentExternalStrategy) resolveTemplatePath() string {
	tmpl := s.agent.TemplatePath
	if filepath.IsAbs(tmpl) {
		return tmpl
	}
	return filepath.Join(s.composePath(), tmpl)
}

// agentEnv builds the environment variables for agent compose commands.
func (s *agentExternalStrategy) agentEnv(worktreePath string, slot int) []string {
	env := []string{
		s.ext.EnvVar + "=" + worktreePath,
	}
	if slot > 0 {
		env = append(env, fmt.Sprintf("AGENT_SLOT=%d", slot))
	}
	return env
}

// resolveComposePath resolves ~ in a compose directory path.
func resolveComposePath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	return path
}

// setupWorktreeFiles copies credentials and creates symlinks from the external config.
// This is shared between externalStrategy and agentExternalStrategy.
func setupWorktreeFiles(ext *config.ExternalComposeConfig, newPath, mainPath string) error {
	var firstErr error

	for _, relPath := range ext.CopyFiles {
		src := filepath.Join(mainPath, relPath)
		dst := filepath.Join(newPath, relPath)

		if err := copyFile(src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to copy %s: %v\n", relPath, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Fprintf(os.Stderr, "  copied %s\n", relPath)
	}

	for _, relPath := range ext.SymlinkFiles {
		src := filepath.Join(mainPath, relPath)
		dst := filepath.Join(newPath, relPath)

		if err := createSymlink(src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to symlink %s: %v\n", relPath, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Fprintf(os.Stderr, "  symlinked %s\n", relPath)
	}

	for _, relPath := range ext.SymlinkDirs {
		src := filepath.Join(mainPath, relPath)
		dst := filepath.Join(newPath, relPath)

		if err := createSymlink(src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to symlink %s: %v\n", relPath, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Fprintf(os.Stderr, "  symlinked %s\n", relPath)
	}

	return firstErr
}

// agentComposeCommand creates a docker compose command with -f and -p flags for agent projects.
func agentComposeCommand(composePath string, templateFile string, projectName string, env []string, args ...string) *exec.Cmd {
	cmdArgs := []string{"compose", "-f", templateFile, "-p", projectName}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("docker", cmdArgs...)
	cmd.Dir = composePath
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd
}

// checkDockerNetwork verifies that the named external Docker network exists.
func checkDockerNetwork(networkName string) error {
	output, err := cmdexec.Output(context.TODO(), "docker", []string{"network", "ls", "--format", "{{.Name}}"}, "", cmdexec.Docker)
	if err != nil {
		return fmt.Errorf("failed to list Docker networks: %w", err)
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) == networkName {
			return nil
		}
	}

	return fmt.Errorf("main infrastructure must be running: Docker network %q not found\nEnsure the shared compose stack is running first", networkName)
}

// estimatedRAMPerStack is the approximate RAM per full agent stack in GB.
const estimatedRAMPerStack = 1.5

// warnMemoryUsage prints a warning if system memory is low relative to active stacks.
// This is best-effort — silently does nothing if memory info is unavailable.
func warnMemoryUsage(activeStacks int) {
	totalGB := totalSystemMemoryGB()
	if totalGB <= 0 {
		return
	}

	estimatedUsage := float64(activeStacks) * estimatedRAMPerStack
	// Rough estimate: base system + main stack needs ~8GB
	available := totalGB - 8.0 - estimatedUsage

	if available < 2.0 {
		fmt.Fprintf(os.Stderr, "Warning: low memory — %.0fGB total with %d active stack(s)\n", totalGB, activeStacks)
		fmt.Fprintf(os.Stderr, "  Consider stopping unused stacks with 'grove down'\n")
	}
}

// totalSystemMemoryGB returns total system RAM in GB, or 0 if unavailable.
func totalSystemMemoryGB() float64 {
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err != nil {
			return 0
		}
		bytes, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			return 0
		}
		return float64(bytes) / (1024 * 1024 * 1024)
	}
	// Linux: read /proc/meminfo
	if runtime.GOOS == "linux" {
		out, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					kb, err := strconv.ParseUint(fields[1], 10, 64)
					if err == nil {
						return float64(kb) / (1024 * 1024)
					}
				}
			}
		}
	}
	return 0
}
