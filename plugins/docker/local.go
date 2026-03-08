package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
)

// localStrategy implements the docker mode for projects with their own docker-compose file.
type localStrategy struct {
	cfg *config.Config
}

func newLocalStrategy(cfg *config.Config) *localStrategy {
	return &localStrategy{cfg: cfg}
}

func (s *localStrategy) OnPreSwitch(ctx *hooks.Context) error {
	if !s.getAutoStop() {
		return nil
	}

	worktreePath := s.getWorktreePath(ctx.PrevWorktree)
	if !hasDockerCompose(worktreePath) {
		return nil
	}

	return s.down(worktreePath)
}

func (s *localStrategy) OnPostSwitch(ctx *hooks.Context) error {
	if !s.getAutoStart() {
		return nil
	}

	worktreePath := s.getWorktreePath(ctx.Worktree)
	if !hasDockerCompose(worktreePath) {
		return nil
	}

	return s.up(worktreePath, false)
}

func (s *localStrategy) OnPostCreate(_ *hooks.Context) error {
	// Local mode has no post-create behavior
	return nil
}

func (s *localStrategy) Up(worktreePath string, detach bool) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	args := []string{"up"}
	if detach {
		args = append(args, "-d")
	}

	cmd := composeCommand(worktreePath, "", nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *localStrategy) Down(worktreePath string) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	cmd := composeCommand(worktreePath, "", nil, "down")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *localStrategy) Logs(worktreePath string, service string, follow bool) error {
	if !hasDockerCompose(worktreePath) {
		return fmt.Errorf("no docker-compose file found in %s", worktreePath)
	}

	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if service != "" {
		args = append(args, service)
	}

	cmd := composeCommand(worktreePath, "", nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (s *localStrategy) Run(worktreePath string, service string, command string) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	cmd := composeCommand(worktreePath, "", nil, "run", "--rm", service, "bash", "-cil", command)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (s *localStrategy) Restart(worktreePath string, service string) error {
	if !hasDockerCompose(worktreePath) {
		return fmt.Errorf("no docker-compose file found in %s", worktreePath)
	}

	args := []string{"restart"}
	if service != "" {
		args = append(args, service)
	}

	cmd := composeCommand(worktreePath, "", nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *localStrategy) down(worktreePath string) error {
	return s.Down(worktreePath)
}

func (s *localStrategy) up(worktreePath string, detach bool) error {
	return s.Up(worktreePath, detach)
}

func (s *localStrategy) getAutoStart() bool {
	if s.cfg != nil && s.cfg.Plugins.Docker.AutoStart != nil {
		return *s.cfg.Plugins.Docker.AutoStart
	}
	return true
}

func (s *localStrategy) getAutoStop() bool {
	if s.cfg != nil && s.cfg.Plugins.Docker.AutoStop != nil {
		return *s.cfg.Plugins.Docker.AutoStop
	}
	return false
}

func (s *localStrategy) getWorktreePath(name string) string {
	if name == "" {
		return ""
	}

	if filepath.IsAbs(name) {
		return name
	}

	if s.cfg != nil && s.cfg.ProjectsDir != "" {
		return filepath.Join(resolveComposePath(s.cfg.ProjectsDir), name)
	}

	cwd, _ := os.Getwd()
	return filepath.Join(cwd, name)
}

// hasDockerCompose checks if a directory has a docker-compose file
func hasDockerCompose(dir string) bool {
	composeFiles := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, file := range composeFiles {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// composeCommand creates a docker-compose command with optional environment variables.
// dir sets the working directory. envFile, when non-empty and not ".env", adds
// --env-file to the compose command for YAML variable interpolation. env is a list
// of extra KEY=VALUE strings to add to the process environment.
//
// Callers set cmd.Stdout = os.Stderr intentionally: grove reserves stdout for
// shell-integration directives (cd:, env:, tmux-attach:), so Docker output
// must go to stderr to avoid corrupting the directive stream.
func composeCommand(dir string, envFile string, env []string, args ...string) *exec.Cmd {
	if _, err := exec.LookPath("docker"); err == nil {
		var cmdArgs []string
		if envFile != "" && envFile != ".env" {
			cmdArgs = append(cmdArgs, "compose", "--env-file", envFile)
		} else {
			cmdArgs = append(cmdArgs, "compose")
		}
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.Command("docker", cmdArgs...)
		cmd.Dir = dir
		if len(env) > 0 {
			cmd.Env = append(os.Environ(), env...)
		}
		return cmd
	}

	// docker-compose v1 fallback — --env-file goes after compose subcommand
	var cmdArgs []string
	if envFile != "" && envFile != ".env" {
		cmdArgs = append(cmdArgs, "--env-file", envFile)
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("docker-compose", cmdArgs...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd
}
